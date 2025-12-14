import AppSupportClients
import ComposableArchitecture
import Foundation

@Reducer
public struct ProfileFeature {
    @ObservableState
    public struct State: Equatable {
        public var userProfile: UserProfile?
        public var isEditing: Bool = false
        public var isLoading: Bool = false
        public var errorMessage: String?

        // Editable fields
        public var editableDisplayName: String = ""
        public var editableUsername: String = ""
        public var editableEmail: String = ""
        public var editableMusicPreferences: String = ""

        public init() {}
    }

    public enum Action: BindableAction, Equatable {
        case binding(BindingAction<State>)
        case onAppear
        case profileResponse(TaskResult<UserProfile>)
        case toggleEditMode
        case saveButtonTapped
        case updateProfileResponse(TaskResult<UserProfile>)
        case logoutButtonTapped
        case linkAccount(AuthenticationClient.SocialHelper.SocialProvider)
        case unlinkAccount(AuthenticationClient.SocialHelper.SocialProvider)
        case linkAccountResponse(TaskResult<UserProfile>)
    }

    @Dependency(\.user) var userClient
    @Dependency(\.authentication) var authClient
    @Dependency(\.webAuthenticationSession) var webAuth
    @Dependency(\.appSettings) var appSettings

    public init() {}

    public var body: some ReducerOf<Self> {
        BindingReducer()
        Reduce { state, action in
            switch action {
            case .onAppear:
                state.isLoading = true
                return .run { [userClient] send in
                    await send(.profileResponse(TaskResult { try await userClient.me() }))
                }

            case .profileResponse(.success(let profile)):
                state.isLoading = false
                state.userProfile = profile
                state.editableDisplayName = profile.displayName
                state.editableUsername = profile.username
                state.editableEmail = profile.email ?? ""
                state.editableMusicPreferences = profile.preferences?["genres"] ?? ""
                return .none

            case .profileResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = error.localizedDescription
                return .none

            case .toggleEditMode:
                state.isEditing.toggle()
                if !state.isEditing, let profile = state.userProfile {
                    // Reset fields on cancel
                    state.editableDisplayName = profile.displayName
                    state.editableUsername = profile.username
                    state.editableEmail = profile.email ?? ""
                    state.editableMusicPreferences = profile.preferences?["genres"] ?? ""
                }
                return .none

            case .saveButtonTapped:
                guard let currentProfile = state.userProfile else { return .none }
                state.isLoading = true

                let preferences = ["genres": state.editableMusicPreferences]

                let updatedProfile = UserProfile(
                    id: currentProfile.id,
                    userId: currentProfile.userId,
                    username: state.editableUsername,
                    displayName: state.editableDisplayName,
                    avatarUrl: currentProfile.avatarUrl,
                    hasCustomAvatar: currentProfile.hasCustomAvatar,
                    email: state.editableEmail.isEmpty ? nil : state.editableEmail,
                    preferences: preferences
                )

                return .run { [userClient] send in
                    await send(
                        .updateProfileResponse(
                            TaskResult { try await userClient.updateProfile(updatedProfile) }))
                }

            case .updateProfileResponse(.success(let profile)):
                state.isLoading = false
                state.isEditing = false
                state.userProfile = profile
                state.errorMessage = nil
                return .none

            case .updateProfileResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Failed to update profile: \(error.localizedDescription)"
                return .none

            case .logoutButtonTapped:
                return .run { [authClient] _ in
                    await authClient.logout()
                }

            case .linkAccount(let provider):
                state.isLoading = true
                return .run { [appSettings, webAuth, userClient] send in
                    let settings = appSettings.load()
                    let authURL = AuthenticationClient.SocialHelper.authURL(
                        for: provider, baseURL: settings.backendURL)

                    do {
                        let callbackURL = try await webAuth.authenticate(authURL, "musicroom")
                        if let tokens = AuthenticationClient.SocialHelper.parseCallback(
                            url: callbackURL)
                        {
                            // Link using the access token (or idToken) we got
                            // Note: UserClient.link might behave differently depending on backend expectations
                            // Assuming backend "link" endpoint takes the foreign token
                            await send(
                                .linkAccountResponse(
                                    TaskResult {
                                        try await userClient.link(
                                            provider.rawValue, tokens.accessToken)
                                    }))
                        } else {
                            await send(.linkAccountResponse(.failure(URLError(.badServerResponse))))
                        }
                    } catch {
                        await send(.linkAccountResponse(.failure(error)))
                    }
                }

            case .unlinkAccount(let provider):
                state.isLoading = true
                return .run { [userClient] send in
                    await send(
                        .linkAccountResponse(
                            TaskResult {
                                try await userClient.unlink(provider.rawValue)
                            }))
                }

            case .linkAccountResponse(.success(let profile)):
                state.isLoading = false
                state.userProfile = profile
                state.errorMessage = nil
                return .none

            case .linkAccountResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Failed to link/unlink account: \(error.localizedDescription)"
                return .none

            case .binding:
                return .none
            }
        }
    }
}

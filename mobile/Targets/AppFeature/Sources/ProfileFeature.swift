import AppSupportClients
import ComposableArchitecture
import Foundation

@Reducer
public struct ProfileFeature: Sendable {
    @ObservableState
    public struct State: Equatable {
        public var userProfile: UserProfile?
        public var isEditing: Bool = false
        public var isLoading: Bool = false
        public var isAvatarLoading: Bool = false
        public var errorMessage: String?

        // Editable fields
        public var editableDisplayName: String = ""
        public var editableUsername: String = ""
        public var editableEmail: String = ""
        public var editableBio: String = ""
        public var editableVisibility: String = "public"
        public var editableMusicPreferences: String = ""

        // Password change fields
        public var currentPassword = ""
        public var newPassword = ""
        public var confirmNewPassword = ""
        public var isChangingPassword = false
        public var passwordChangeSuccessMessage: String?
        public var isOffline: Bool = false

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
        case changePasswordButtonTapped
        case changePasswordResponse(TaskResult<Bool>)
        case toggleChangePasswordMode
        case generateRandomAvatarTapped
        case generateRandomAvatarResponse(TaskResult<UserProfile>)
    }

    @Dependency(\.user) var userClient
    @Dependency(\.authentication) var authClient
    @Dependency(\.webAuthenticationSession) var webAuth
    @Dependency(\.appSettings) var appSettings
    @Dependency(\.telemetry) var telemetry

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
                state.userProfile = normalizeAvatarUrl(profile)
                state.editableDisplayName = profile.displayName
                state.editableUsername = profile.username
                state.editableEmail = profile.email ?? ""
                state.editableMusicPreferences =
                    profile.preferences.genres?.joined(separator: ",") ?? ""
                return .none

            case .profileResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = error.localizedDescription
                return .none

            case .toggleEditMode:
                if state.isOffline {
                    state.errorMessage = "You cannot edit your profile while offline."
                    return .none
                }
                state.isEditing.toggle()
                if !state.isEditing, let profile = state.userProfile {
                    // Reset fields on cancel
                    state.editableDisplayName = profile.displayName
                    state.editableUsername = profile.username
                    state.editableEmail = profile.email ?? ""
                    state.editableBio = profile.bio ?? ""
                    state.editableVisibility = profile.visibility
                    state.editableMusicPreferences =
                        profile.preferences.genres?.joined(separator: ",") ?? ""
                }
                return .none

            case .saveButtonTapped:
                guard let currentProfile = state.userProfile else { return .none }
                if state.isOffline {
                    state.errorMessage = "You cannot save changes while offline."
                    return .none
                }
                state.isLoading = true

                let genres = state.editableMusicPreferences
                    .split(separator: ",")
                    .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
                    .filter { !$0.isEmpty }

                let preferences = UserPreferences(
                    genres: genres, artists: currentProfile.preferences.artists,
                    moods: currentProfile.preferences.moods)

                let updatedProfile = UserProfile(
                    id: currentProfile.id,
                    userId: currentProfile.userId,
                    username: state.editableUsername,
                    displayName: state.editableDisplayName,
                    avatarUrl: currentProfile.avatarUrl,
                    hasCustomAvatar: currentProfile.hasCustomAvatar,
                    bio: state.editableBio,
                    visibility: state.editableVisibility,
                    preferences: preferences,
                    linkedProviders: currentProfile.linkedProviders,
                    email: state.editableEmail.isEmpty ? nil : state.editableEmail
                )

                if updatedProfile == currentProfile {
                    state.isEditing = false
                    state.isLoading = false
                    return .none
                }

                return .run { [userClient] send in
                    await send(
                        .updateProfileResponse(
                            TaskResult { try await userClient.updateProfile(updatedProfile) }))
                }

            case .updateProfileResponse(.success(let profile)):
                state.isLoading = false
                state.isEditing = false
                state.userProfile = normalizeAvatarUrl(profile)
                state.errorMessage = nil
                return .run { [telemetry] _ in
                    await telemetry.log("user.profile.update.success", [:])
                }

            case .updateProfileResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Failed to update profile: \(error.localizedDescription)"
                return .none

            case .logoutButtonTapped:
                return .run { [authClient] _ in
                    await authClient.logout()
                }

            case .linkAccount(let provider):
                if state.isOffline {
                    state.errorMessage = "You cannot link accounts while offline."
                    return .none
                }
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
                            await send(
                                .linkAccountResponse(
                                    TaskResult {
                                        try await userClient.link(
                                            provider.rawValue, tokens.accessToken)
                                    }))
                            await telemetry.log(
                                "user.account.link.attempt", ["provider": provider.rawValue])
                        } else {
                            await send(.linkAccountResponse(.failure(URLError(.badServerResponse))))
                        }
                    } catch {
                        await send(.linkAccountResponse(.failure(error)))
                    }
                }

            case .unlinkAccount(let provider):
                if state.isOffline {
                    state.errorMessage = "You cannot unlink accounts while offline."
                    return .none
                }
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
                state.userProfile = normalizeAvatarUrl(profile)
                state.errorMessage = nil
                return .run { [telemetry] _ in
                    await telemetry.log("user.account.link.success", [:])
                }

            case .linkAccountResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Failed to link/unlink account: \(error.localizedDescription)"
                return .none

            case .toggleChangePasswordMode:
                state.isChangingPassword.toggle()
                state.errorMessage = nil
                state.passwordChangeSuccessMessage = nil
                state.currentPassword = ""
                state.newPassword = ""
                state.confirmNewPassword = ""
                return .none

            case .changePasswordButtonTapped:
                if state.isOffline {
                    state.errorMessage = "You cannot change password while offline."
                    return .none
                }
                guard !state.currentPassword.isEmpty, !state.newPassword.isEmpty,
                    !state.confirmNewPassword.isEmpty
                else {
                    state.errorMessage = "Please fill in all password fields."
                    return .none
                }
                guard state.newPassword == state.confirmNewPassword else {
                    state.errorMessage = "New passwords do not match."
                    return .none
                }
                state.isLoading = true
                state.errorMessage = nil

                return .run {
                    [current = state.currentPassword, new = state.newPassword, userClient] send in
                    await send(
                        .changePasswordResponse(
                            TaskResult {
                                try await userClient.changePassword(current, new)
                                return true
                            }))
                }

            case .changePasswordResponse(.success):
                state.isLoading = false
                state.isChangingPassword = false
                state.passwordChangeSuccessMessage = "Password changed successfully."
                state.currentPassword = ""
                state.newPassword = ""
                state.confirmNewPassword = ""
                return .run { [telemetry] _ in
                    await telemetry.log("user.password.change.success", [:])
                }

            case .changePasswordResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Failed to change password: \(error.localizedDescription)"
                return .none

            case .generateRandomAvatarTapped:
                if state.isOffline {
                    state.errorMessage = "You cannot generate avatar while offline."
                    return .none
                }
                state.isAvatarLoading = true
                return .run { [userClient] send in
                    await send(
                        .generateRandomAvatarResponse(
                            TaskResult { try await userClient.generateRandomAvatar() }
                        )
                    )
                }

            case .generateRandomAvatarResponse(.success(let profile)):
                state.isAvatarLoading = false
                var normalized = normalizeAvatarUrl(profile)

                // Force UI update by appending a unique query parameter to bypass cache/id check
                if let url = normalized.avatarUrl, !url.isEmpty {
                    let separator = url.contains("?") ? "&" : "?"
                    let newUrl = "\(url)\(separator)v=\(UUID().uuidString)"

                    normalized = UserProfile(
                        id: normalized.id,
                        userId: normalized.userId,
                        username: normalized.username,
                        displayName: normalized.displayName,
                        avatarUrl: newUrl,
                        hasCustomAvatar: normalized.hasCustomAvatar,
                        bio: normalized.bio,
                        visibility: normalized.visibility,
                        preferences: normalized.preferences,
                        linkedProviders: normalized.linkedProviders,
                        email: normalized.email
                    )
                }
                state.userProfile = normalized
                return .none

            case .generateRandomAvatarResponse(.failure(let error)):
                state.isAvatarLoading = false
                state.errorMessage = "Failed to generate avatar: \(error.localizedDescription)"
                return .none

            case .binding:
                return .none
            }
        }
    }

    private func normalizeAvatarUrl(_ profile: UserProfile) -> UserProfile {
        guard let avatarUrl = profile.avatarUrl, !avatarUrl.isEmpty else {
            return profile
        }

        if avatarUrl.lowercased().hasPrefix("http") {
            return profile
        }

        let settings = appSettings.load()
        let baseUrlString = settings.backendURL.absoluteString.trimmingCharacters(
            in: .init(charactersIn: "/"))
        let cleanPath = avatarUrl.trimmingCharacters(in: .init(charactersIn: "/"))
        let fullUrl = "\(baseUrlString)/\(cleanPath)"

        return UserProfile(
            id: profile.id,
            userId: profile.userId,
            username: profile.username,
            displayName: profile.displayName,
            avatarUrl: fullUrl,
            hasCustomAvatar: profile.hasCustomAvatar,
            bio: profile.bio,
            visibility: profile.visibility,
            preferences: profile.preferences,
            linkedProviders: profile.linkedProviders,
            email: profile.email
        )
    }
}

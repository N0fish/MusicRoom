import AppSupportClients
import AuthenticationServices
import ComposableArchitecture
import Foundation
import MusicRoomAPI
import UIKit

@Reducer
public struct ProfileFeature: Sendable {
    @ObservableState
    public struct State: Equatable {
        public var userProfile: UserProfile?
        public var isEditing: Bool = false
        public var isLoading: Bool = false
        public var isAvatarLoading: Bool = false
        public var userStats: MusicRoomAPIClient.UserStats?
        public var errorMessage: String?

        // Editable fields
        public var editableDisplayName: String = ""
        public var editableUsername: String = ""
        public var editableEmail: String = ""
        public var editableBio: String = ""
        public var editableVisibility: String = "public"
        public var editableMusicPreferences: String = ""
        public var isOffline: Bool = false
        public var hasLoaded: Bool = false
        public var isImagePlaygroundPresented: Bool = false
        @Presents public var alert: AlertState<Action.Alert>?

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
        case forgotPasswordButtonTapped
        case forgotPasswordResponse(TaskResult<Bool>)
        case generateRandomAvatarTapped
        case generateRandomAvatarResponse(TaskResult<UserProfile>)
        case fetchStats
        case statsResponse(TaskResult<MusicRoomAPIClient.UserStats>)
        case becomePremiumTapped
        case becomePremiumResponse(TaskResult<UserProfile>)
        case toggleImagePlayground(Bool)
        case imagePlaygroundResponse(URL?)
        case uploadGeneratedAvatar(Data)
        case uploadGeneratedAvatarResponse(TaskResult<UserProfile>)
        case alert(PresentationAction<Alert>)

        public enum Alert: Equatable {
            case confirmPasswordReset
        }
    }

    @Dependency(\.user) var userClient
    @Dependency(\.authentication) var authClient
    @Dependency(\.webAuthenticationSession) var webAuth
    @Dependency(\.appSettings) var appSettings
    @Dependency(\.telemetry) var telemetry
    @Dependency(\.musicRoomAPI) var api

    public init() {}

    public var body: some ReducerOf<Self> {
        BindingReducer()
        Reduce { state, action in
            switch action {
            case .onAppear:
                let fetchStatsEffect = Effect<Action>.send(.fetchStats)
                let profileEffect = Effect<Action>.run { [userClient] send in
                    await send(.profileResponse(TaskResult { try await userClient.me() }))
                }

                if state.userProfile == nil {
                    state.isLoading = true
                    state.errorMessage = nil
                }

                return .merge(fetchStatsEffect, profileEffect)

            case .profileResponse(.success(let profile)):
                state.isLoading = false
                state.userProfile = normalizeAvatarUrl(profile)
                state.editableDisplayName = profile.displayName
                state.editableUsername = profile.username
                state.editableEmail = profile.email ?? ""
                state.editableMusicPreferences =
                    profile.preferences.genres?.joined(separator: ",") ?? ""
                state.hasLoaded = true
                return .none

            case .profileResponse(.failure(let error)):
                state.isLoading = false
                if state.userProfile == nil {
                    state.errorMessage = error.localizedDescription
                }
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
                    isPremium: currentProfile.isPremium,
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
                return .run { [userId = profile.userId, telemetry] _ in
                    await telemetry.log("user.profile.update.success", ["userId": userId])
                }

            case .updateProfileResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Failed to update profile: \(error.localizedDescription)"
                return .none

            case .logoutButtonTapped:
                return .run { [authClient] _ in
                    await authClient.logout()
                }

            case .forgotPasswordButtonTapped:
                if state.isOffline {
                    state.errorMessage = "You cannot reset your password while offline."
                    return .none
                }
                let email = (state.userProfile?.email ?? state.editableEmail)
                    .trimmingCharacters(in: .whitespacesAndNewlines)
                guard !email.isEmpty else {
                    state.errorMessage = "No email address found for this account."
                    return .none
                }
                state.alert = AlertState {
                    TextState("Reset Password")
                } actions: {
                    ButtonState(role: .destructive, action: .confirmPasswordReset) {
                        TextState("Reset")
                    }
                    ButtonState(role: .cancel) {
                        TextState("Cancel")
                    }
                } message: {
                    TextState("Send password reset instructions to \(email)?")
                }
                return .none

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
                return .run { [userId = profile.userId, telemetry] _ in
                    await telemetry.log("user.account.link.success", ["userId": userId])
                }

            case .linkAccountResponse(.failure(let error)):
                state.isLoading = false

                if let error = error as? ASWebAuthenticationSessionError,
                    error.code == .canceledLogin
                {
                    return .none
                }

                // Check also for NSError in case it's bridged
                let nsError = error as NSError
                if nsError.domain == ASWebAuthenticationSessionError.errorDomain
                    && nsError.code == ASWebAuthenticationSessionError.canceledLogin.rawValue
                {
                    return .none
                }

                // Handle Conflict (409) specifically
                if let userError = error as? UserClientError,
                    case .serverError(let statusCode) = userError,
                    statusCode == 409
                {
                    state.errorMessage =
                        "This account is already linked to another user. Please log in to that account to unlink it first."
                    return .none
                }

                // Generic error handling
                state.errorMessage = "Failed to link/unlink account: \(error.localizedDescription)"
                return .none

            case .alert(.presented(.confirmPasswordReset)):
                if state.isOffline {
                    state.errorMessage = "You cannot reset your password while offline."
                    return .none
                }
                let email = (state.userProfile?.email ?? state.editableEmail)
                    .trimmingCharacters(in: .whitespacesAndNewlines)
                guard !email.isEmpty else {
                    state.errorMessage = "No email address found for this account."
                    return .none
                }
                state.isLoading = true
                state.errorMessage = nil
                return .run { [authClient, email] send in
                    do {
                        try await authClient.forgotPassword(email)
                        await send(.forgotPasswordResponse(.success(true)))
                    } catch {
                        await send(.forgotPasswordResponse(.failure(error)))
                    }
                    await send(.logoutButtonTapped)
                }

            case .forgotPasswordResponse(.success):
                state.isLoading = false
                return .none

            case .forgotPasswordResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Failed to reset password: \(error.localizedDescription)"
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
                        isPremium: normalized.isPremium,
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

            case .becomePremiumTapped:
                state.isLoading = true
                return .run { [userClient] send in
                    await send(
                        .becomePremiumResponse(TaskResult { try await userClient.becomePremium() }))
                }

            case .becomePremiumResponse(.success(let profile)):
                state.isLoading = false
                state.userProfile = normalizeAvatarUrl(profile)
                return .none

            case .becomePremiumResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Premium activation failed: \(error.localizedDescription)"
                return .none

            case .toggleImagePlayground(let isPresented):
                state.isImagePlaygroundPresented = isPresented
                return .none

            case .imagePlaygroundResponse(let url):
                state.isImagePlaygroundPresented = false
                guard let url = url else { return .none }
                guard !state.isAvatarLoading else { return .none }
                state.isAvatarLoading = true

                return .run { send in
                    do {
                        let rawData: Data
                        if url.isFileURL {
                            rawData = try await Task.detached(priority: .userInitiated) {
                                try Data(contentsOf: url)
                            }.value
                        } else {
                            let (data, _) = try await URLSession.shared.data(from: url)
                            rawData = data
                        }

                        let jpegData = try await Task.detached(priority: .userInitiated) {
                            guard let image = UIImage(data: rawData),
                                let jpegData = image.jpegData(compressionQuality: 0.8)
                            else {
                                throw URLError(.cannotDecodeContentData)
                            }
                            return jpegData
                        }.value

                        await send(.uploadGeneratedAvatar(jpegData))
                    } catch {
                        await send(.uploadGeneratedAvatarResponse(.failure(error)))
                    }
                }

            case .uploadGeneratedAvatar(let data):
                return .run { [userClient] send in
                    await send(
                        .uploadGeneratedAvatarResponse(
                            TaskResult { try await userClient.uploadAvatar(data) }
                        )
                    )
                }

            case .uploadGeneratedAvatarResponse(.success(let profile)):
                state.isAvatarLoading = false
                state.userProfile = normalizeAvatarUrl(profile)
                return .none

            case .uploadGeneratedAvatarResponse(.failure(let error)):
                state.isAvatarLoading = false
                state.errorMessage = "Failed to upload AI avatar: \(error.localizedDescription)"
                return .none

            case .binding:
                return .none

            case .fetchStats:
                return .run { send in
                    await send(.statsResponse(TaskResult { try await api.getStats() }))
                }

            case .statsResponse(.success(let stats)):
                state.userStats = stats
                return .none

            case .statsResponse(.failure):
                // stats failure shouldn't block profile, maybe just log or ignore
                return .none

            case .alert:
                return .none
            }
        }
        .ifLet(\.$alert, action: \.alert)
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
            isPremium: profile.isPremium,
            linkedProviders: profile.linkedProviders,
            email: profile.email
        )
    }
}

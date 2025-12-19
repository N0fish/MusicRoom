import AppSettingsClient
import AppSupportClients
import ComposableArchitecture
import SwiftUI

@Reducer
public struct FriendProfileFeature {
    @ObservableState
    public struct State: Equatable, Identifiable, Sendable {
        public var id: String { userId }
        public let userId: String
        public var profile: PublicUserProfile?
        public var isFriend: Bool
        public var isMe: Bool = false
        public var isLoading: Bool = false
        @Presents public var alert: AlertState<Action.Alert>?

        public init(userId: String, isFriend: Bool, profile: PublicUserProfile? = nil) {
            self.userId = userId
            self.isFriend = isFriend
            self.profile = profile
        }
    }

    public enum Action: ViewAction, Equatable, Sendable {
        case view(View)
        case delegate(Delegate)
        case alert(PresentationAction<Alert>)
        case profileLoaded(TaskResult<PublicUserProfile>)
        case userLoaded(TaskResult<UserProfile>)

        public enum View: Equatable, Sendable {
            case onAppear
            case removeFriendTapped
            case addFriendTapped
        }

        public enum Delegate: Equatable, Sendable {
            case friendRemoved(String)
            case friendRequestSent(String)
        }

        public enum Alert: Equatable, Sendable {
            case confirmRemoval
        }
    }

    @Dependency(\.friendsClient) var friendsClient
    @Dependency(\.user) var userClient
    @Dependency(\.dismiss) var dismiss
    @Dependency(\.appSettings) var appSettings

    public init() {}

    public var body: some ReducerOf<Self> {
        Reduce { state, action in
            switch action {
            case .view(.onAppear):
                state.isLoading = true
                return .run { [friendsClient, userClient, userId = state.userId] send in
                    await send(
                        .profileLoaded(
                            TaskResult {
                                try await friendsClient.getProfile(userId)
                            }))

                    await send(
                        .userLoaded(
                            TaskResult {
                                try await userClient.me()
                            }))
                }

            case .profileLoaded(.success(let profile)):
                state.isLoading = false
                state.profile = normalizeAvatarUrl(profile)
                return .none

            case .profileLoaded(.failure):
                state.isLoading = false
                return .none

            case .userLoaded(.success(let user)):
                if user.userId == state.userId {
                    state.isMe = true
                }
                return .none

            case .userLoaded(.failure):
                return .none

            case .view(.addFriendTapped):
                state.isLoading = true
                return .run { [friendsClient, dismiss, userId = state.userId] send in
                    try await friendsClient.sendRequest(userId)
                    await send(.delegate(.friendRequestSent(userId)))
                    await dismiss()
                } catch: { error, _ in
                    print("Error sending request: \(error)")
                }

            case .view(.removeFriendTapped):
                let displayName = state.profile?.displayName ?? "this user"
                state.alert = AlertState {
                    TextState("Remove Friend")
                } actions: {
                    ButtonState(role: .destructive, action: .confirmRemoval) {
                        TextState("Remove")
                    }
                    ButtonState(role: .cancel) {
                        TextState("Cancel")
                    }
                } message: {
                    TextState(
                        "Are you sure you want to remove \(displayName) from your friends?"
                    )
                }
                return .none

            case .alert(.presented(.confirmRemoval)):
                state.isLoading = true
                return .run { [friendsClient, dismiss, userId = state.userId] send in
                    try await friendsClient.removeFriend(userId)
                    await send(.delegate(.friendRemoved(userId)))
                    await dismiss()
                } catch: { [dismiss] error, _ in
                    print("Error removing friend: \(error)")
                    await dismiss()
                }

            case .alert, .delegate:
                return .none
            }
        }
        .ifLet(\.$alert, action: \.alert)
    }

    private func normalizeAvatarUrl(_ profile: PublicUserProfile) -> PublicUserProfile {
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

        return PublicUserProfile(
            userId: profile.userId,
            username: profile.username,
            displayName: profile.displayName,
            avatarUrl: fullUrl,
            bio: profile.bio,
            visibility: profile.visibility,
            preferences: profile.preferences
        )
    }
}

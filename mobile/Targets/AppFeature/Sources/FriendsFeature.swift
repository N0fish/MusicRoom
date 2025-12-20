import AppSettingsClient
import AppSupportClients
import ComposableArchitecture
import EventFeature
import Foundation

@Reducer
public struct FriendsFeature: Sendable {
    public enum Segment: String, CaseIterable, Identifiable, Sendable {
        case friends = "My Friends"
        case requests = "Requests"
        case search = "Search"

        public var id: String { rawValue }
    }

    @ObservableState
    public struct State: Equatable, Sendable {
        public var selectedSegment: Segment = .friends
        public var friends: [Friend] = []
        public var incomingRequests: [FriendRequest] = []
        public var searchResults: [Friend] = []
        public var searchQuery: String = ""
        public var isLoading: Bool = false
        public var errorMessage: String?
        public var hasLoaded: Bool = false
        public var currentUserId: String?

        // Navigation
        public var path = StackState<FriendProfileFeature.State>()

        public init() {}
    }

    public enum Action: BindableAction, Sendable, Equatable {
        case binding(BindingAction<State>)
        case onAppear
        case segmentChanged(Segment)
        case loadData
        case friendsLoaded(Result<[Friend], EquatableError>)
        case requestsLoaded(Result<[FriendRequest], EquatableError>)
        case currentUserLoaded(Result<String, EquatableError>)

        // Search Actions
        case performSearch
        case searchResultsLoaded(Result<[Friend], EquatableError>)

        // Management Actions
        case sendRequest(String)  // userID
        case acceptRequest(String)  // senderID (which is usually userID)
        case rejectRequest(String)  // senderID
        // case removeFriend(String)  // friendID - Handled via Profile Delegate now or direct action? Keeping for list swipe delete if needed, but user wants to remove "automatic delete on tap".
        case removeFriend(String)
        case friendTapped(Friend)

        // Response Actions
        case requestSent(Result<EmptyResponse, EquatableError>)
        case requestAccepted(Result<EmptyResponse, EquatableError>)
        case requestRejected(Result<EmptyResponse, EquatableError>)
        case friendRemoved(Result<EmptyResponse, EquatableError>)

        case clearError

        // Navigation
        case path(StackAction<FriendProfileFeature.State, FriendProfileFeature.Action>)
        case searchUserTapped(Friend)
    }

    public struct EquatableError: Error, Equatable, Sendable {
        public let message: String
        public init(_ error: Error) {
            self.message = error.localizedDescription
        }
    }

    public struct EmptyResponse: Equatable, Sendable {}

    @Dependency(\.friendsClient) var friendsClient
    @Dependency(\.user) var userClient
    @Dependency(\.appSettings) var appSettings

    public init() {}

    public var body: some ReducerOf<Self> {
        BindingReducer()
        Reduce { (state: inout State, action: Action) -> Effect<Action> in
            switch action {
            case .binding:
                // Handle search query changes
                if state.searchQuery.isEmpty {
                    state.searchResults = []
                }
                return .none

            case .onAppear:
                guard !state.hasLoaded else { return .none }
                return .send(.loadData)

            case .segmentChanged(let segment):
                state.selectedSegment = segment
                if segment == .requests && state.incomingRequests.isEmpty {
                    return .send(.loadData)
                }
                if segment == .friends && state.friends.isEmpty {
                    return .send(.loadData)
                }
                return .none

            case .loadData:
                state.isLoading = true
                state.errorMessage = nil
                return .run { [userClient] send in
                    await withThrowingTaskGroup(of: Void.self) { group in
                        group.addTask {
                            do {
                                let friends = try await friendsClient.listFriends()
                                await send(.friendsLoaded(.success(friends)))
                            } catch {
                                await send(.friendsLoaded(.failure(EquatableError(error))))
                            }
                        }
                        group.addTask {
                            do {
                                let requests = try await friendsClient.incomingRequests()
                                await send(.requestsLoaded(.success(requests)))
                            } catch {
                                await send(.requestsLoaded(.failure(EquatableError(error))))
                            }
                        }
                        group.addTask {
                            do {
                                let me = try await userClient.me()
                                await send(.currentUserLoaded(.success(me.userId)))
                            } catch {
                                await send(.currentUserLoaded(.failure(EquatableError(error))))
                            }
                        }
                    }
                }

            case .currentUserLoaded(.success(let userId)):
                state.currentUserId = userId
                return .none

            case .currentUserLoaded(.failure):
                return .none

            case .friendsLoaded(.success(let friends)):
                state.isLoading = false
                state.friends = friends.map { friend in
                    Friend(
                        id: friend.id,
                        userId: friend.userId,
                        username: friend.username,
                        displayName: friend.displayName,
                        avatarUrl: normalizeUrl(friend.avatarUrl),
                        isPremium: friend.isPremium
                    )
                }
                state.hasLoaded = true
                return .none

            case .friendsLoaded(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Failed to load friends: \(error.message)"
                return .none

            case .requestsLoaded(.success(let requests)):
                state.incomingRequests = requests.map { req in
                    FriendRequest(
                        id: req.id,
                        senderId: req.senderId,
                        senderUsername: req.senderUsername,
                        senderDisplayName: req.senderDisplayName,
                        senderAvatarUrl: normalizeUrl(req.senderAvatarUrl),
                        senderIsPremium: req.senderIsPremium,
                        status: req.status,
                        sentAt: req.sentAt
                    )
                }
                return .none

            case .requestsLoaded(.failure(let error)):
                state.errorMessage = "Failed to load requests: \(error.message)"
                return .none

            case .performSearch:
                guard !state.searchQuery.isEmpty else { return .none }
                state.isLoading = true
                return .run { [query = state.searchQuery] send in
                    do {
                        let results = try await friendsClient.searchUsers(query)
                        await send(.searchResultsLoaded(.success(results)))
                    } catch {
                        await send(.searchResultsLoaded(.failure(EquatableError(error))))
                    }
                }

            case .searchResultsLoaded(.success(let results)):
                state.isLoading = false
                state.searchResults = results.map { friend in
                    Friend(
                        id: friend.id,
                        userId: friend.userId,
                        username: friend.username,
                        displayName: friend.displayName,
                        avatarUrl: normalizeUrl(friend.avatarUrl),
                        isPremium: friend.isPremium
                    )
                }
                return .none

            case .searchResultsLoaded(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Search failed: \(error.message)"
                return .none

            case .sendRequest(let userId):
                // Optimistically remove from search results
                state.searchResults.removeAll { $0.userId == userId || $0.id == userId }

                return .run { send in
                    do {
                        try await friendsClient.sendRequest(userId)
                        await send(.requestSent(.success(EmptyResponse())))
                    } catch {
                        await send(.requestSent(.failure(EquatableError(error))))
                    }
                }

            case .requestSent(.success):
                return .none

            case .requestSent(.failure(let error)):
                state.errorMessage = "Failed to send request: \(error.message)"
                return .none

            case .acceptRequest(let senderId):
                return .run { send in
                    do {
                        try await friendsClient.acceptRequest(senderId)
                        await send(.requestAccepted(.success(EmptyResponse())))
                    } catch {
                        await send(.requestAccepted(.failure(EquatableError(error))))
                    }
                }

            case .requestAccepted(.success):
                return .send(.loadData)

            case .requestAccepted(.failure(let error)):
                state.errorMessage = "Failed to accept: \(error.message)"
                return .none

            case .rejectRequest(let senderId):
                return .run { send in
                    do {
                        try await friendsClient.rejectRequest(senderId)
                        await send(.requestRejected(.success(EmptyResponse())))
                    } catch {
                        await send(.requestRejected(.failure(EquatableError(error))))
                    }
                }

            case .requestRejected(.success):
                return .send(.loadData)

            case .requestRejected(.failure(let error)):
                state.errorMessage = "Failed to reject: \(error.message)"
                return .none

            case .removeFriend(let friendId):
                return .run { send in
                    do {
                        try await friendsClient.removeFriend(friendId)
                        await send(.friendRemoved(.success(EmptyResponse())))
                    } catch {
                        await send(.friendRemoved(.failure(EquatableError(error))))
                    }
                }

            case .friendTapped(let friend):
                // Map Friend to PublicUserProfile if needed, or just pass ID
                let profile = PublicUserProfile(
                    userId: friend.userId,
                    username: friend.username,
                    displayName: friend.displayName,
                    avatarUrl: friend.avatarUrl,
                    isPremium: friend.isPremium,
                    bio: nil,
                    visibility: "public",
                    preferences: nil
                )
                state.path.append(
                    FriendProfileFeature.State(
                        userId: friend.userId, isFriend: true, profile: profile))
                return .none

            case .searchUserTapped(let user):
                // Check if actually friend
                let isFriend = state.friends.contains(where: { $0.userId == user.userId })
                // Pre-fill profile if possible
                let profile = PublicUserProfile(
                    userId: user.userId,
                    username: user.username,
                    displayName: user.displayName,
                    avatarUrl: user.avatarUrl,
                    isPremium: user.isPremium,
                    bio: nil,
                    visibility: "public",
                    preferences: nil
                )
                state.path.append(
                    FriendProfileFeature.State(
                        userId: user.userId, isFriend: isFriend, profile: profile))
                return .none

            case .friendRemoved(.success):
                return .send(.loadData)

            case .friendRemoved(.failure(let error)):
                state.errorMessage = "Failed to remove friend: \(error.message)"
                return .none

            case .clearError:
                state.errorMessage = nil
                return .none

            case .path(.element(id: _, action: .delegate(.friendRemoved))):
                // Friend removed from profile view, reload list
                return .send(.loadData)

            case .path:
                return .none
            }
        }
        .forEach(\.path, action: \.path) {
            FriendProfileFeature()
        }
    }

    private func normalizeUrl(_ url: String?) -> String? {
        guard let url, !url.isEmpty else { return url }
        if url.lowercased().hasPrefix("http") { return url }
        let settings = appSettings.load()
        let baseUrlString = settings.backendURL.absoluteString.trimmingCharacters(
            in: .init(charactersIn: "/"))
        let cleanPath = url.trimmingCharacters(in: .init(charactersIn: "/"))
        return "\(baseUrlString)/\(cleanPath)"
    }
}

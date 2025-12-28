import AppSettingsClient
import Dependencies
import Foundation

// MARK: - Models

public struct Friend: Codable, Equatable, Identifiable, Sendable {
    public let id: String
    public let userId: String
    public let username: String
    public let displayName: String
    public let avatarUrl: String?
    public let isPremium: Bool

    public init(
        id: String, userId: String, username: String, displayName: String, avatarUrl: String?,
        isPremium: Bool
    ) {
        self.id = id
        self.userId = userId
        self.username = username
        self.displayName = displayName
        self.avatarUrl = avatarUrl
        self.isPremium = isPremium
    }
}

public struct FriendRequest: Codable, Equatable, Identifiable, Sendable {
    public let id: String  // Usually same as sender user ID in this API design
    public let senderId: String
    public let senderUsername: String
    public let senderDisplayName: String
    public let senderAvatarUrl: String?
    public let senderIsPremium: Bool
    public let status: String
    public let sentAt: Date

    public init(
        id: String, senderId: String, senderUsername: String, senderDisplayName: String,
        senderAvatarUrl: String?, senderIsPremium: Bool, status: String, sentAt: Date
    ) {
        self.id = id
        self.senderId = senderId
        self.senderUsername = senderUsername
        self.senderDisplayName = senderDisplayName
        self.senderAvatarUrl = senderAvatarUrl
        self.senderIsPremium = senderIsPremium
        self.status = status
        self.sentAt = sentAt
    }
}

// MARK: - API Response Models (Internal)

struct UserListItem: Codable, Sendable {
    let userId: String
    let username: String
    let displayName: String
    let avatarUrl: String?
    let isPremium: Bool

    func toFriend() -> Friend {
        Friend(
            id: userId,
            userId: userId,
            username: username,
            displayName: displayName,
            avatarUrl: avatarUrl,
            isPremium: isPremium
        )
    }
}

struct FriendsListResponse: Codable, Sendable {
    let items: [UserListItem]?
}

struct BackendFriendItem: Codable, Sendable {
    let userId: String
    let username: String
    let displayName: String
    let avatarUrl: String?
    let isPremium: Bool
}

struct IncomingRequestItem: Codable, Sendable {
    let from: BackendFriendItem
    let createdAt: String  // Default Go JSON marshaling for time.Time is ISO8601 string
}

struct IncomingRequestsResponse: Codable, Sendable {
    let items: [IncomingRequestItem]?
}

// MARK: - Client Interface

public struct FriendsClient: Sendable {
    public var listFriends: @Sendable () async throws -> [Friend]
    public var incomingRequests: @Sendable () async throws -> [FriendRequest]
    public var sendRequest: @Sendable (_ userId: String) async throws -> Void
    public var acceptRequest: @Sendable (_ senderId: String) async throws -> Void
    public var rejectRequest: @Sendable (_ senderId: String) async throws -> Void
    public var removeFriend: @Sendable (_ friendId: String) async throws -> Void
    public var searchUsers: @Sendable (_ query: String) async throws -> [Friend]
    public var getProfile: @Sendable (_ userId: String) async throws -> PublicUserProfile
}

// MARK: - Dependency Key

extension FriendsClient: DependencyKey {
    public static let liveValue = FriendsClient.live()

    public static let testValue = FriendsClient(
        listFriends: { [] },
        incomingRequests: { [] },
        sendRequest: { _ in },
        acceptRequest: { _ in },
        rejectRequest: { _ in },
        removeFriend: { _ in },
        searchUsers: { _ in [] },
        getProfile: { _ in
            PublicUserProfile(
                userId: "test", username: "test", displayName: "test",
                avatarUrl: nil, isPremium: false, bio: nil, visibility: "public", preferences: nil
            )
        }
    )

    public static let previewValue = FriendsClient(
        listFriends: {
            [
                Friend(
                    id: "1", userId: "u1", username: "alice", displayName: "Alice Wonderland",
                    avatarUrl: nil, isPremium: true),
                Friend(
                    id: "2", userId: "u2", username: "bob", displayName: "Bob Builder",
                    avatarUrl: nil, isPremium: false),
            ]
        },
        incomingRequests: {
            [
                FriendRequest(
                    id: "3", senderId: "u3", senderUsername: "charlie",
                    senderDisplayName: "Charlie C", senderAvatarUrl: nil, senderIsPremium: false,
                    status: "pending",
                    sentAt: Date())
            ]
        },
        sendRequest: { _ in },
        acceptRequest: { _ in },
        rejectRequest: { _ in },
        removeFriend: { _ in },
        searchUsers: { _ in [] },
        getProfile: { _ in
            PublicUserProfile(
                userId: "u1", username: "alice", displayName: "Alice Wonderland",
                avatarUrl: nil, isPremium: true, bio: "Test Bio", visibility: "public",
                preferences: nil
            )
        }
    )
}

extension DependencyValues {
    public var friendsClient: FriendsClient {
        get { self[FriendsClient.self] }
        set { self[FriendsClient.self] = newValue }
    }
}

// MARK: - Live Implementation

extension FriendsClient {
    static func live(urlSession: URLSession = .shared) -> Self {
        @Dependency(\.appSettings) var appSettings
        @Dependency(\.authentication) var authentication
        let executor = AuthenticatedRequestExecutor(urlSession: urlSession, authentication: authentication)
        let friendsCache = FriendsListCache(ttl: 5)
        let requestsCache = IncomingRequestsCache(ttl: 5)

        @Sendable func logError(
            _ request: URLRequest, _ response: HTTPURLResponse?, _ data: Data?, _ error: Error?
        ) {
            print("\n❌ [FriendsClient] Request Failed")
            print("   URL: \(request.url?.absoluteString ?? "nil")")
            print("   Method: \(request.httpMethod ?? "GET")")
            if let response {
                print("   Status: \(response.statusCode)")
            }
            if let data, let body = String(data: data, encoding: .utf8), !body.isEmpty {
                print("   Response Body: \(body)")
            }
            if let error {
                print("   Error: \(error.localizedDescription)")
            }
            print("--------------------------------------------------\n")
        }

        @Sendable func baseURLString() -> String {
            appSettings.load().backendURLString
        }

        @Sendable func performRequest<T: Decodable & Sendable>(
            _ request: URLRequest
        ) async throws -> T {
            do {
                let (data, httpResponse) = try await executor.data(for: request)

                if !(200...299).contains(httpResponse.statusCode) {
                    let error = URLError(.badServerResponse)  // Or custom error
                    logError(request, httpResponse, data, error)
                    throw error
                }

                let decoder = JSONDecoder()
                do {
                    return try decoder.decode(T.self, from: data)
                } catch {
                    logError(request, httpResponse, data, error)
                    throw error
                }
            } catch {
                logError(request, nil, nil, error)
                throw error
            }
        }

        @Sendable func performRequestNoContent(_ request: URLRequest) async throws {
            do {
                let (data, httpResponse) = try await executor.data(for: request)

                if !(200...299).contains(httpResponse.statusCode) {
                    let error = URLError(.badServerResponse)
                    logError(request, httpResponse, data, error)
                    throw error
                }
            } catch {
                logError(request, nil, nil, error)
                throw error
            }
        }

        return Self(
            listFriends: {
                let token = authentication.getAccessToken()
                return try await friendsCache.get(token: token) {
                    let baseUrl = baseURLString()
                    let url = URL(string: "\(baseUrl)/users/me/friends")!
                    var request = URLRequest(url: url)
                    request.httpMethod = "GET"

                    let list: FriendsListResponse = try await performRequest(request)
                    return (list.items ?? []).map { item in
                        var friend = item.toFriend()
                        if let avatarUrl = friend.avatarUrl, !avatarUrl.hasPrefix("http") {
                            friend = Friend(
                                id: friend.id,
                                userId: friend.userId,
                                username: friend.username,
                                displayName: friend.displayName,
                                avatarUrl: baseUrl + avatarUrl,
                                isPremium: friend.isPremium
                            )
                        }
                        return friend
                    }
                }
            },
            incomingRequests: {
                let token = authentication.getAccessToken()
                return try await requestsCache.get(token: token) {
                    let baseUrl = baseURLString()
                    let url = URL(string: "\(baseUrl)/users/me/friends/requests/incoming")!
                    var request = URLRequest(url: url)
                    request.httpMethod = "GET"

                    let list: IncomingRequestsResponse = try await performRequest(request)
                    return (list.items ?? []).map { item in
                        let date = ISO8601DateFormatter().date(from: item.createdAt) ?? Date()
                        var avatarUrl = item.from.avatarUrl
                        if let url = avatarUrl, !url.hasPrefix("http") {
                            avatarUrl = baseUrl + url
                        }

                        return FriendRequest(
                            id: item.from.userId,
                            senderId: item.from.userId,
                            senderUsername: item.from.username,
                            senderDisplayName: item.from.displayName,
                            senderAvatarUrl: avatarUrl,
                            senderIsPremium: item.from.isPremium,
                            status: "pending",
                            sentAt: date
                        )
                    }
                }
            },
            sendRequest: { userId in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/me/friends/\(userId)/request")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                try await performRequestNoContent(request)
                await requestsCache.invalidate()
            },
            acceptRequest: { senderId in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/me/friends/\(senderId)/accept")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                try await performRequestNoContent(request)
                await friendsCache.invalidate()
                await requestsCache.invalidate()
            },
            rejectRequest: { senderId in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/me/friends/\(senderId)/reject")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                try await performRequestNoContent(request)
                await requestsCache.invalidate()
            },
            removeFriend: { friendId in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/me/friends/\(friendId)")!
                var request = URLRequest(url: url)
                request.httpMethod = "DELETE"
                try await performRequestNoContent(request)
                await friendsCache.invalidate()
            },
            searchUsers: { query in
                guard !query.isEmpty else { return [] }
                let baseUrl = baseURLString()
                var components = URLComponents(string: "\(baseUrl)/users/search")!
                components.queryItems = [URLQueryItem(name: "query", value: query)]

                var request = URLRequest(url: components.url!)
                request.httpMethod = "GET"

                let list: FriendsListResponse = try await performRequest(request)
                return (list.items ?? []).map { item in
                    var friend = item.toFriend()
                    if let avatarUrl = friend.avatarUrl, !avatarUrl.hasPrefix("http") {
                        friend = Friend(
                            id: friend.id,
                            userId: friend.userId,
                            username: friend.username,
                            displayName: friend.displayName,
                            avatarUrl: baseUrl + avatarUrl,
                            isPremium: friend.isPremium
                        )
                    }
                    return friend
                }
            },
            getProfile: { userId in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/\(userId)")!
                var request = URLRequest(url: url)
                request.httpMethod = "GET"

                var profile: PublicUserProfile = try await performRequest(request)
                if let avatarUrl = profile.avatarUrl, !avatarUrl.hasPrefix("http") {
                    profile = PublicUserProfile(
                        userId: profile.userId,
                        username: profile.username,
                        displayName: profile.displayName,
                        avatarUrl: baseUrl + avatarUrl,
                        isPremium: profile.isPremium,
                        bio: profile.bio,
                        visibility: profile.visibility,
                        preferences: profile.preferences
                    )
                }
                return profile
            }
        )
    }

}

private actor FriendsListCache {
    private struct Entry {
        let value: [Friend]
        let token: String?
        let fetchedAt: Date
    }

    private let ttl: TimeInterval
    private var entry: Entry?
    private var inFlight: Task<[Friend], Error>?
    private var inFlightToken: String?

    init(ttl: TimeInterval) {
        self.ttl = ttl
    }

    func get(
        token: String?,
        fetch: @escaping @Sendable () async throws -> [Friend]
    ) async throws -> [Friend] {
        let now = Date()
        if let entry, entry.token == token, now.timeIntervalSince(entry.fetchedAt) < ttl {
            return entry.value
        }

        if entry?.token != token {
            entry = nil
            if let currentInFlight = inFlight, inFlightToken != token {
                currentInFlight.cancel()
                inFlight = nil
                inFlightToken = nil
            }
        }

        if let inFlight, inFlightToken == token {
            return try await inFlight.value
        }

        let task = Task {
            try await fetch()
        }
        inFlight = task
        inFlightToken = token
        let result = await task.result
        inFlight = nil
        inFlightToken = nil

        switch result {
        case .success(let value):
            entry = Entry(value: value, token: token, fetchedAt: Date())
            return value
        case .failure(let error):
            throw error
        }
    }

    func invalidate() {
        entry = nil
        inFlight?.cancel()
        inFlight = nil
        inFlightToken = nil
    }
}

private actor IncomingRequestsCache {
    private struct Entry {
        let value: [FriendRequest]
        let token: String?
        let fetchedAt: Date
    }

    private let ttl: TimeInterval
    private var entry: Entry?
    private var inFlight: Task<[FriendRequest], Error>?
    private var inFlightToken: String?

    init(ttl: TimeInterval) {
        self.ttl = ttl
    }

    func get(
        token: String?,
        fetch: @escaping @Sendable () async throws -> [FriendRequest]
    ) async throws -> [FriendRequest] {
        let now = Date()
        if let entry, entry.token == token, now.timeIntervalSince(entry.fetchedAt) < ttl {
            return entry.value
        }

        if entry?.token != token {
            entry = nil
            if let currentInFlight = inFlight, inFlightToken != token {
                currentInFlight.cancel()
                inFlight = nil
                inFlightToken = nil
            }
        }

        if let inFlight, inFlightToken == token {
            return try await inFlight.value
        }

        let task = Task {
            try await fetch()
        }
        inFlight = task
        inFlightToken = token
        let result = await task.result
        inFlight = nil
        inFlightToken = nil

        switch result {
        case .success(let value):
            entry = Entry(value: value, token: token, fetchedAt: Date())
            return value
        case .failure(let error):
            throw error
        }
    }

    func invalidate() {
        entry = nil
        inFlight?.cancel()
        inFlight = nil
        inFlightToken = nil
    }
}

// MARK: - Public Profile Model
public struct PublicUserProfile: Codable, Equatable, Sendable {
    public let userId: String
    public let username: String
    public let displayName: String
    public let avatarUrl: String?
    public let isPremium: Bool
    public let bio: String?
    public let visibility: String
    public let preferences: PublicMusicPreferences?  // Use optional if it might be missing or define struct

    public init(
        userId: String,
        username: String,
        displayName: String,
        avatarUrl: String?,
        isPremium: Bool,
        bio: String?,
        visibility: String,
        preferences: PublicMusicPreferences?
    ) {
        self.userId = userId
        self.username = username
        self.displayName = displayName
        self.avatarUrl = avatarUrl
        self.isPremium = isPremium
        self.bio = bio
        self.visibility = visibility
        self.preferences = preferences
    }
}

public struct PublicMusicPreferences: Codable, Equatable, Sendable {
    public var genres: [String]?
    public var artists: [String]?
    public var moods: [String]?

    public init(genres: [String]? = nil, artists: [String]? = nil, moods: [String]? = nil) {
        self.genres = genres
        self.artists = artists
        self.moods = moods
    }
}

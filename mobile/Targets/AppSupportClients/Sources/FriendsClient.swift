import Dependencies
import Foundation

// MARK: - Models

public struct Friend: Codable, Equatable, Identifiable, Sendable {
    public let id: String
    public let userId: String
    public let username: String
    public let displayName: String
    public let avatarUrl: String?

    public init(
        id: String, userId: String, username: String, displayName: String, avatarUrl: String?
    ) {
        self.id = id
        self.userId = userId
        self.username = username
        self.displayName = displayName
        self.avatarUrl = avatarUrl
    }
}

public struct FriendRequest: Codable, Equatable, Identifiable, Sendable {
    public let id: String  // Usually same as sender user ID in this API design
    public let senderId: String
    public let senderUsername: String
    public let senderDisplayName: String
    public let senderAvatarUrl: String?
    public let status: String
    public let sentAt: Date

    public init(
        id: String, senderId: String, senderUsername: String, senderDisplayName: String,
        senderAvatarUrl: String?, status: String, sentAt: Date
    ) {
        self.id = id
        self.senderId = senderId
        self.senderUsername = senderUsername
        self.senderDisplayName = senderDisplayName
        self.senderAvatarUrl = senderAvatarUrl
        self.status = status
        self.sentAt = sentAt
    }
}

// MARK: - API Response Models (Internal)

struct UserListItem: Codable {
    let userId: String
    let username: String
    let displayName: String
    let avatarUrl: String?

    func toFriend() -> Friend {
        Friend(
            id: userId,
            userId: userId,
            username: username,
            displayName: displayName,
            avatarUrl: avatarUrl
        )
    }
}

struct FriendsListResponse: Codable {
    let items: [UserListItem]?
}

struct BackendFriendItem: Codable {
    let userId: String
    let username: String
    let displayName: String
    let avatarUrl: String?
}

struct IncomingRequestItem: Codable {
    let from: BackendFriendItem
    let createdAt: String  // Default Go JSON marshaling for time.Time is ISO8601 string
}

struct IncomingRequestsResponse: Codable {
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
                avatarUrl: nil, bio: nil, visibility: "public", preferences: nil
            )
        }
    )

    public static let previewValue = FriendsClient(
        listFriends: {
            [
                Friend(
                    id: "1", userId: "u1", username: "alice", displayName: "Alice Wonderland",
                    avatarUrl: nil),
                Friend(
                    id: "2", userId: "u2", username: "bob", displayName: "Bob Builder",
                    avatarUrl: nil),
            ]
        },
        incomingRequests: {
            [
                FriendRequest(
                    id: "3", senderId: "u3", senderUsername: "charlie",
                    senderDisplayName: "Charlie C", senderAvatarUrl: nil, status: "pending",
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
                avatarUrl: nil, bio: "Test Bio", visibility: "public", preferences: nil
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
    static func live() -> Self {
        return Self(
            listFriends: {
                let url = URL(string: "http://localhost:8080/users/me/friends")!
                var request = URLRequest(url: url)
                request.httpMethod = "GET"
                attachAuth(to: &request)

                let (data, response) = try await URLSession.shared.data(for: request)
                try validate(response: response)

                let list = try JSONDecoder().decode(FriendsListResponse.self, from: data)
                return (list.items ?? []).map { $0.toFriend() }
            },
            incomingRequests: {
                let url = URL(string: "http://localhost:8080/users/me/friends/requests/incoming")!
                var request = URLRequest(url: url)
                request.httpMethod = "GET"
                attachAuth(to: &request)

                let (data, response) = try await URLSession.shared.data(for: request)
                try validate(response: response)

                let decoder = JSONDecoder()

                let list = try decoder.decode(IncomingRequestsResponse.self, from: data)
                return (list.items ?? []).map { item in
                    // Flexible Date Parsing safely
                    let date = ISO8601DateFormatter().date(from: item.createdAt) ?? Date()

                    return FriendRequest(
                        id: item.from.userId,  // Use userId as request ID key since API doesn't return request ID here explicitly in "from"
                        senderId: item.from.userId,
                        senderUsername: item.from.username,
                        senderDisplayName: item.from.displayName,
                        senderAvatarUrl: item.from.avatarUrl,
                        status: "pending",  // Implicitly pending for incoming requests
                        sentAt: date
                    )
                }
            },
            sendRequest: { userId in
                let url = URL(string: "http://localhost:8080/users/me/friends/\(userId)/request")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                attachAuth(to: &request)

                let (_, response) = try await URLSession.shared.data(for: request)
                try validate(response: response)
            },
            acceptRequest: { senderId in
                let url = URL(string: "http://localhost:8080/users/me/friends/\(senderId)/accept")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                attachAuth(to: &request)

                let (_, response) = try await URLSession.shared.data(for: request)
                try validate(response: response)
            },
            rejectRequest: { senderId in
                let url = URL(string: "http://localhost:8080/users/me/friends/\(senderId)/reject")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                attachAuth(to: &request)

                let (_, response) = try await URLSession.shared.data(for: request)
                try validate(response: response)
            },
            removeFriend: { friendId in
                let url = URL(string: "http://localhost:8080/users/me/friends/\(friendId)")!
                var request = URLRequest(url: url)
                request.httpMethod = "DELETE"
                attachAuth(to: &request)

                let (_, response) = try await URLSession.shared.data(for: request)
                try validate(response: response)
            },
            searchUsers: { query in
                guard !query.isEmpty else { return [] }
                var components = URLComponents(string: "http://localhost:8080/users/search")!
                components.queryItems = [URLQueryItem(name: "query", value: query)]

                var request = URLRequest(url: components.url!)
                request.httpMethod = "GET"
                attachAuth(to: &request)

                let (data, response) = try await URLSession.shared.data(for: request)
                try validate(response: response)

                let list = try JSONDecoder().decode(FriendsListResponse.self, from: data)
                return (list.items ?? []).map { $0.toFriend() }
            },
            getProfile: { userId in
                let url = URL(string: "http://localhost:8080/users/\(userId)")!
                var request = URLRequest(url: url)
                request.httpMethod = "GET"
                attachAuth(to: &request)

                let (data, response) = try await URLSession.shared.data(for: request)
                try validate(response: response)

                return try JSONDecoder().decode(PublicUserProfile.self, from: data)
            }
        )
    }

    private static func attachAuth(to request: inout URLRequest) {
        if let token = KeychainHelper().read("accessToken") {
            request.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
    }

    private static func validate(response: URLResponse) throws {
        guard let httpResponse = response as? HTTPURLResponse,
            (200...299).contains(httpResponse.statusCode)
        else {
            throw URLError(.badServerResponse)
        }
    }
}

// MARK: - Public Profile Model
public struct PublicUserProfile: Codable, Equatable, Sendable {
    public let userId: String
    public let username: String
    public let displayName: String
    public let avatarUrl: String?
    public let bio: String?
    public let visibility: String
    public let preferences: PublicMusicPreferences?  // Use optional if it might be missing or define struct

    public init(
        userId: String,
        username: String,
        displayName: String,
        avatarUrl: String?,
        bio: String?,
        visibility: String,
        preferences: PublicMusicPreferences?
    ) {
        self.userId = userId
        self.username = username
        self.displayName = displayName
        self.avatarUrl = avatarUrl
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

// Private KeychainHelper helper to avoid dependency on AppSettings if circular
private struct KeychainHelper {
    func read(_ key: String) -> String? {
        let query =
            [
                kSecClass: kSecClassGenericPassword,
                kSecAttrAccount: key,
                kSecReturnData: true,
                kSecMatchLimit: kSecMatchLimitOne,
            ] as [String: Any]

        var dataTypeRef: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &dataTypeRef)

        if status == errSecSuccess, let data = dataTypeRef as? Data {
            return String(data: data, encoding: .utf8)
        }
        return nil
    }
}

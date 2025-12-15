import Dependencies
import Foundation

public struct UserPreferences: Codable, Equatable, Sendable {
    public var genres: [String]?
    public var artists: [String]?
    public var moods: [String]?

    public init(genres: [String]? = nil, artists: [String]? = nil, moods: [String]? = nil) {
        self.genres = genres
        self.artists = artists
        self.moods = moods
    }
}

public struct UserProfile: Codable, Equatable, Sendable {
    public let id: String
    public let userId: String
    public let username: String
    public let displayName: String
    public let avatarUrl: String?
    public let hasCustomAvatar: Bool
    public let bio: String?
    public let visibility: String
    public let preferences: UserPreferences
    // Fields not returned by /users/me, but used in UI.
    // We make them optional and mutable to potentialy fill them later or locally.
    public var linkedProviders: [String] = []
    public var email: String?

    private enum CodingKeys: String, CodingKey {
        case id, userId, username, displayName, avatarUrl, hasCustomAvatar, bio, visibility,
            preferences
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        userId = try container.decode(String.self, forKey: .userId)
        username = try container.decode(String.self, forKey: .username)
        displayName = try container.decode(String.self, forKey: .displayName)
        avatarUrl = try container.decodeIfPresent(String.self, forKey: .avatarUrl)
        hasCustomAvatar = try container.decode(Bool.self, forKey: .hasCustomAvatar)
        bio = try container.decodeIfPresent(String.self, forKey: .bio)
        visibility = try container.decode(String.self, forKey: .visibility)
        preferences = try container.decode(UserPreferences.self, forKey: .preferences)

        email = nil
        linkedProviders = []
    }

    public init(
        id: String,
        userId: String,
        username: String,
        displayName: String,
        avatarUrl: String?,
        hasCustomAvatar: Bool,
        bio: String? = nil,
        visibility: String = "public",
        preferences: UserPreferences = UserPreferences(),
        linkedProviders: [String] = [],
        email: String? = nil
    ) {
        self.id = id
        self.userId = userId
        self.username = username
        self.displayName = displayName
        self.avatarUrl = avatarUrl
        self.hasCustomAvatar = hasCustomAvatar
        self.bio = bio
        self.visibility = visibility
        self.preferences = preferences
        self.linkedProviders = linkedProviders
        self.email = email
    }
}

public struct UserClient: Sendable {
    public var me: @Sendable () async throws -> UserProfile
    public var updateProfile: @Sendable (UserProfile) async throws -> UserProfile
    public var link: @Sendable (String, String) async throws -> UserProfile
    public var unlink: @Sendable (String) async throws -> UserProfile
    public var changePassword: @Sendable (_ current: String, _ new: String) async throws -> Void
}

extension UserClient: DependencyKey {
    public static let liveValue = UserClient.live()

    public static let previewValue = UserClient(
        me: {
            UserProfile(
                id: "mock-id",
                userId: "mock-user-id",
                username: "Preview User",
                displayName: "Preview Display Name",
                avatarUrl: "",
                hasCustomAvatar: false,
                bio: "Music lover",
                visibility: "public",
                preferences: UserPreferences(genres: ["Pop", "Rock"]),
                linkedProviders: ["google"],
                email: "preview@example.com"
            )
        },
        updateProfile: { profile in
            return profile
        },
        link: { _, _ in
            UserProfile(
                id: "mock-id",
                userId: "mock-user-id",
                username: "Preview User",
                displayName: "Preview Display Name",
                avatarUrl: "",
                hasCustomAvatar: false,
                bio: "Music lover",
                visibility: "public",
                preferences: UserPreferences(genres: ["Pop", "Rock"]),
                linkedProviders: ["google", "42"],
                email: "preview@example.com"
            )
        },
        unlink: { _ in
            UserProfile(
                id: "mock-id",
                userId: "mock-user-id",
                username: "Preview User",
                displayName: "Preview Display Name",
                avatarUrl: "",
                hasCustomAvatar: false,
                bio: "Music lover",
                visibility: "public",
                preferences: UserPreferences(genres: ["Pop", "Rock"]),
                linkedProviders: [],
                email: "preview@example.com"
            )
        },
        changePassword: { _, _ in }
    )

    public static let testValue = UserClient(
        me: {
            UserProfile(
                id: "test-id",
                userId: "test-user-id",
                username: "Test User",
                displayName: "Test Display Name",
                avatarUrl: "",
                hasCustomAvatar: false,
                bio: nil,
                visibility: "public",
                preferences: UserPreferences(),
                linkedProviders: ["google"],
                email: "test@example.com"
            )
        },
        updateProfile: { profile in
            return profile
        },
        link: { _, _ in
            UserProfile(
                id: "test-id",
                userId: "test-user-id",
                username: "Test User",
                displayName: "Test Display Name",
                avatarUrl: "",
                hasCustomAvatar: false,
                bio: nil,
                visibility: "public",
                preferences: UserPreferences(),
                linkedProviders: ["google", "42"],
                email: "test@example.com"
            )
        },
        unlink: { _ in
            UserProfile(
                id: "test-id",
                userId: "test-user-id",
                username: "Test User",
                displayName: "Test Display Name",
                avatarUrl: "",
                hasCustomAvatar: false,
                bio: nil,
                visibility: "public",
                preferences: UserPreferences(),
                linkedProviders: [],
                email: "test@example.com"
            )
        },
        changePassword: { _, _ in }
    )
}

extension DependencyValues {
    public var user: UserClient {
        get { self[UserClient.self] }
        set { self[UserClient.self] = newValue }
    }
}

extension UserClient {
    // in UserClient.swift

    static func live() -> Self {
        return Self(
            me: {
                // 1. Fetch User Profile
                // TODO: Use configured base URL
                let urlProfile = URL(string: "http://localhost:8080/users/me")!
                var reqProfile = URLRequest(url: urlProfile)
                reqProfile.httpMethod = "GET"

                if let token = KeychainHelper().read("accessToken") {
                    reqProfile.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
                }

                let (dataProfile, respProfile) = try await URLSession.shared.data(for: reqProfile)

                guard let httpRespProfile = respProfile as? HTTPURLResponse,
                    (200...299).contains(httpRespProfile.statusCode)
                else {
                    throw URLError(.badServerResponse)
                }

                var profile = try JSONDecoder().decode(UserProfile.self, from: dataProfile)

                // 2. Fetch Auth Info (Linked Providers)
                let urlAuth = URL(string: "http://localhost:8080/auth/me")!
                var reqAuth = URLRequest(url: urlAuth)
                reqAuth.httpMethod = "GET"
                if let token = KeychainHelper().read("accessToken") {
                    reqAuth.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
                }

                // We try-catch auth fetch so we don't block profile load if auth service fails?
                // But user explicitly complained about linking. So failure should probably be visible.
                // However, failing entire profile load because auth/me failed might be harsh.
                // Let's try to fetch it, if it fails, default to empty.

                struct AuthMeResponse: Decodable {
                    let linkedProviders: [String]?
                }

                do {
                    let (dataAuth, respAuth) = try await URLSession.shared.data(for: reqAuth)
                    if let httpRespAuth = respAuth as? HTTPURLResponse,
                        (200...299).contains(httpRespAuth.statusCode)
                    {
                        let authData = try JSONDecoder().decode(AuthMeResponse.self, from: dataAuth)
                        profile.linkedProviders = authData.linkedProviders ?? []
                    }
                } catch {
                    print("UserClient: Failed to fetch auth/me: \(error)")
                    // Keep default empty linkedProviders
                }

                return profile
            },
            updateProfile: { profile in
                let url = URL(string: "http://localhost:8080/users/me")!
                var request = URLRequest(url: url)
                request.httpMethod = "PUT"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                if let token = KeychainHelper().read("accessToken") {
                    request.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
                }

                request.httpBody = try JSONEncoder().encode(profile)

                let (data, response) = try await URLSession.shared.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse,
                    (200...299).contains(httpResponse.statusCode)
                else {
                    throw URLError(.badServerResponse)
                }

                return try JSONDecoder().decode(UserProfile.self, from: data)
            },
            link: { provider, token in
                let url = URL(string: "http://localhost:8080/users/me/link/\(provider)")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                if let accessToken = KeychainHelper().read("accessToken") {
                    request.addValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
                }

                // Send the provider's token (accessToken/idToken) as body
                let body = ["token": token]
                request.httpBody = try JSONEncoder().encode(body)

                let (data, response) = try await URLSession.shared.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse,
                    (200...299).contains(httpResponse.statusCode)
                else {
                    throw URLError(.badServerResponse)
                }

                return try JSONDecoder().decode(UserProfile.self, from: data)
            },
            unlink: { provider in
                let url = URL(string: "http://localhost:8080/users/me/link/\(provider)")!
                var request = URLRequest(url: url)
                request.httpMethod = "DELETE"

                if let accessToken = KeychainHelper().read("accessToken") {
                    request.addValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
                }

                let (data, response) = try await URLSession.shared.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse,
                    (200...299).contains(httpResponse.statusCode)
                else {
                    throw URLError(.badServerResponse)
                }

                return try JSONDecoder().decode(UserProfile.self, from: data)
            },
            changePassword: { current, new in
                let url = URL(string: "http://localhost:8080/users/me/password")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                if let accessToken = KeychainHelper().read("accessToken") {
                    request.addValue("Bearer \(accessToken)", forHTTPHeaderField: "Authorization")
                }

                let body = ["currentPassword": current, "newPassword": new]
                request.httpBody = try JSONEncoder().encode(body)

                let (_, response) = try await URLSession.shared.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse,
                    (200...299).contains(httpResponse.statusCode)
                else {
                    throw URLError(.badServerResponse)
                }
            }
        )
    }
}

// Duplicate KeychainHelper for now to avoid public exposure or creating a separate module just for this.
// In a real app, this would be in a Core/Utils module.
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

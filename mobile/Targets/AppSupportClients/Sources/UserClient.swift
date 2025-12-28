import AppSettingsClient
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
    public let isPremium: Bool
    // Fields not returned by /users/me, but used in UI.
    // We make them optional and mutable to potentialy fill them later or locally.
    public var linkedProviders: [String] = []
    public var email: String?

    private enum CodingKeys: String, CodingKey {
        case id, userId, username, displayName, avatarUrl, hasCustomAvatar, bio, visibility,
            preferences, isPremium
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
        isPremium = try container.decode(Bool.self, forKey: .isPremium)

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
        isPremium: Bool = false,
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
        self.isPremium = isPremium
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
    public var generateRandomAvatar: @Sendable () async throws -> UserProfile
    public var becomePremium: @Sendable () async throws -> UserProfile
    public var uploadAvatar: @Sendable (Data) async throws -> UserProfile
}

public enum UserClientError: Error, Equatable {
    case serverError(statusCode: Int)
    case networkError(String)
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
                isPremium: false,
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
                isPremium: true,
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
                isPremium: false,
                linkedProviders: [],
                email: "preview@example.com"
            )
        },
        changePassword: { _, _ in },
        generateRandomAvatar: {
            UserProfile(
                id: "mock-id",
                userId: "mock-user-id",
                username: "Preview User",
                displayName: "Preview Display Name",
                avatarUrl: "https://api.dicebear.com/7.x/avataaars/svg?seed=new",
                hasCustomAvatar: false,
                bio: "Music lover",
                visibility: "public",
                preferences: UserPreferences(genres: ["Pop", "Rock"]),
                isPremium: false,
                linkedProviders: ["google"],
                email: "preview@example.com"
            )
        },
        becomePremium: {
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
                isPremium: true,
                linkedProviders: ["google"],
                email: "preview@example.com"
            )
        },
        uploadAvatar: { _ in
            UserProfile(
                id: "mock-id",
                userId: "mock-user-id",
                username: "Preview User",
                displayName: "Preview Display Name",
                avatarUrl: "https://api.dicebear.com/7.x/avataaars/svg?seed=uploaded",
                hasCustomAvatar: true,
                bio: "Music lover",
                visibility: "public",
                preferences: UserPreferences(genres: ["Pop", "Rock"]),
                isPremium: false,
                linkedProviders: ["google"],
                email: "preview@example.com"
            )
        }
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
                isPremium: false,
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
                isPremium: true,
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
                isPremium: false,
                linkedProviders: [],
                email: "test@example.com"
            )
        },
        changePassword: { _, _ in },
        generateRandomAvatar: {
            UserProfile(
                id: "test-id",
                userId: "test-user-id",
                username: "Test User",
                displayName: "Test Display Name",
                avatarUrl: "https://api.dicebear.com/7.x/avataaars/svg?seed=new",
                hasCustomAvatar: false,
                bio: nil,
                visibility: "public",
                preferences: UserPreferences(),
                isPremium: false,
                linkedProviders: ["google"],
                email: "test@example.com"
            )
        },
        becomePremium: {
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
                isPremium: true,
                linkedProviders: ["google"],
                email: "test@example.com"
            )
        },
        uploadAvatar: { _ in
            UserProfile(
                id: "test-id",
                userId: "test-user-id",
                username: "Test User",
                displayName: "Test Display Name",
                avatarUrl: "https://api.dicebear.com/7.x/avataaars/svg?seed=uploaded",
                hasCustomAvatar: true,
                bio: nil,
                visibility: "public",
                preferences: UserPreferences(),
                isPremium: false,
                linkedProviders: ["google"],
                email: "test@example.com"
            )
        }
    )
}

extension DependencyValues {
    public var user: UserClient {
        get { self[UserClient.self] }
        set { self[UserClient.self] = newValue }
    }
}

extension UserClient {
    static func live(urlSession: URLSession = .shared) -> Self {
        @Dependency(\.appSettings) var appSettings
        @Dependency(\.authentication) var authentication
        let executor = AuthenticatedRequestExecutor(urlSession: urlSession, authentication: authentication)
        let profileCache = UserProfileCache(ttl: 5)

        @Sendable func logError(
            _ request: URLRequest, _ response: HTTPURLResponse?, _ data: Data?, _ error: Error?
        ) {
            print("\n❌ [UserClient] Request Failed")
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
                    // Try to decode error response if possible, or just log
                    let error = URLError(.badServerResponse)
                    logError(request, httpResponse, data, error)

                    // Specific case for server error to match existing logic if needed?
                    if httpResponse.statusCode == 500 {
                        throw UserClientError.serverError(statusCode: 500)
                    }
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
                if let err = error as? UserClientError { throw err }
                logError(request, nil, nil, error)
                throw error
            }
        }

        @Sendable func performRequestNoReturn(_ request: URLRequest) async throws {
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
            me: {
                let token = authentication.getAccessToken()
                return try await profileCache.get(token: token) {
                    let baseUrl = baseURLString()
                    // 1. Fetch User Profile
                    let urlProfile = URL(string: "\(baseUrl)/users/me")!
                    var reqProfile = URLRequest(url: urlProfile)
                    reqProfile.httpMethod = "GET"

                    var profile: UserProfile = try await performRequest(reqProfile)

                    // 2. Fetch Auth Info (Linked Providers)
                    let urlAuth = URL(string: "\(baseUrl)/auth/me")!
                    var reqAuth = URLRequest(url: urlAuth)
                    reqAuth.httpMethod = "GET"

                    struct AuthMeResponse: Decodable {
                        let linkedProviders: [String]?
                        let email: String?
                    }

                    do {
                        // We use performRequest but catch error locally to not fail the whole load
                        let authInfo: AuthMeResponse = try await performRequest(reqAuth)
                        profile.linkedProviders = authInfo.linkedProviders ?? []
                        profile.email = authInfo.email
                    } catch {
                        print("UserClient: Failed to fetch auth/me: \(error)")
                        // Keep default empty linkedProviders
                    }

                    if let avatarUrl = profile.avatarUrl, !avatarUrl.hasPrefix("http") {
                        profile = UserProfile(
                            id: profile.id,
                            userId: profile.userId,
                            username: profile.username,
                            displayName: profile.displayName,
                            avatarUrl: baseUrl + avatarUrl,
                            hasCustomAvatar: profile.hasCustomAvatar,
                            bio: profile.bio,
                            visibility: profile.visibility,
                            preferences: profile.preferences,
                            isPremium: profile.isPremium,
                            linkedProviders: profile.linkedProviders,
                            email: profile.email
                        )
                    }

                    return profile
                }
            },
            updateProfile: { profile in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/me")!
                var request = URLRequest(url: url)
                request.httpMethod = "PATCH"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                struct UpdateRequest: Encodable {
                    let displayName: String
                    let bio: String
                    let visibility: String
                    let preferences: UserPreferences
                }

                let updateReq = UpdateRequest(
                    displayName: profile.displayName,
                    bio: profile.bio ?? "",
                    visibility: profile.visibility,
                    preferences: profile.preferences
                )

                request.httpBody = try JSONEncoder().encode(updateReq)

                var updatedProfile: UserProfile = try await performRequest(request)

                if let avatarUrl = updatedProfile.avatarUrl, !avatarUrl.hasPrefix("http") {
                    updatedProfile = UserProfile(
                        id: updatedProfile.id,
                        userId: updatedProfile.userId,
                        username: updatedProfile.username,
                        displayName: updatedProfile.displayName,
                        avatarUrl: baseUrl + avatarUrl,
                        hasCustomAvatar: updatedProfile.hasCustomAvatar,
                        bio: updatedProfile.bio,
                        visibility: updatedProfile.visibility,
                        preferences: updatedProfile.preferences,
                        isPremium: updatedProfile.isPremium,
                        linkedProviders: updatedProfile.linkedProviders,
                        email: updatedProfile.email
                    )
                }
                await profileCache.set(updatedProfile, token: authentication.getAccessToken())
                return updatedProfile
            },
            link: { provider, token in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/auth/link/\(provider)")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["token": token]
                request.httpBody = try JSONEncoder().encode(body)

                try await performRequestNoReturn(request)

                // Refetch logic duplicate? No, I can call the implementation of `me` or just fetch manually.
                // Since I cannot call `self.me()` which is not available in closure context yet.
                // I'll replicate the exact logic from `me` above.

                // 1. Fetch User Profile
                let urlProfile = URL(string: "\(baseUrl)/users/me")!
                var reqProfile = URLRequest(url: urlProfile)
                reqProfile.httpMethod = "GET"
                var profile: UserProfile = try await performRequest(reqProfile)

                // 2. Auth Info
                let urlAuth = URL(string: "\(baseUrl)/auth/me")!
                var reqAuth = URLRequest(url: urlAuth)
                reqAuth.httpMethod = "GET"

                struct AuthMeResponse: Decodable {
                    let linkedProviders: [String]?
                    let email: String?
                }
                do {
                    let authInfo: AuthMeResponse = try await performRequest(reqAuth)
                    profile.linkedProviders = authInfo.linkedProviders ?? []
                    profile.email = authInfo.email
                } catch {}

                if let avatarUrl = profile.avatarUrl, !avatarUrl.hasPrefix("http") {
                    profile = UserProfile(
                        id: profile.id,
                        userId: profile.userId,
                        username: profile.username,
                        displayName: profile.displayName,
                        avatarUrl: baseUrl + avatarUrl,
                        hasCustomAvatar: profile.hasCustomAvatar,
                        bio: profile.bio,
                        visibility: profile.visibility,
                        preferences: profile.preferences,
                        isPremium: profile.isPremium,
                        linkedProviders: profile.linkedProviders,
                        email: profile.email
                    )
                }
                await profileCache.set(profile, token: authentication.getAccessToken())
                return profile
            },
            unlink: { provider in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/auth/link/\(provider)")!
                var request = URLRequest(url: url)
                request.httpMethod = "DELETE"

                try await performRequestNoReturn(request)

                // Re-fetch logic
                let urlProfile = URL(string: "\(baseUrl)/users/me")!
                var reqProfile = URLRequest(url: urlProfile)
                reqProfile.httpMethod = "GET"
                var profile: UserProfile = try await performRequest(reqProfile)

                let urlAuth = URL(string: "\(baseUrl)/auth/me")!
                var reqAuth = URLRequest(url: urlAuth)
                reqAuth.httpMethod = "GET"
                struct AuthMeResponse: Decodable {
                    let linkedProviders: [String]?
                    let email: String?
                }
                do {
                    let authInfo: AuthMeResponse = try await performRequest(reqAuth)
                    profile.linkedProviders = authInfo.linkedProviders ?? []
                    profile.email = authInfo.email
                } catch {}

                if let avatarUrl = profile.avatarUrl, !avatarUrl.hasPrefix("http") {
                    profile = UserProfile(
                        id: profile.id,
                        userId: profile.userId,
                        username: profile.username,
                        displayName: profile.displayName,
                        avatarUrl: baseUrl + avatarUrl,
                        hasCustomAvatar: profile.hasCustomAvatar,
                        bio: profile.bio,
                        visibility: profile.visibility,
                        preferences: profile.preferences,
                        isPremium: profile.isPremium,
                        linkedProviders: profile.linkedProviders,
                        email: profile.email
                    )
                }
                await profileCache.set(profile, token: authentication.getAccessToken())
                return profile
            },
            changePassword: { current, new in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/me/password")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["currentPassword": current, "newPassword": new]
                request.httpBody = try JSONEncoder().encode(body)

                try await performRequestNoReturn(request)
            },
            generateRandomAvatar: {
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/me/avatar/random")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"

                // This endpoint returns UserProfile.
                var updatedProfile: UserProfile = try await performRequest(request)

                if let avatarUrl = updatedProfile.avatarUrl, !avatarUrl.hasPrefix("http") {
                    updatedProfile = UserProfile(
                        id: updatedProfile.id,
                        userId: updatedProfile.userId,
                        username: updatedProfile.username,
                        displayName: updatedProfile.displayName,
                        avatarUrl: baseUrl + avatarUrl,
                        hasCustomAvatar: updatedProfile.hasCustomAvatar,
                        bio: updatedProfile.bio,
                        visibility: updatedProfile.visibility,
                        preferences: updatedProfile.preferences,
                        isPremium: updatedProfile.isPremium,
                        linkedProviders: updatedProfile.linkedProviders,
                        email: updatedProfile.email
                    )
                }
                await profileCache.set(updatedProfile, token: authentication.getAccessToken())
                return updatedProfile
            },
            becomePremium: {
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/me/premium")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"

                var updatedProfile: UserProfile = try await performRequest(request)

                if let avatarUrl = updatedProfile.avatarUrl, !avatarUrl.hasPrefix("http") {
                    updatedProfile = UserProfile(
                        id: updatedProfile.id,
                        userId: updatedProfile.userId,
                        username: updatedProfile.username,
                        displayName: updatedProfile.displayName,
                        avatarUrl: baseUrl + avatarUrl,
                        hasCustomAvatar: updatedProfile.hasCustomAvatar,
                        bio: updatedProfile.bio,
                        visibility: updatedProfile.visibility,
                        preferences: updatedProfile.preferences,
                        isPremium: updatedProfile.isPremium,
                        linkedProviders: updatedProfile.linkedProviders,
                        email: updatedProfile.email
                    )
                }
                await profileCache.set(updatedProfile, token: authentication.getAccessToken())
                return updatedProfile
            },
            uploadAvatar: { imageData in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/users/me/avatar/upload")!
                let boundary = "Boundary-\(UUID().uuidString)"
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.setValue(
                    "multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")

                var body = Data()
                body.append("--\(boundary)\r\n".data(using: .utf8)!)
                body.append(
                    "Content-Disposition: form-data; name=\"avatar\"; filename=\"avatar.jpg\"\r\n"
                        .data(using: .utf8)!)
                body.append("Content-Type: image/jpeg\r\n\r\n".data(using: .utf8)!)
                body.append(imageData)
                body.append("\r\n--\(boundary)--\r\n".data(using: .utf8)!)
                request.httpBody = body

                var updatedProfile: UserProfile = try await performRequest(request)

                if let avatarUrl = updatedProfile.avatarUrl, !avatarUrl.hasPrefix("http") {
                    updatedProfile = UserProfile(
                        id: updatedProfile.id,
                        userId: updatedProfile.userId,
                        username: updatedProfile.username,
                        displayName: updatedProfile.displayName,
                        avatarUrl: baseUrl + avatarUrl,
                        hasCustomAvatar: updatedProfile.hasCustomAvatar,
                        bio: updatedProfile.bio,
                        visibility: updatedProfile.visibility,
                        preferences: updatedProfile.preferences,
                        isPremium: updatedProfile.isPremium,
                        linkedProviders: updatedProfile.linkedProviders,
                        email: updatedProfile.email
                    )
                }
                await profileCache.set(updatedProfile, token: authentication.getAccessToken())
                return updatedProfile
            }
        )
    }
}

private actor UserProfileCache {
    private struct Entry {
        let profile: UserProfile
        let token: String?
        let fetchedAt: Date
    }

    private let ttl: TimeInterval
    private var entry: Entry?
    private var inFlight: Task<UserProfile, Error>?
    private var inFlightToken: String?

    init(ttl: TimeInterval) {
        self.ttl = ttl
    }

    func get(
        token: String?,
        fetch: @escaping @Sendable () async throws -> UserProfile
    ) async throws -> UserProfile {
        let now = Date()
        if let entry, entry.token == token, now.timeIntervalSince(entry.fetchedAt) < ttl {
            return entry.profile
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
        case .success(let profile):
            entry = Entry(profile: profile, token: token, fetchedAt: Date())
            return profile
        case .failure(let error):
            throw error
        }
    }

    func set(_ profile: UserProfile, token: String?) {
        entry = Entry(profile: profile, token: token, fetchedAt: Date())
        inFlight = nil
        inFlightToken = nil
    }
}

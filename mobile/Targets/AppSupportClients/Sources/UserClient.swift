import Dependencies
import Foundation

public struct UserProfile: Decodable, Equatable, Sendable {
    public let id: String
    public let userId: String
    public let username: String
    public let displayName: String
    public let avatarUrl: String
    public let hasCustomAvatar: Bool
}

public struct UserClient: Sendable {
    public var me: @Sendable () async throws -> UserProfile
}

extension UserClient: DependencyKey {
    public static let liveValue = UserClient.live()

    public static let previewValue = UserClient {
        UserProfile(
            id: "mock-id",
            userId: "mock-user-id",
            username: "Preview User",
            displayName: "Preview Display Name",
            avatarUrl: "",
            hasCustomAvatar: false
        )
    }

    public static let testValue = UserClient {
        UserProfile(
            id: "test-id",
            userId: "test-user-id",
            username: "Test User",
            displayName: "Test Display Name",
            avatarUrl: "",
            hasCustomAvatar: false
        )
    }
}

extension DependencyValues {
    public var user: UserClient {
        get { self[UserClient.self] }
        set { self[UserClient.self] = newValue }
    }
}

extension UserClient {
    static func live() -> Self {
        // TODO: Inject AuthenticationClient to get the token, or read from Keychain directly?
        // Ideally, we should use an interceptor or inject the token.
        // For simplicity here, we'll read from Keychain again (duplication, but decouples for now).

        return Self(
            me: {
                // TODO: Use configured base URL
                let url = URL(string: "http://localhost:8080/users/me")!
                var request = URLRequest(url: url)
                request.httpMethod = "GET"

                if let token = KeychainHelper().read("accessToken") {
                    request.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
                }

                let (data, response) = try await URLSession.shared.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse,
                    (200...299).contains(httpResponse.statusCode)
                else {
                    throw URLError(.badServerResponse)
                }

                return try JSONDecoder().decode(UserProfile.self, from: data)
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

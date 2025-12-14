import Dependencies
import Foundation
import Security

public struct AuthenticationClient: Sendable {
    public var login: @Sendable (_ email: String, _ password: String) async throws -> Void
    public var register: @Sendable (_ email: String, _ password: String) async throws -> Void
    public var logout: @Sendable () async -> Void
    public var isAuthenticated: @Sendable () -> Bool
    public var getAccessToken: @Sendable () -> String?
    public var saveTokens: @Sendable (_ accessToken: String, _ refreshToken: String) async -> Void
}

public enum AuthenticationError: Error, Equatable {
    case invalidCredentials
    case networkError(String)
    case unknown
}

// MARK: - Dependency Key

extension AuthenticationClient: DependencyKey {
    public static let liveValue = AuthenticationClient.live()

    public static let previewValue = AuthenticationClient(
        login: { _, _ in },
        register: { _, _ in },
        logout: {},
        isAuthenticated: { true },
        getAccessToken: { "mock_token" },
        saveTokens: { _, _ in }
    )

    public static let testValue = AuthenticationClient(
        login: { _, _ in },
        register: { _, _ in },
        logout: {},
        isAuthenticated: { false },
        getAccessToken: { nil },
        saveTokens: { _, _ in }
    )
}

extension DependencyValues {
    public var authentication: AuthenticationClient {
        get { self[AuthenticationClient.self] }
        set { self[AuthenticationClient.self] = newValue }
    }
}

// MARK: - Live Implementation

extension AuthenticationClient {
    static func live() -> Self {
        let keychain = KeychainHelper()

        // TODO: Use a proper configured base URL
        let baseURL = URL(string: "http://localhost:8080/auth")!

        return Self(
            login: { email, password in
                let url = baseURL.appendingPathComponent("login")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["email": email, "password": password]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)

                let (data, response) = try await URLSession.shared.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse else {
                    throw AuthenticationError.networkError("Invalid response")
                }

                if httpResponse.statusCode == 401 {
                    throw AuthenticationError.invalidCredentials
                }

                guard (200...299).contains(httpResponse.statusCode) else {
                    throw AuthenticationError.networkError(
                        "Server error: \(httpResponse.statusCode)")
                }

                struct AuthResponse: Decodable {
                    let accessToken: String
                    let refreshToken: String
                }

                let authResponse = try JSONDecoder().decode(AuthResponse.self, from: data)

                keychain.save(authResponse.accessToken, for: "accessToken")
                keychain.save(authResponse.refreshToken, for: "refreshToken")
            },
            register: { email, password in
                let url = baseURL.appendingPathComponent("register")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["email": email, "password": password]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)

                let (data, response) = try await URLSession.shared.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse,
                    (200...299).contains(httpResponse.statusCode)
                else {
                    throw AuthenticationError.networkError("Registration failed")
                }

                struct AuthResponse: Decodable {
                    let accessToken: String
                    let refreshToken: String
                }

                let authResponse = try JSONDecoder().decode(AuthResponse.self, from: data)

                keychain.save(authResponse.accessToken, for: "accessToken")
                keychain.save(authResponse.refreshToken, for: "refreshToken")
            },
            logout: {
                keychain.delete("accessToken")
                keychain.delete("refreshToken")
            },
            isAuthenticated: {
                return keychain.read("accessToken") != nil
            },
            getAccessToken: {
                return keychain.read("accessToken")
            },
            saveTokens: { accessToken, refreshToken in
                keychain.save(accessToken, for: "accessToken")
                keychain.save(refreshToken, for: "refreshToken")
            }
        )
    }
}

// MARK: - Keychain Helper

private struct KeychainHelper {
    func save(_ data: String, for key: String) {
        let data = Data(data.utf8)
        let query =
            [
                kSecClass: kSecClassGenericPassword,
                kSecAttrAccount: key,
                kSecValueData: data,
            ] as [String: Any]

        SecItemDelete(query as CFDictionary)
        SecItemAdd(query as CFDictionary, nil)
    }

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

    func delete(_ key: String) {
        let query =
            [
                kSecClass: kSecClassGenericPassword,
                kSecAttrAccount: key,
            ] as [String: Any]

        SecItemDelete(query as CFDictionary)
    }
}

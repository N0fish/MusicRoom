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
    public var refreshToken: @Sendable () async throws -> Void
    public var forgotPassword: @Sendable (_ email: String) async throws -> Void
}

public enum AuthenticationError: Error, Equatable {
    case invalidCredentials
    case userAlreadyExists
    case badRequest(String)
    case serverError(String)
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
        saveTokens: { _, _ in },
        refreshToken: {},
        forgotPassword: { _ in }
    )

    public static let testValue = AuthenticationClient(
        login: { _, _ in },
        register: { _, _ in },
        logout: {},
        isAuthenticated: { false },
        getAccessToken: { nil },
        saveTokens: { _, _ in },
        refreshToken: {},
        forgotPassword: { _ in }
    )
}

extension DependencyValues {
    public var authentication: AuthenticationClient {
        get { self[AuthenticationClient.self] }
        set { self[AuthenticationClient.self] = newValue }
    }
}

extension AuthenticationClient {
    public struct SocialHelper {
        public enum SocialProvider: String, Sendable {
            case google
            case intra42 = "42"
        }

        public static func authURL(for provider: SocialProvider, baseURL: URL) -> URL {
            return
                baseURL
                .appendingPathComponent("auth")
                .appendingPathComponent(provider.rawValue)
                .appendingPathComponent("login")
        }

        public static func parseCallback(url: URL) -> (accessToken: String, refreshToken: String)? {
            let components = URLComponents(url: url, resolvingAgainstBaseURL: true)
            let items = components?.queryItems ?? []

            guard let accessToken = items.first(where: { $0.name == "accessToken" })?.value,
                let refreshToken = items.first(where: { $0.name == "refreshToken" })?.value
            else {
                return nil
            }
            return (accessToken, refreshToken)
        }
    }
}

// MARK: - Live Implementation

extension AuthenticationClient {
    static func live(urlSession: URLSession = .shared) -> Self {
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

                let (data, response) = try await urlSession.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse else {
                    throw AuthenticationError.networkError("Invalid response")
                }

                if httpResponse.statusCode == 401 {
                    throw AuthenticationError.invalidCredentials
                }

                if httpResponse.statusCode == 500 {
                    throw AuthenticationError.serverError("Internal Server Error")
                }

                guard (200...299).contains(httpResponse.statusCode) else {
                    if httpResponse.statusCode == 400 {
                        struct ErrorResponse: Decodable { let error: String }
                        if let errResp = try? JSONDecoder().decode(ErrorResponse.self, from: data) {
                            throw AuthenticationError.badRequest(errResp.error)
                        }
                    }
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

                let (data, response) = try await urlSession.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse else {
                    throw AuthenticationError.networkError("Registration failed")
                }

                if httpResponse.statusCode == 409 {
                    throw AuthenticationError.userAlreadyExists
                }

                if httpResponse.statusCode == 500 {
                    throw AuthenticationError.serverError("Internal Server Error")
                }

                guard (200...299).contains(httpResponse.statusCode) else {
                    if httpResponse.statusCode == 400 {
                        struct ErrorResponse: Decodable { let error: String }
                        if let errResp = try? JSONDecoder().decode(ErrorResponse.self, from: data) {
                            throw AuthenticationError.badRequest(errResp.error)
                        }
                    }
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
            },
            refreshToken: {
                guard let currentRefreshToken = keychain.read("refreshToken") else {
                    throw AuthenticationError.invalidCredentials
                }

                let url = baseURL.appendingPathComponent("refresh")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["refreshToken": currentRefreshToken]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)

                let (data, response) = try await urlSession.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse else {
                    throw AuthenticationError.networkError("Invalid response")
                }

                if httpResponse.statusCode == 401 || httpResponse.statusCode == 403 {
                    // Refresh token invalid/expired
                    keychain.delete("accessToken")
                    keychain.delete("refreshToken")
                    throw AuthenticationError.invalidCredentials
                }

                guard (200...299).contains(httpResponse.statusCode) else {
                    throw AuthenticationError.networkError(
                        "Server error: \(httpResponse.statusCode)")
                }

                struct RefreshResponse: Decodable {
                    let accessToken: String
                    let refreshToken: String
                }

                let refreshResponse = try JSONDecoder().decode(RefreshResponse.self, from: data)

                keychain.save(refreshResponse.accessToken, for: "accessToken")
                keychain.save(refreshResponse.refreshToken, for: "refreshToken")
            },
            forgotPassword: { email in
                let url = baseURL.appendingPathComponent("forgot-password")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["email": email]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)

                let (_, response) = try await urlSession.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse,
                    (200...299).contains(httpResponse.statusCode)
                else {
                    throw AuthenticationError.networkError("Forgot password request failed")
                }
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

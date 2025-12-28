import AppSettingsClient
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
            let url =
                baseURL
                .appendingPathComponent("auth")
                .appendingPathComponent(provider.rawValue)
                .appendingPathComponent("login")

            guard var components = URLComponents(url: url, resolvingAgainstBaseURL: true) else {
                return url
            }

            components.queryItems = [
                URLQueryItem(name: "redirect", value: "musicroom://auth/callback")
            ]

            return components.url ?? url
        }

        public static func parseCallback(url: URL) -> (accessToken: String, refreshToken: String)? {
            let components = URLComponents(url: url, resolvingAgainstBaseURL: true)

            // 1. Try query items first
            var items = components?.queryItems ?? []

            // 2. If empty, try parsing fragment as query string
            if items.isEmpty, let fragment = components?.fragment {
                let dummyURL = URL(string: "http://dummy.com?\(fragment)")!
                if let fragmentComponents = URLComponents(
                    url: dummyURL, resolvingAgainstBaseURL: true)
                {
                    items = fragmentComponents.queryItems ?? []
                }
            }

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
    static func live(
        urlSession: URLSession = .shared,
        keychain: KeychainStoring = KeychainHelper()
    ) -> Self {
        let keychain = keychain
        let refreshActor = RefreshActor()
        let authGeneration = AuthGeneration()
        @Dependency(\.appSettings) var appSettings

        @Sendable func logError(
            _ request: URLRequest, _ response: HTTPURLResponse?, _ data: Data?, _ error: Error?
        ) {
            print("\n❌ [AuthenticationClient] Request Failed")
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

        return Self(
            login: { email, password in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/auth/login")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["email": email, "password": password]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)

                do {
                    let (data, response) = try await urlSession.data(for: request)

                    guard let httpResponse = response as? HTTPURLResponse else {
                        throw AuthenticationError.networkError("Invalid response")
                    }

                    if httpResponse.statusCode == 401 {
                        logError(
                            request, httpResponse, data, AuthenticationError.invalidCredentials)
                        throw AuthenticationError.invalidCredentials
                    }

                    if httpResponse.statusCode == 500 {
                        let error = AuthenticationError.serverError("Internal Server Error")
                        logError(request, httpResponse, data, error)
                        throw error
                    }

                    guard (200...299).contains(httpResponse.statusCode) else {
                        if httpResponse.statusCode == 400 {
                            struct ErrorResponse: Decodable { let error: String }
                            if let errResp = try? JSONDecoder().decode(
                                ErrorResponse.self, from: data)
                            {
                                let error = AuthenticationError.badRequest(errResp.error)
                                logError(request, httpResponse, data, error)
                                throw error
                            }
                        }
                        let error = AuthenticationError.networkError(
                            "Server error: \(httpResponse.statusCode)")
                        logError(request, httpResponse, data, error)
                        throw error
                    }

                    struct AuthResponse: Decodable {
                        let accessToken: String
                        let refreshToken: String
                    }

                    do {
                        let authResponse = try JSONDecoder().decode(AuthResponse.self, from: data)
                        await authGeneration.bump()
                        await refreshActor.cancel()
                        keychain.save(authResponse.accessToken, for: "accessToken")
                        keychain.save(authResponse.refreshToken, for: "refreshToken")
                    } catch {
                        logError(request, httpResponse, data, error)
                        throw error
                    }
                } catch {
                    // If we haven't already logged it (internal throws), log here if it's a network error caught from URLSession
                    // Simpler: Just log if it's not one of our typed errors, or just rely on the throw points above?
                    // URLSession errors (offline etc) land here.
                    if !(error is AuthenticationError) {
                        logError(request, nil, nil, error)
                    }
                    throw error
                }
            },
            register: { email, password in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/auth/register")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["email": email, "password": password]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)

                do {
                    let (data, response) = try await urlSession.data(for: request)

                    guard let httpResponse = response as? HTTPURLResponse else {
                        throw AuthenticationError.networkError("Registration failed")
                    }

                    if httpResponse.statusCode == 409 {
                        logError(request, httpResponse, data, AuthenticationError.userAlreadyExists)
                        throw AuthenticationError.userAlreadyExists
                    }

                    if httpResponse.statusCode == 500 {
                        let error = AuthenticationError.serverError("Internal Server Error")
                        logError(request, httpResponse, data, error)
                        throw error
                    }

                    guard (200...299).contains(httpResponse.statusCode) else {
                        if httpResponse.statusCode == 400 {
                            struct ErrorResponse: Decodable { let error: String }
                            if let errResp = try? JSONDecoder().decode(
                                ErrorResponse.self, from: data)
                            {
                                let error = AuthenticationError.badRequest(errResp.error)
                                logError(request, httpResponse, data, error)
                                throw error
                            }
                        }
                        let error = AuthenticationError.networkError("Registration failed")
                        logError(request, httpResponse, data, error)
                        throw error
                    }

                    struct AuthResponse: Decodable {
                        let accessToken: String
                        let refreshToken: String
                    }

                    do {
                        let authResponse = try JSONDecoder().decode(AuthResponse.self, from: data)
                        await authGeneration.bump()
                        await refreshActor.cancel()
                        keychain.save(authResponse.accessToken, for: "accessToken")
                        keychain.save(authResponse.refreshToken, for: "refreshToken")
                    } catch {
                        logError(request, httpResponse, data, error)
                        throw error
                    }
                } catch {
                    if !(error is AuthenticationError) {
                        logError(request, nil, nil, error)
                    }
                    throw error
                }
            },
            logout: {
                await authGeneration.bump()
                await refreshActor.cancel()
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
                await authGeneration.bump()
                await refreshActor.cancel()
                keychain.save(accessToken, for: "accessToken")
                keychain.save(refreshToken, for: "refreshToken")
            },
            refreshToken: {
                let generationAtStart = await authGeneration.current()
                do {
                    try await refreshActor.refresh {
                        guard let currentRefreshToken = keychain.read("refreshToken") else {
                            throw AuthenticationError.invalidCredentials
                        }

                        let baseUrl = baseURLString()
                        let url = URL(string: "\(baseUrl)/auth/refresh")!
                        var request = URLRequest(url: url)
                        request.httpMethod = "POST"
                        request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                        let body = ["refreshToken": currentRefreshToken]
                        request.httpBody = try JSONSerialization.data(withJSONObject: body)

                        do {
                            let (data, response) = try await urlSession.data(for: request)

                            guard let httpResponse = response as? HTTPURLResponse else {
                                throw AuthenticationError.networkError("Invalid response")
                            }

                            if httpResponse.statusCode == 401 || httpResponse.statusCode == 403 {
                                // Refresh token invalid/expired
                                keychain.delete("accessToken")
                                keychain.delete("refreshToken")
                                logError(
                                    request, httpResponse, data,
                                    AuthenticationError.invalidCredentials)
                                throw AuthenticationError.invalidCredentials
                            }

                            guard (200...299).contains(httpResponse.statusCode) else {
                                let error = AuthenticationError.networkError(
                                    "Server error: \(httpResponse.statusCode)")
                                logError(request, httpResponse, data, error)
                                throw error
                            }

                            struct RefreshResponse: Decodable {
                                let accessToken: String
                                let refreshToken: String
                            }

                            do {
                                let refreshResponse = try JSONDecoder().decode(
                                    RefreshResponse.self, from: data)
                                if await authGeneration.current() != generationAtStart {
                                    return
                                }
                                keychain.save(refreshResponse.accessToken, for: "accessToken")
                                keychain.save(refreshResponse.refreshToken, for: "refreshToken")
                            } catch {
                                logError(request, httpResponse, data, error)
                                throw error
                            }
                        } catch {
                            if !(error is AuthenticationError) && !(error is CancellationError) {
                                logError(request, nil, nil, error)
                            }
                            throw error
                        }
                    }
                } catch is CancellationError {
                    return
                }
            },
            forgotPassword: { email in
                let baseUrl = baseURLString()
                let url = URL(string: "\(baseUrl)/auth/forgot-password")!
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["email": email]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)

                do {
                    let (data, response) = try await urlSession.data(for: request)

                    guard let httpResponse = response as? HTTPURLResponse,
                        (200...299).contains(httpResponse.statusCode)
                    else {
                        let error = AuthenticationError.networkError(
                            "Forgot password request failed")
                        logError(request, response as? HTTPURLResponse, data, error)
                        throw error
                    }
                } catch {
                    if !(error is AuthenticationError) {
                        logError(request, nil, nil, error)
                    }
                    throw error
                }
            }
        )
    }
}

// MARK: - Keychain Helper

protocol KeychainStoring: Sendable {
    func save(_ value: String, for key: String)
    func read(_ key: String) -> String?
    func delete(_ key: String)
}

private struct KeychainHelper: KeychainStoring {
    func save(_ value: String, for key: String) {
        let data = value.data(using: .utf8)!
        let query =
            [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrAccount as String: key,
            ] as [String: Any]

        let attributesToUpdate =
            [
                kSecValueData as String: data
            ] as [String: Any]

        // Try to update first
        let status = SecItemUpdate(query as CFDictionary, attributesToUpdate as CFDictionary)

        if status == errSecItemNotFound {
            // Item doesn't exist, so add it
            var newItem = query
            newItem[kSecValueData as String] = data
            // Optional: Set accessibility attribute if needed (e.g. kSecAttrAccessibleAfterFirstUnlock)
            // newItem[kSecAttrAccessible] = kSecAttrAccessibleAfterFirstUnlock

            let addStatus = SecItemAdd(newItem as CFDictionary, nil)
            if addStatus != errSecSuccess {
                print("Keychain add error for key \(key): \(addStatus)")
            }
        } else if status != errSecSuccess {
            print("Keychain update error for key \(key): \(status)")
        }
    }

    func read(_ key: String) -> String? {
        let query =
            [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrAccount as String: key,
                kSecReturnData as String: true,
                kSecMatchLimit as String: kSecMatchLimitOne,
            ] as [String: Any]

        var dataTypeRef: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &dataTypeRef)

        if status == errSecSuccess, let data = dataTypeRef as? Data {
            return String(data: data, encoding: .utf8)
        } else if status != errSecItemNotFound {
            print("Keychain read error for key \(key): \(status)")
        }
        return nil
    }

    func delete(_ key: String) {
        let query =
            [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrAccount as String: key,
            ] as [String: Any]

        let status = SecItemDelete(query as CFDictionary)
        if status != errSecSuccess && status != errSecItemNotFound {
            print("Keychain delete error for key \(key): \(status)")
        }
    }
}

private actor RefreshActor {
    private var refreshTask: Task<Void, Error>?

    func refresh(operation: @escaping @Sendable () async throws -> Void) async throws {
        if let task = refreshTask {
            return try await task.value
        }
        let task = Task {
            try await operation()
        }
        refreshTask = task
        let result = await task.result
        refreshTask = nil
        try result.get()
    }

    func cancel() {
        refreshTask?.cancel()
        refreshTask = nil
    }
}

private actor AuthGeneration {
    private var value: Int = 0

    @discardableResult
    func bump() -> Int {
        value &+= 1
        return value
    }

    func current() -> Int {
        value
    }
}

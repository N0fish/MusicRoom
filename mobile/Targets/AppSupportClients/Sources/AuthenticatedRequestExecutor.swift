import Foundation

public struct AuthenticatedRequestExecutor: Sendable {
    private let authentication: AuthenticationClient
    private let urlSession: URLSession
    private let maxRetryCount: Int
    private let sessionEvents: SessionEventsClient

    public init(
        urlSession: URLSession = .shared,
        authentication: AuthenticationClient,
        sessionEvents: SessionEventsClient,
        maxRetryCount: Int = 1
    ) {
        self.urlSession = urlSession
        self.authentication = authentication
        self.sessionEvents = sessionEvents
        self.maxRetryCount = maxRetryCount
    }

    public func data(for request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        try await data(for: request, retryCount: 0)
    }

    private func data(for request: URLRequest, retryCount: Int) async throws -> (Data, HTTPURLResponse) {
        var request = request
        let accessToken = authentication.getAccessToken()
        let hadAccessToken = accessToken != nil
        if let token = accessToken {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        let (payload, response) = try await urlSession.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw URLError(.badServerResponse)
        }

        if httpResponse.statusCode == 401 {
            if retryCount < maxRetryCount {
                do {
                    try await authentication.refreshToken()
                } catch let error as AuthenticationError {
                    if error == .invalidCredentials {
                        await handleSessionExpired(shouldNotify: hadAccessToken)
                    }
                    throw error
                }
                return try await data(for: request, retryCount: retryCount + 1)
            } else {
                await handleSessionExpired(shouldNotify: hadAccessToken)
                throw AuthenticationError.invalidCredentials
            }
        }

        return (payload, httpResponse)
    }

    private func handleSessionExpired(shouldNotify: Bool) async {
        await authentication.logout()
        guard shouldNotify else { return }
        await sessionEvents.send(.expired)
    }
}

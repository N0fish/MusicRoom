import Foundation

public struct AuthenticatedRequestExecutor: Sendable {
    private let authentication: AuthenticationClient
    private let urlSession: URLSession
    private let maxRetryCount: Int

    public init(
        urlSession: URLSession = .shared,
        authentication: AuthenticationClient,
        maxRetryCount: Int = 1
    ) {
        self.urlSession = urlSession
        self.authentication = authentication
        self.maxRetryCount = maxRetryCount
    }

    public func data(for request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        try await data(for: request, retryCount: 0)
    }

    private func data(for request: URLRequest, retryCount: Int) async throws -> (Data, HTTPURLResponse) {
        var request = request
        if let token = authentication.getAccessToken() {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        let (payload, response) = try await urlSession.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse else {
            throw URLError(.badServerResponse)
        }

        if httpResponse.statusCode == 401, retryCount < maxRetryCount {
            try await authentication.refreshToken()
            return try await data(for: request, retryCount: retryCount + 1)
        }

        return (payload, httpResponse)
    }
}

import AuthenticationServices
import Dependencies
import Foundation

public struct WebAuthenticationSessionClient: Sendable {
    public var authenticate: @Sendable (_ url: URL, _ callbackURLScheme: String) async throws -> URL
}

extension WebAuthenticationSessionClient: DependencyKey {
    public static let liveValue = WebAuthenticationSessionClient {
        (url: URL, callbackURLScheme: String) async throws -> URL in
        let task = Task { @MainActor in
            return try await withCheckedThrowingContinuation {
                (continuation: CheckedContinuation<URL, Error>) in
                let session = ASWebAuthenticationSession(
                    url: url,
                    callbackURLScheme: callbackURLScheme
                ) { callbackURL, error in
                    if let error {
                        continuation.resume(throwing: error)
                    } else if let callbackURL {
                        continuation.resume(returning: callbackURL)
                    } else {
                        continuation.resume(throwing: URLError(.unknown))
                    }
                }

                // Context provider is required on iOS
                session.presentationContextProvider = WebAuthenticationSessionContextProvider.shared
                session.prefersEphemeralWebBrowserSession = false

                session.start()
            }
        }
        return try await task.value
    }

    public static let testValue = WebAuthenticationSessionClient { url, _ in
        return URL(
            string: "musicroom://auth/callback?accessToken=mockAccess&refreshToken=mockRefresh")!
    }
}

// Singleton for Presentation Context
private class WebAuthenticationSessionContextProvider: NSObject,
    ASWebAuthenticationPresentationContextProviding
{
    @MainActor
    static let shared = WebAuthenticationSessionContextProvider()

    @MainActor
    func presentationAnchor(for session: ASWebAuthenticationSession) -> ASPresentationAnchor {
        // Find the active window scene
        if let scene = UIApplication.shared.connectedScenes.first(where: {
            $0.activationState == .foregroundActive
        }) as? UIWindowScene,
            let window = scene.windows.first(where: { $0.isKeyWindow })
        {
            return window
        }
        fatalError("No active window scene found for authentication session presentation.")
    }
}

extension DependencyValues {
    public var webAuthenticationSession: WebAuthenticationSessionClient {
        get { self[WebAuthenticationSessionClient.self] }
        set { self[WebAuthenticationSessionClient.self] = newValue }
    }
}

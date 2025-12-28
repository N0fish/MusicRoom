import Dependencies
import Foundation
import XCTest

@testable import AppSettingsClient
@testable import AppSupportClients

final class UserClientTests: XCTestCase {
    override func tearDown() {
        URLProtocol.unregisterClass(UserMockURLProtocol.self)
        UserMockURLProtocol.requestHandler = nil
        super.tearDown()
    }

    func testMeDeduplicatesConcurrentRequests() async throws {
        let counter = PathCounter()

        UserMockURLProtocol.requestHandler = { request in
            guard let url = request.url else { fatalError("Missing URL") }
            counter.increment(url.path)

            let response = HTTPURLResponse(
                url: url,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!

            switch url.path {
            case "/users/me":
                let data = """
                    {
                        "id": "profile-1",
                        "userId": "user-1",
                        "username": "test",
                        "displayName": "Test User",
                        "avatarUrl": null,
                        "hasCustomAvatar": false,
                        "bio": null,
                        "visibility": "public",
                        "preferences": {"genres": [], "artists": [], "moods": []},
                        "isPremium": false
                    }
                    """.data(using: .utf8)!
                return (response, data)

            case "/auth/me":
                let data = """
                    {"linkedProviders": ["google"], "email": "test@example.com"}
                    """.data(using: .utf8)!
                return (response, data)

            default:
                let errorResponse = HTTPURLResponse(
                    url: url,
                    statusCode: 404,
                    httpVersion: nil,
                    headerFields: nil
                )!
                return (errorResponse, Data())
            }
        }

        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [UserMockURLProtocol.self]
        let session = URLSession(configuration: config)

        let settings = AppSettings(
            selectedPreset: .local,
            localURL: URL(string: "http://localhost:8080")!,
            hostedURL: URL(string: "http://localhost:8080")!
        )

        let client = withDependencies {
            $0.appSettings.load = { settings }
            $0.authentication.getAccessToken = { "token" }
        } operation: {
            UserClient.live(urlSession: session)
        }

        async let p1 = client.me()
        async let p2 = client.me()
        _ = try await (p1, p2)

        XCTAssertEqual(counter.value(for: "/users/me"), 1)
        XCTAssertEqual(counter.value(for: "/auth/me"), 1)
    }

    func testMeUsesCacheWithinTTL() async throws {
        let counter = PathCounter()

        UserMockURLProtocol.requestHandler = { request in
            guard let url = request.url else { fatalError("Missing URL") }
            counter.increment(url.path)

            let response = HTTPURLResponse(
                url: url,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!

            switch url.path {
            case "/users/me":
                let data = """
                    {
                        "id": "profile-1",
                        "userId": "user-1",
                        "username": "test",
                        "displayName": "Test User",
                        "avatarUrl": null,
                        "hasCustomAvatar": false,
                        "bio": null,
                        "visibility": "public",
                        "preferences": {"genres": [], "artists": [], "moods": []},
                        "isPremium": false
                    }
                    """.data(using: .utf8)!
                return (response, data)

            case "/auth/me":
                let data = """
                    {"linkedProviders": ["google"], "email": "test@example.com"}
                    """.data(using: .utf8)!
                return (response, data)

            default:
                let errorResponse = HTTPURLResponse(
                    url: url,
                    statusCode: 404,
                    httpVersion: nil,
                    headerFields: nil
                )!
                return (errorResponse, Data())
            }
        }

        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [UserMockURLProtocol.self]
        let session = URLSession(configuration: config)

        let settings = AppSettings(
            selectedPreset: .local,
            localURL: URL(string: "http://localhost:8080")!,
            hostedURL: URL(string: "http://localhost:8080")!
        )

        let client = withDependencies {
            $0.appSettings.load = { settings }
            $0.authentication.getAccessToken = { "token" }
        } operation: {
            UserClient.live(urlSession: session)
        }

        _ = try await client.me()
        _ = try await client.me()

        XCTAssertEqual(counter.value(for: "/users/me"), 1)
        XCTAssertEqual(counter.value(for: "/auth/me"), 1)
    }
}

final class UserMockURLProtocol: URLProtocol {
    nonisolated(unsafe) static var requestHandler: ((URLRequest) throws -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool {
        true
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest {
        request
    }

    override func startLoading() {
        guard let handler = UserMockURLProtocol.requestHandler else {
            fatalError("Handler is unavailable.")
        }

        do {
            let (response, data) = try handler(request)
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: data)
            client?.urlProtocolDidFinishLoading(self)
        } catch {
            client?.urlProtocol(self, didFailWithError: error)
        }
    }

    override func stopLoading() {}
}

final class PathCounter: @unchecked Sendable {
    private let lock = NSLock()
    private var counts: [String: Int] = [:]

    func increment(_ path: String) {
        lock.lock()
        counts[path, default: 0] += 1
        lock.unlock()
    }

    func value(for path: String) -> Int {
        lock.lock()
        let value = counts[path, default: 0]
        lock.unlock()
        return value
    }
}

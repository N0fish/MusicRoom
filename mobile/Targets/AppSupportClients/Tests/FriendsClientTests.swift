import Dependencies
import Foundation
import XCTest

@testable import AppSettingsClient
@testable import AppSupportClients

final class FriendsClientTests: XCTestCase {
    override func tearDown() {
        URLProtocol.unregisterClass(FriendsMockURLProtocol.self)
        FriendsMockURLProtocol.requestHandler = nil
        super.tearDown()
    }

    func testListFriendsDeduplicatesConcurrentRequests() async throws {
        let counter = FriendsPathCounter()

        FriendsMockURLProtocol.requestHandler = { request in
            guard let url = request.url else { fatalError("Missing URL") }
            counter.increment(url.path)

            let response = HTTPURLResponse(
                url: url,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!

            switch url.path {
            case "/users/me/friends":
                let data = """
                    {"items":[{"userId":"u1","username":"one","displayName":"One","avatarUrl":null,"isPremium":false}]}
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
        config.protocolClasses = [FriendsMockURLProtocol.self]
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
            FriendsClient.live(urlSession: session)
        }

        async let first = client.listFriends()
        async let second = client.listFriends()
        _ = try await (first, second)

        XCTAssertEqual(counter.value(for: "/users/me/friends"), 1)
    }

    func testIncomingRequestsUsesCacheWithinTTL() async throws {
        let counter = FriendsPathCounter()

        FriendsMockURLProtocol.requestHandler = { request in
            guard let url = request.url else { fatalError("Missing URL") }
            counter.increment(url.path)

            let response = HTTPURLResponse(
                url: url,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!

            switch url.path {
            case "/users/me/friends/requests/incoming":
                let data = """
                    {"items":[{"from":{"userId":"u2","username":"two","displayName":"Two","avatarUrl":null,"isPremium":false},"createdAt":"2025-01-01T00:00:00Z"}]}
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
        config.protocolClasses = [FriendsMockURLProtocol.self]
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
            FriendsClient.live(urlSession: session)
        }

        _ = try await client.incomingRequests()
        _ = try await client.incomingRequests()

        XCTAssertEqual(counter.value(for: "/users/me/friends/requests/incoming"), 1)
    }
}

final class FriendsMockURLProtocol: URLProtocol {
    nonisolated(unsafe) static var requestHandler: ((URLRequest) throws -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool {
        true
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest {
        request
    }

    override func startLoading() {
        guard let handler = FriendsMockURLProtocol.requestHandler else {
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

final class FriendsPathCounter: @unchecked Sendable {
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

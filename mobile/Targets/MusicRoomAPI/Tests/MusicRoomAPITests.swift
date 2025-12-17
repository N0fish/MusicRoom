import ComposableArchitecture
import Dependencies
import Foundation
import XCTest

@testable import AppSettingsClient
@testable import AppSupportClients
@testable import MusicRoomAPI
@testable import MusicRoomDomain

final class MockURLProtocol: URLProtocol {
    nonisolated(unsafe) static var requestHandler: ((URLRequest) throws -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool {
        return true
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest {
        return request
    }

    override func startLoading() {
        guard let handler = MockURLProtocol.requestHandler else {
            XCTFail("Received unexpected request with no handler set")
            return
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

    override func stopLoading() {
        // No-op
    }
}

final class MusicRoomAPITests: XCTestCase {

    override func setUp() {
        super.setUp()
        URLProtocol.registerClass(MockURLProtocol.self)
    }

    override func tearDown() {
        URLProtocol.unregisterClass(MockURLProtocol.self)
        MockURLProtocol.requestHandler = nil
        super.tearDown()
    }

    func testInterceptorRefreshesTokenOn401() async throws {
        let refreshCalled = LockIsolated(false)
        let token = LockIsolated("bad_token")

        let mockAuth = AuthenticationClient(
            login: { _, _ in },
            register: { _, _ in },
            logout: {},
            isAuthenticated: { true },
            getAccessToken: { token.value },
            saveTokens: { _, _ in },
            refreshToken: {
                refreshCalled.setValue(true)
                token.setValue("recovered_token")
            },
            forgotPassword: { _ in }
        )

        try await withDependencies {
            $0.authentication = mockAuth
            $0.appSettings = .testValue
        } operation: {
            // Register MockURLProtocol
            let config = URLSessionConfiguration.ephemeral
            config.protocolClasses = [MockURLProtocol.self]
            let session = URLSession(configuration: config)

            // Setup Mock Handler
            MockURLProtocol.requestHandler = { request in
                if request.allHTTPHeaderFields?["Authorization"] == "Bearer recovered_token" {
                    // Success after refresh
                    let response = HTTPURLResponse(
                        url: request.url!, statusCode: 200, httpVersion: nil, headerFields: nil)!
                    return (response, #"{"items": []}"#.data(using: .utf8)!)
                } else {
                    // First failure
                    let response = HTTPURLResponse(
                        url: request.url!, statusCode: 401, httpVersion: nil, headerFields: nil)!
                    // 401 typically returns valid JSON error or empty
                    return (response, "{}".data(using: .utf8)!)
                }
            }

            // Instantiate live client with injected session
            let client = MusicRoomAPIClient.live(urlSession: session)

            // Call an endpoint that triggers the interceptor
            // search uses performRequest
            _ = try await client.search("test")

            XCTAssertTrue(refreshCalled.value)
        }
    }

    func testInterceptorFailsOnRepeated401() async throws {
        let refreshCalled = LockIsolated(false)
        let token = LockIsolated("bad_token")

        let mockAuth = AuthenticationClient(
            login: { _, _ in },
            register: { _, _ in },
            logout: {},
            isAuthenticated: { true },
            getAccessToken: { token.value },
            saveTokens: { _, _ in },
            refreshToken: {
                refreshCalled.setValue(true)
                token.setValue("recovered_token")
            },
            forgotPassword: { _ in }
        )

        await withDependencies {
            $0.authentication = mockAuth
            $0.appSettings = .testValue
        } operation: {
            let config = URLSessionConfiguration.ephemeral
            config.protocolClasses = [MockURLProtocol.self]
            let session = URLSession(configuration: config)

            MockURLProtocol.requestHandler = { request in
                // Always return 401
                let response = HTTPURLResponse(
                    url: request.url!, statusCode: 401, httpVersion: nil, headerFields: nil)!
                return (response, "{}".data(using: .utf8)!)
            }

            let client = MusicRoomAPIClient.live(urlSession: session)

            do {
                _ = try await client.search("test")
                XCTFail("Should have thrown error")
            } catch let error as MusicRoomAPIError {
                XCTAssertEqual(error, .sessionExpired)
            } catch {
                XCTFail("Unexpected error: \(error)")
            }

            XCTAssertTrue(refreshCalled.value)
        }
    }

    func testListEvents() async throws {
        // Setup data
        let events = [
            Event(
                id: UUID(),
                name: "Test Event",
                visibility: .publicEvent,
                ownerId: "u1",
                licenseMode: .everyone,
                createdAt: Date(),
                updatedAt: Date()
            )
        ]
        let data = try JSONEncoder.iso8601.encode(events)

        // Setup mock response
        MockURLProtocol.requestHandler = { request in
            XCTAssertEqual(request.url?.path, "/events")
            XCTAssertEqual(request.httpMethod, "GET")
            XCTAssertEqual(request.value(forHTTPHeaderField: "X-Platform"), "iOS")

            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!
            return (response, data)
        }

        // Execute
        // We need to override the AppSettings dependency to ensure the client picks up the base URL
        // In a real test we'd invoke the .liveValue within a `withDependencies` block
        // However, `MusicRoomAPIClient.liveValue` uses @Dependency which looks up from the current context.

        try await withDependencies { _ in
            // Configure AppSettings to point to a known base URL
            // Since we can't easily mock the struct inside .liveValue without AppSettingsClient access,
            // we rely on the default or ensuring the test environment provides it.
            // But verify "liveValue" logic uses the standard URLSession.shared which MockURLProtocol intercepts.
            // Actually, static .liveValue creates a NEW client each time now.
        } operation: {
            let client = MusicRoomAPIClient.liveValue
            let result = try await client.listEvents()

            XCTAssertEqual(result.count, 1)
            XCTAssertEqual(result.first?.name, "Test Event")
        }
    }

    func testVote() async throws {
        // Setup mock response
        let jsonString = "{\"voteCount\": 42}"
        let data = jsonString.data(using: .utf8)!

        MockURLProtocol.requestHandler = { request in
            XCTAssertTrue(request.url?.path.hasSuffix("/vote") == true)
            XCTAssertEqual(request.httpMethod, "POST")

            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 200,
                httpVersion: nil,
                headerFields: ["Content-Length": String(data.count)]
            )!
            return (response, data)
        }

        try await withDependencies { _ in
            // context
        } operation: {
            let client = MusicRoomAPIClient.liveValue
            let eventId = UUID()
            let result = try await client.vote(eventId.uuidString, "t1")
            XCTAssertEqual(result.voteCount, 42)
        }
    }
    func testGetEvent() async throws {
        let eventId = UUID()
        let event = Event(
            id: eventId,
            name: "Specific Event",
            visibility: .privateEvent,
            ownerId: "u2",
            licenseMode: .invitedOnly,
            createdAt: Date(),
            updatedAt: Date()
        )
        let data = try JSONEncoder.iso8601.encode(event)

        MockURLProtocol.requestHandler = { request in
            XCTAssertTrue(request.url?.path.contains(eventId.uuidString) == true)
            XCTAssertEqual(request.httpMethod, "GET")

            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!
            return (response, data)
        }

        try await withDependencies { _ in
        } operation: {
            let client = MusicRoomAPIClient.liveValue
            let result = try await client.getEvent(eventId)

            XCTAssertEqual(result.name, "Specific Event")
            XCTAssertEqual(result.id, eventId)
        }
    }

    func testTally() async throws {
        let eventId = UUID()
        let tallyItems = [
            MusicRoomAPIClient.TallyItem(track: "track1", count: 10),
            MusicRoomAPIClient.TallyItem(track: "track2", count: 5),
        ]
        let data = try JSONEncoder().encode(tallyItems)

        MockURLProtocol.requestHandler = { request in
            XCTAssertTrue(request.url?.path.contains("\(eventId.uuidString)/tally") == true)
            XCTAssertEqual(request.httpMethod, "GET")

            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!
            return (response, data)
        }

        try await withDependencies { _ in
        } operation: {
            let client = MusicRoomAPIClient.liveValue
            let result = try await client.tally(eventId)

            XCTAssertEqual(result.count, 2)
            XCTAssertEqual(result.first?.track, "track1")
            XCTAssertEqual(result.first?.count, 10)
        }
    }

    func testSearch() async throws {
        let items = [
            MusicSearchItem(
                title: "Song A", artist: "Artist A", provider: "deezer", providerTrackId: "1",
                thumbnailUrl: nil),
            MusicSearchItem(
                title: "Song B", artist: "Artist B", provider: "deezer", providerTrackId: "2",
                thumbnailUrl: nil),
        ]

        // The API client expects { "items": [...] }
        struct SearchResponse: Encodable {
            let items: [MusicSearchItem]
        }
        let responseBody = SearchResponse(items: items)
        let data = try JSONEncoder().encode(responseBody)

        MockURLProtocol.requestHandler = { request in
            XCTAssertEqual(request.url?.path, "/music/search")
            XCTAssertEqual(request.httpMethod, "GET")

            // Check query param
            let components = URLComponents(url: request.url!, resolvingAgainstBaseURL: false)
            XCTAssertEqual(
                components?.queryItems?.first(where: { $0.name == "query" })?.value, "daft punk")

            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!
            return (response, data)
        }

        try await withDependencies { _ in
        } operation: {
            let client = MusicRoomAPIClient.liveValue
            let result = try await client.search("daft punk")

            XCTAssertEqual(result.count, 2)
            XCTAssertEqual(result[0].title, "Song A")
            XCTAssertEqual(result[1].artist, "Artist B")
        }
    }
}

// Helper to extract body data from URLRequest
extension URLRequest {
    func streamData() -> Data {
        if let body = self.httpBody { return body }
        guard let stream = self.httpBodyStream else { return Data() }

        stream.open()
        var data = Data()
        let bufferSize = 1024
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: bufferSize)

        while stream.hasBytesAvailable {
            let read = stream.read(buffer, maxLength: bufferSize)
            if read > 0 {
                data.append(buffer, count: read)
            } else {
                break
            }
        }
        buffer.deallocate()
        stream.close()
        return data
    }
}

// Helper for date encoding matching the app's decoder
extension JSONEncoder {
    static var iso8601: JSONEncoder {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        return encoder
    }
}

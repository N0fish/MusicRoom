import ComposableArchitecture
import Foundation
import XCTest

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
        let voteResponse = VoteResponse(status: "voted", trackId: "t1", totalVotes: 42)
        let data = try JSONEncoder().encode(voteResponse)

        MockURLProtocol.requestHandler = { request in
            XCTAssertTrue(request.url?.path.hasSuffix("/vote") == true)
            XCTAssertEqual(request.httpMethod, "POST")

            // Verify body
            let bodyData = request.streamData()
            let body = try JSONDecoder().decode(VoteRequest.self, from: bodyData)
            XCTAssertEqual(body.trackId, "t1")

            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!
            return (response, data)
        }

        try await withDependencies { _ in
            // context
        } operation: {
            let client = MusicRoomAPIClient.liveValue
            let eventId = UUID()
            let result = try await client.vote(eventId, "t1", nil, nil)

            XCTAssertEqual(result.totalVotes, 42)
            XCTAssertEqual(result.status, "voted")
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

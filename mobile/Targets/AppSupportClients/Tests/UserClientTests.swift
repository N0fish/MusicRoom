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

    func testUpdateProfileEnrichesAuthInfo() async throws {
        UserMockURLProtocol.requestHandler = { request in
            guard let url = request.url else { fatalError("Missing URL") }

            let response = HTTPURLResponse(
                url: url,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!

            switch (url.path, request.httpMethod) {
            case ("/users/me", "PATCH"):
                let data = """
                    {
                        "id": "profile-1",
                        "userId": "user-1",
                        "username": "test",
                        "displayName": "Updated User",
                        "avatarUrl": null,
                        "hasCustomAvatar": false,
                        "bio": null,
                        "visibility": "public",
                        "preferences": {"genres": [], "artists": [], "moods": []},
                        "isPremium": false
                    }
                    """.data(using: .utf8)!
                return (response, data)

            case ("/auth/me", "GET"):
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

        let profile = UserProfile(
            id: "profile-1",
            userId: "user-1",
            username: "test",
            displayName: "Updated User",
            avatarUrl: nil,
            hasCustomAvatar: false,
            bio: nil,
            visibility: "public",
            preferences: UserPreferences(),
            isPremium: false,
            linkedProviders: [],
            email: nil
        )

        let updatedProfile = try await client.updateProfile(profile)

        XCTAssertEqual(updatedProfile.email, "test@example.com")
        XCTAssertEqual(updatedProfile.linkedProviders, ["google"])
    }

    func testUploadAvatar_UsesFileFormField() async throws {
        let didValidateUpload = LockIsolated(false)

        UserMockURLProtocol.requestHandler = { request in
            guard let url = request.url else { fatalError("Missing URL") }

            let response = HTTPURLResponse(
                url: url,
                statusCode: 200,
                httpVersion: nil,
                headerFields: nil
            )!

            switch (url.path, request.httpMethod) {
            case ("/users/me/avatar/upload", "POST"):
                let contentType = request.value(forHTTPHeaderField: "Content-Type") ?? ""
                XCTAssertTrue(contentType.contains("multipart/form-data"))

                let body = readRequestBody(from: request)
                XCTAssertNotNil(body.range(of: Data("name=\"file\"".utf8)))
                XCTAssertNotNil(body.range(of: Data("filename=\"avatar.jpg\"".utf8)))
                didValidateUpload.setValue(true)

                let data = """
                    {
                        "id": "profile-1",
                        "userId": "user-1",
                        "username": "test",
                        "displayName": "Test User",
                        "avatarUrl": "/avatars/custom/user-1.jpg",
                        "hasCustomAvatar": true,
                        "bio": null,
                        "visibility": "public",
                        "preferences": {"genres": [], "artists": [], "moods": []},
                        "isPremium": false
                    }
                    """.data(using: .utf8)!
                return (response, data)

            case ("/auth/me", "GET"):
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

        _ = try await client.uploadAvatar(Data([0x01, 0x02, 0x03]))
        XCTAssertTrue(didValidateUpload.value)
    }
}

private func readRequestBody(from request: URLRequest) -> Data {
    if let body = request.httpBody {
        return body
    }

    guard let stream = request.httpBodyStream else {
        return Data()
    }

    stream.open()
    defer { stream.close() }

    var data = Data()
    let bufferSize = 1024
    let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: bufferSize)
    defer { buffer.deallocate() }

    while stream.hasBytesAvailable {
        let read = stream.read(buffer, maxLength: bufferSize)
        if read > 0 {
            data.append(buffer, count: read)
        } else {
            break
        }
    }

    return data
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

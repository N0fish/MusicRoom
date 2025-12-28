import Dependencies
import Foundation
import XCTest

@testable import AppSupportClients

final class AuthenticatedRequestExecutorTests: XCTestCase {

    override func tearDown() {
        URLProtocol.unregisterClass(ExecutorMockURLProtocol.self)
        ExecutorMockURLProtocol.requestHandler = nil
        super.tearDown()
    }

    func testRefreshesAndRetriesOn401() async throws {
        let refreshCalled = LockIsolated(false)
        let token = LockIsolated("bad_token")

        let auth = AuthenticationClient(
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
        let sessionEvents = SessionEventsClient(
            stream: { AsyncStream { $0.finish() } },
            send: { _ in }
        )

        ExecutorMockURLProtocol.requestHandler = { request in
            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 401,
                httpVersion: nil,
                headerFields: nil
            )!

            if request.value(forHTTPHeaderField: "Authorization") == "Bearer recovered_token" {
                let successResponse = HTTPURLResponse(
                    url: request.url!,
                    statusCode: 200,
                    httpVersion: nil,
                    headerFields: nil
                )!
                return (successResponse, Data())
            }

            return (response, Data())
        }

        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [ExecutorMockURLProtocol.self]
        let session = URLSession(configuration: config)

        let executor = AuthenticatedRequestExecutor(
            urlSession: session,
            authentication: auth,
            sessionEvents: sessionEvents
        )
        let request = URLRequest(url: URL(string: "https://example.com/test")!)

        let (_, response) = try await executor.data(for: request)

        XCTAssertEqual(response.statusCode, 200)
        XCTAssertTrue(refreshCalled.value)
    }

    func testSessionExpiredAfterRepeated401() async throws {
        let refreshCalled = LockIsolated(false)
        let logoutCalled = LockIsolated(false)
        let sessionExpiredCalled = LockIsolated(false)
        let token = LockIsolated("bad_token")

        let auth = AuthenticationClient(
            login: { _, _ in },
            register: { _, _ in },
            logout: { logoutCalled.setValue(true) },
            isAuthenticated: { true },
            getAccessToken: { token.value },
            saveTokens: { _, _ in },
            refreshToken: {
                refreshCalled.setValue(true)
                token.setValue("recovered_token")
            },
            forgotPassword: { _ in }
        )
        let sessionEvents = SessionEventsClient(
            stream: { AsyncStream { $0.finish() } },
            send: { event in
                if event == .expired {
                    sessionExpiredCalled.setValue(true)
                }
            }
        )

        ExecutorMockURLProtocol.requestHandler = { request in
            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 401,
                httpVersion: nil,
                headerFields: nil
            )!
            return (response, Data())
        }

        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [ExecutorMockURLProtocol.self]
        let session = URLSession(configuration: config)

        let executor = AuthenticatedRequestExecutor(
            urlSession: session,
            authentication: auth,
            sessionEvents: sessionEvents
        )
        let request = URLRequest(url: URL(string: "https://example.com/test")!)

        do {
            _ = try await executor.data(for: request)
            XCTFail("Expected invalidCredentials error")
        } catch let error as AuthenticationError {
            XCTAssertEqual(error, .invalidCredentials)
        } catch {
            XCTFail("Unexpected error: \(error)")
        }

        XCTAssertTrue(refreshCalled.value)
        XCTAssertTrue(logoutCalled.value)
        XCTAssertTrue(sessionExpiredCalled.value)
    }

    func testRefreshFailureTriggersSessionExpired() async throws {
        let refreshCalled = LockIsolated(false)
        let logoutCalled = LockIsolated(false)
        let sessionExpiredCalled = LockIsolated(false)

        let auth = AuthenticationClient(
            login: { _, _ in },
            register: { _, _ in },
            logout: { logoutCalled.setValue(true) },
            isAuthenticated: { true },
            getAccessToken: { "bad_token" },
            saveTokens: { _, _ in },
            refreshToken: {
                refreshCalled.setValue(true)
                throw AuthenticationError.invalidCredentials
            },
            forgotPassword: { _ in }
        )
        let sessionEvents = SessionEventsClient(
            stream: { AsyncStream { $0.finish() } },
            send: { event in
                if event == .expired {
                    sessionExpiredCalled.setValue(true)
                }
            }
        )

        ExecutorMockURLProtocol.requestHandler = { request in
            let response = HTTPURLResponse(
                url: request.url!,
                statusCode: 401,
                httpVersion: nil,
                headerFields: nil
            )!
            return (response, Data())
        }

        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [ExecutorMockURLProtocol.self]
        let session = URLSession(configuration: config)

        let executor = AuthenticatedRequestExecutor(
            urlSession: session,
            authentication: auth,
            sessionEvents: sessionEvents
        )
        let request = URLRequest(url: URL(string: "https://example.com/test")!)

        do {
            _ = try await executor.data(for: request)
            XCTFail("Expected invalidCredentials error")
        } catch let error as AuthenticationError {
            XCTAssertEqual(error, .invalidCredentials)
        } catch {
            XCTFail("Unexpected error: \(error)")
        }

        XCTAssertTrue(refreshCalled.value)
        XCTAssertTrue(logoutCalled.value)
        XCTAssertTrue(sessionExpiredCalled.value)
    }
}

final class ExecutorMockURLProtocol: URLProtocol {
    nonisolated(unsafe) static var requestHandler: ((URLRequest) throws -> (HTTPURLResponse, Data))?

    override class func canInit(with request: URLRequest) -> Bool {
        true
    }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest {
        request
    }

    override func startLoading() {
        guard let handler = ExecutorMockURLProtocol.requestHandler else {
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

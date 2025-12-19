import AppSettingsClient
import AppSupportClients
import Dependencies
import Foundation
import MusicRoomDomain
import UIKit

public struct MusicRoomAPIClient: Sendable {
    public var fetchSampleEvents: @Sendable () async throws -> [Event]
    public var listEvents: @Sendable () async throws -> [Event]
    public var getEvent: @Sendable (UUID) async throws -> Event
    public var vote:
        @Sendable (_ playlistId: String, _ trackId: String) async throws
            -> VoteResponse
    public var nextTrack: @Sendable (_ playlistId: String) async throws -> NextTrackResponse
    public var tally: @Sendable (UUID) async throws -> [TallyItem]
    public var search: @Sendable (_ query: String) async throws -> [MusicSearchItem]
    public var createEvent: @Sendable (CreateEventRequest) async throws -> Event
    public var addTrack: @Sendable (String, AddTrackRequest) async throws -> Track
    public var connectToRealtime: @Sendable () -> AsyncStream<RealtimeMessage>
    public var removeTrack: @Sendable (_ playlistId: String, _ trackId: String) async throws -> Void
    public var authMe: @Sendable () async throws -> AuthMeResponse
    public var getPlaylist: @Sendable (_ playlistId: String) async throws -> PlaylistResponse
    public var inviteUser: @Sendable (_ eventId: UUID, _ userId: String) async throws -> Void
    public var listInvites: @Sendable (_ eventId: UUID) async throws -> [Invite]
    public var leaveEvent: @Sendable (_ eventId: UUID, _ userId: String) async throws -> Void
    public var deleteEvent: @Sendable (_ eventId: UUID) async throws -> Void
    public var joinEvent: @Sendable (_ eventId: UUID) async throws -> Void
    public var transferOwnership:
        @Sendable (_ eventId: UUID, _ newOwnerId: String) async throws -> Void
    public var patchEvent:
        @Sendable (_ eventId: UUID, _ request: PatchEventRequest) async throws -> Event

    public struct Invite: Decodable, Sendable, Equatable {
        public let userId: String
        public let createdAt: Date
    }

    public struct AuthMeResponse: Decodable, Sendable {
        public let userId: String
        public let email: String
        public let emailVerified: Bool
        public let linkedProviders: [String]?
    }

    public struct TallyItem: Codable, Sendable, Equatable {
        public let track: String
        public let count: Int
        public let isMyVote: Bool?

        public init(track: String, count: Int, isMyVote: Bool? = nil) {
            self.track = track
            self.count = count
            self.isMyVote = isMyVote
        }
    }

}

extension DependencyValues {
    public var musicRoomAPI: MusicRoomAPIClient {
        get { self[MusicRoomAPIClient.self] }
        set { self[MusicRoomAPIClient.self] = newValue }
    }
}

extension MusicRoomAPIClient: DependencyKey {
    public static var liveValue: MusicRoomAPIClient {
        live()
    }

    public static func live(urlSession: URLSession = .shared) -> MusicRoomAPIClient {
        @Dependency(\.appSettings) var settings
        @Dependency(\.authentication) var authentication

        let appVersion =
            Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "Unknown"

        @Sendable func performRequest<T: Decodable & Sendable>(
            _ request: URLRequest, retryCount: Int = 0
        )
            async throws -> T
        {
            var request = request
            // Headers
            request.setValue("iOS", forHTTPHeaderField: "X-Platform")

            let deviceName = await MainActor.run { UIDevice.current.name }
            request.setValue(deviceName, forHTTPHeaderField: "X-Device")
            request.setValue(appVersion, forHTTPHeaderField: "X-App-Version")

            // Auth
            if let token = authentication.getAccessToken() {
                request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
            }

            do {
                let (data, response) = try await urlSession.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse else {
                    throw MusicRoomAPIError.networkError("Invalid response")
                }

                // Token Refresh (401)
                if httpResponse.statusCode == 401 {
                    if retryCount < 1 {
                        do {
                            try await authentication.refreshToken()
                            // Update token in new request is handled by recursive call
                            return try await performRequest(request, retryCount: retryCount + 1)
                        } catch {
                            throw MusicRoomAPIError.sessionExpired
                        }
                    } else {
                        throw MusicRoomAPIError.sessionExpired
                    }
                }

                // Error Mapping
                switch httpResponse.statusCode {
                case 200...299:
                    break
                case 403:
                    throw MusicRoomAPIError.forbidden
                case 404:
                    throw MusicRoomAPIError.notFound
                default:
                    throw MusicRoomAPIError.serverError(statusCode: httpResponse.statusCode)
                }

                let decoder = JSONDecoder()
                decoder.dateDecodingStrategy = .iso8601
                return try decoder.decode(T.self, from: data)
            } catch let error as MusicRoomAPIError {
                throw error
            } catch {
                throw MusicRoomAPIError.networkError(error.localizedDescription)
            }
        }

        @Sendable func performRequestNoContent(_ request: URLRequest, retryCount: Int = 0)
            async throws
        {
            var request = request
            // ... same headers ...
            request.setValue("iOS", forHTTPHeaderField: "X-Platform")

            let deviceName = await MainActor.run { UIDevice.current.name }
            request.setValue(deviceName, forHTTPHeaderField: "X-Device")
            request.setValue(appVersion, forHTTPHeaderField: "X-App-Version")

            if let token = authentication.getAccessToken() {
                request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
            }

            do {
                let (_, response) = try await urlSession.data(for: request)

                guard let httpResponse = response as? HTTPURLResponse else {
                    throw MusicRoomAPIError.networkError("Invalid response")
                }

                if httpResponse.statusCode == 401 {
                    if retryCount < 1 {
                        do {
                            try await authentication.refreshToken()
                            return try await performRequestNoContent(
                                request, retryCount: retryCount + 1)
                        } catch {
                            throw MusicRoomAPIError.sessionExpired
                        }
                    } else {
                        throw MusicRoomAPIError.sessionExpired
                    }
                }

                switch httpResponse.statusCode {
                case 200...299:
                    break
                case 403:
                    throw MusicRoomAPIError.forbidden
                case 404:
                    throw MusicRoomAPIError.notFound
                default:
                    throw MusicRoomAPIError.serverError(statusCode: httpResponse.statusCode)
                }
            } catch let error as MusicRoomAPIError {
                throw error
            } catch {
                throw MusicRoomAPIError.networkError(error.localizedDescription)
            }
        }

        return MusicRoomAPIClient(
            fetchSampleEvents: {
                // Return empty list or call listEvents for now
                return []
            },
            listEvents: {
                let url = settings.load().backendURL.appendingPathComponent("events")
                var request = URLRequest(url: url)
                request.httpMethod = "GET"
                return try await performRequest(request)
            },
            getEvent: { id in
                let url = settings.load().backendURL.appendingPathComponent("events")
                    .appendingPathComponent(id.uuidString)
                var request = URLRequest(url: url)
                request.httpMethod = "GET"
                return try await performRequest(request)
            },
            vote: { playlistId, trackId in
                let url = settings.load().backendURL
                    .appendingPathComponent("playlists")
                    .appendingPathComponent(playlistId)
                    .appendingPathComponent("tracks")
                    .appendingPathComponent(trackId)
                    .appendingPathComponent("vote")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                return try await performRequest(request)
            },
            nextTrack: { playlistId in
                let url = settings.load().backendURL
                    .appendingPathComponent("playlists")
                    .appendingPathComponent(playlistId)
                    .appendingPathComponent("next")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                return try await performRequest(request)
            },
            tally: { eventId in
                let url = settings.load().backendURL.appendingPathComponent("events")
                    .appendingPathComponent(eventId.uuidString)
                    .appendingPathComponent("tally")
                var request = URLRequest(url: url)
                request.httpMethod = "GET"
                return try await performRequest(request)
            },
            search: { query in
                let url = settings.load().backendURL.appendingPathComponent("music")
                    .appendingPathComponent("search")
                var components = URLComponents(url: url, resolvingAgainstBaseURL: true)!
                components.queryItems = [URLQueryItem(name: "query", value: query)]
                let request = URLRequest(url: components.url!)
                // Use default GET

                struct SearchResponse: Decodable {
                    let items: [MusicSearchItem]
                }

                let response: SearchResponse = try await performRequest(request)
                return response.items
            },
            createEvent: { requestBody in
                let url = settings.load().backendURL.appendingPathComponent("events")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                request.httpBody = try JSONEncoder.iso8601.encode(requestBody)
                return try await performRequest(request)
            },
            addTrack: { playlistId, trackReq in
                let url = settings.load().backendURL.appendingPathComponent(
                    "playlists/\(playlistId)/tracks")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                request.httpBody = try JSONEncoder().encode(trackReq)
                return try await performRequest(request)
            },
            connectToRealtime: {
                // Construct WS URL
                let currentBackendURL = settings.load().backendURL
                guard let host = currentBackendURL.host,
                    let port = currentBackendURL.port
                else {
                    return AsyncStream { $0.finish() }
                }
                // Convert http/https to ws/wss (simple hack for MVP)
                let scheme = currentBackendURL.scheme == "https" ? "wss" : "ws"

                var components = URLComponents()
                components.scheme = scheme
                // Fix for iOS Simulator: 'localhost' can cause socket errors (SO_CONNECTION_IDLE), use 127.0.0.1
                components.host = host == "localhost" ? "127.0.0.1" : host
                components.port = port  // Use same port as API for now (Gateway)
                components.path = "/ws"

                // Pass token in query param or header?
                // WS standard limits headers in browser, but URLSession supports it.
                // However, many backends expect token in query/protocol.
                // For now, let's try appending token to query if available?
                // Or standard "Authorization" header if backend supports it for WS upgrade.
                // Let's assume header for now.

                guard let url = components.url else { return AsyncStream { $0.finish() } }

                var request = URLRequest(url: url)
                if let token = authentication.getAccessToken() {
                    request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
                }

                return AsyncStream { continuation in
                    let task = URLSession.shared.webSocketTask(with: request)
                    task.resume()

                    // Simple recursive receiver
                    @Sendable func receive() {
                        task.receive { result in
                            switch result {
                            case .success(let message):
                                switch message {
                                case .data(let data):
                                    if let msg = try? JSONDecoder().decode(
                                        RealtimeMessage.self, from: data)
                                    {
                                        continuation.yield(msg)
                                    }
                                case .string(let text):
                                    if let data = text.data(using: .utf8),
                                        let msg = try? JSONDecoder().decode(
                                            RealtimeMessage.self, from: data)
                                    {
                                        continuation.yield(msg)
                                    }
                                @unknown default:
                                    break
                                }
                                receive()  // Continue listening

                            case .failure(let error):
                                print("WS Error: \(error)")
                                continuation.finish()
                            }
                        }
                    }

                    receive()

                    continuation.onTermination = { _ in
                        task.cancel(with: .normalClosure, reason: nil)
                    }
                }
            },
            removeTrack: { playlistId, trackId in
                let url = settings.load().backendURL.appendingPathComponent(
                    "playlists/\(playlistId)/tracks/\(trackId)")
                var request = URLRequest(url: url)
                request.httpMethod = "DELETE"
                try await performRequestNoContent(request)
            },
            authMe: {
                let url = settings.load().backendURL.appendingPathComponent("auth/me")
                var request = URLRequest(url: url)
                request.httpMethod = "GET"
                return try await performRequest(request)
            },
            getPlaylist: { playlistId in
                let url = settings.load().backendURL.appendingPathComponent(
                    "playlists/\(playlistId)")
                let request = URLRequest(url: url)
                return try await performRequest(request)
            },
            inviteUser: { eventId, userId in
                let url = settings.load().backendURL.appendingPathComponent("events")
                    .appendingPathComponent(eventId.uuidString)
                    .appendingPathComponent(userId)
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                try await performRequestNoContent(request)
            },
            listInvites: { eventId in
                let url = settings.load().backendURL.appendingPathComponent(
                    "events/\(eventId.uuidString)/invites")
                var request = URLRequest(url: url)
                request.httpMethod = "GET"
                return try await performRequest(request)
            },
            leaveEvent: { eventId, userId in
                let url = settings.load().backendURL.appendingPathComponent("events")
                    .appendingPathComponent(eventId.uuidString)
                    .appendingPathComponent("invites")
                    .appendingPathComponent(userId)
                var request = URLRequest(url: url)
                request.httpMethod = "DELETE"
                try await performRequestNoContent(request)
            },
            deleteEvent: { eventId in
                let url = settings.load().backendURL.appendingPathComponent("events")
                    .appendingPathComponent(eventId.uuidString)
                var request = URLRequest(url: url)
                request.httpMethod = "DELETE"
                try await performRequestNoContent(request)
            },
            joinEvent: { eventId in
                let url = settings.load().backendURL.appendingPathComponent("events")
                    .appendingPathComponent(eventId.uuidString)
                    .appendingPathComponent("invites")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                // We need the user ID. Fetch from auth/me manually since we can't reference self.authMe here
                let authUrl = settings.load().backendURL.appendingPathComponent("auth/me")
                var authRequest = URLRequest(url: authUrl)
                authRequest.httpMethod = "GET"
                let me: AuthMeResponse = try await performRequest(authRequest)

                let body = ["userId": me.userId]
                request.httpBody = try JSONEncoder().encode(body)

                try await performRequestNoContent(request)
            },
            transferOwnership: { eventId, newOwnerId in
                let url = settings.load().backendURL.appendingPathComponent("events")
                    .appendingPathComponent(eventId.uuidString)
                    .appendingPathComponent("transfer-ownership")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = ["newOwnerId": newOwnerId]
                request.httpBody = try JSONEncoder().encode(body)

                try await performRequestNoContent(request)
            },
            patchEvent: { eventId, requestBody in
                let url = settings.load().backendURL.appendingPathComponent("events")
                    .appendingPathComponent(eventId.uuidString)
                var request = URLRequest(url: url)
                request.httpMethod = "PATCH"
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                request.httpBody = try JSONEncoder.iso8601.encode(requestBody)
                return try await performRequest(request)
            }
        )
    }

    private static func validate(response: URLResponse) throws {
        guard let httpResponse = response as? HTTPURLResponse else {
            throw MusicRoomAPIError.networkError("Invalid response")
        }
        switch httpResponse.statusCode {
        case 200...299:
            break
        case 401:
            throw MusicRoomAPIError.sessionExpired
        case 403:
            throw MusicRoomAPIError.forbidden
        case 404:
            throw MusicRoomAPIError.notFound
        default:
            throw MusicRoomAPIError.serverError(statusCode: httpResponse.statusCode)
        }
    }

    public static var previewValue: MusicRoomAPIClient {
        MusicRoomAPIClient(
            fetchSampleEvents: { [] },
            listEvents: { MockDataFactory.sampleEvents() },
            getEvent: { _ in MockDataFactory.sampleEvents().first! },
            vote: { _, _ in VoteResponse(voteCount: 5) },
            nextTrack: { _ in
                NextTrackResponse(
                    playlistId: "1", currentTrackId: "2", playingStartedAt: Date(),
                    status: "playing")
            },
            tally: { _ in [] },
            search: { _ in
                [
                    MusicSearchItem(
                        title: "Get Lucky", artist: "Daft Punk", provider: "deezer",
                        providerTrackId: "1", thumbnailUrl: nil),
                    MusicSearchItem(
                        title: "Instant Crush", artist: "Daft Punk", provider: "deezer",
                        providerTrackId: "2", thumbnailUrl: nil),
                ]
            },
            createEvent: { _ in MockDataFactory.sampleEvents().first! },
            addTrack: { _, req in
                Track(
                    title: req.title, artist: req.artist, provider: req.provider,
                    providerTrackId: req.providerTrackId,
                    thumbnailUrl: URL(string: req.thumbnailUrl))
            },
            connectToRealtime: { AsyncStream { $0.finish() } },
            removeTrack: { _, _ in },
            authMe: {
                AuthMeResponse(
                    userId: "user1", email: "test@example.com", emailVerified: true,
                    linkedProviders: ["google"])
            },
            getPlaylist: { _ in
                PlaylistResponse(
                    playlist: PlaylistResponse.PlaylistMetadata(
                        id: "1", ownerId: "user1", name: "Mock Playlist", isPublic: true,
                        editMode: "everyone"),
                    tracks: [
                        Track(
                            id: "1", title: "Get Lucky", artist: "Daft Punk", provider: "deezer",
                            providerTrackId: "1", thumbnailUrl: nil)
                    ]
                )
            },
            inviteUser: { _, _ in },
            listInvites: { _ in
                [
                    Invite(userId: "user2", createdAt: Date()),
                    Invite(userId: "user3", createdAt: Date()),
                ]
            },
            leaveEvent: { _, _ in },
            deleteEvent: { _ in },
            joinEvent: { _ in },
            transferOwnership: { _, _ in },
            patchEvent: { _, _ in MockDataFactory.sampleEvents().first! }
        )
    }

    public static var testValue: MusicRoomAPIClient {
        MusicRoomAPIClient(
            fetchSampleEvents: { [] },
            listEvents: { [] },
            getEvent: { _ in throw MusicRoomAPIError.networkError("Test unimplemented") },
            vote: { _, _ in VoteResponse(voteCount: 1) },
            nextTrack: { _ in
                NextTrackResponse(
                    playlistId: "1", currentTrackId: "2", playingStartedAt: Date(),
                    status: "playing")
            },
            tally: { _ in [] },
            search: { _ in [] },
            createEvent: { _ in throw MusicRoomAPIError.networkError("Test unimplemented") },
            addTrack: { _, _ in throw MusicRoomAPIError.networkError("Test unimplemented") },
            connectToRealtime: { AsyncStream { $0.finish() } },
            removeTrack: { _, _ in },
            authMe: {
                AuthMeResponse(
                    userId: "user1", email: "test@example.com", emailVerified: true,
                    linkedProviders: [])
            }, getPlaylist: { _ in throw MusicRoomAPIError.networkError("Test unimplemented") },
            inviteUser: { _, _ in },
            listInvites: { _ in [] },
            leaveEvent: { _, _ in },
            deleteEvent: { _ in },
            joinEvent: { _ in },
            transferOwnership: { _, _ in },
            patchEvent: { _, _ in throw MusicRoomAPIError.networkError("Test unimplemented") }
        )
    }
}

public enum MusicRoomAPIError: Error, Equatable, LocalizedError {
    case networkError(String)
    case serverError(statusCode: Int)
    case sessionExpired
    case forbidden
    case notFound

    public var errorDescription: String? {
        switch self {
        case .networkError(let message): return "Network Error: \(message)"
        case .serverError(let code): return "Server Error: \(code)"
        case .sessionExpired: return "Session Expired"
        case .forbidden: return "Access Denied"
        case .notFound: return "Not Found"
        }
    }
}

// Helper for date decoding
extension JSONDecoder {
    static var iso8601: JSONDecoder {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return decoder
    }
}

extension JSONEncoder {
    static var iso8601: JSONEncoder {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        return encoder
    }
}

extension URLRequest {
    mutating func addCommonHeaders() {
        self.setValue("iOS", forHTTPHeaderField: "X-Platform")
        // UIDevice.current is MainActor isolated, so we use a generic placeholder or need to perform this async
        self.setValue("iOS Device", forHTTPHeaderField: "X-Device")
        if let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String {
            self.setValue(version, forHTTPHeaderField: "X-App-Version")
        }
    }
}

private enum MockDataFactory {
    static func sampleEvents() -> [Event] {
        return [
            Event(
                id: UUID(),
                name: "Friday Party",
                visibility: .publicEvent,
                ownerId: "user1",
                licenseMode: .everyone,
                createdAt: Date(),
                updatedAt: Date()
            ),
            Event(
                id: UUID(),
                name: "Private Lounge",
                visibility: .privateEvent,
                ownerId: "user2",
                licenseMode: .invitedOnly,
                createdAt: Date(),
                updatedAt: Date()
            ),
        ]
    }
}

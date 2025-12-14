import AppSettingsClient
import Dependencies
import Foundation
import MusicRoomDomain
import UIKit

public struct MusicRoomAPIClient: Sendable {
    public var fetchSampleEvents: @Sendable () async throws -> [Event]
    public var listEvents: @Sendable () async throws -> [Event]
    public var getEvent: @Sendable (UUID) async throws -> Event
    public var vote:
        @Sendable (_ eventId: UUID, _ trackId: String, _ lat: Double?, _ lng: Double?) async throws
            -> VoteResponse
    public var tally: @Sendable (UUID) async throws -> [TallyItem]
    public var search: @Sendable (_ query: String) async throws -> [MusicSearchItem]
    public var createEvent: @Sendable (CreateEventRequest) async throws -> Event
    public var addTrack: @Sendable (String, AddTrackRequest) async throws -> Track
    public var connectToRealtime: @Sendable () -> AsyncStream<RealtimeMessage>
    public var removeTrack: @Sendable (_ playlistId: String, _ trackId: String) async throws -> Void
    public var getPlaylist: @Sendable (_ playlistId: String) async throws -> PlaylistResponse

    public struct TallyItem: Codable, Sendable, Equatable {
        public let track: String
        public let count: Int

        public init(track: String, count: Int) {
            self.track = track
            self.count = count
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
        @Dependency(\.appSettings) var settings

        return MusicRoomAPIClient(
            fetchSampleEvents: {
                // Return empty list or call listEvents for now
                return []
            },
            listEvents: {
                let url = settings.load().backendURL.appendingPathComponent("events")
                var request = URLRequest(url: url)
                request.addCommonHeaders()
                let (data, _) = try await URLSession.shared.data(for: request)
                return try JSONDecoder.iso8601.decode([Event].self, from: data)
            },
            getEvent: { id in
                let url = settings.load().backendURL.appendingPathComponent(
                    "events/\(id.uuidString)")
                var request = URLRequest(url: url)
                request.addCommonHeaders()
                let (data, _) = try await URLSession.shared.data(for: request)
                return try JSONDecoder.iso8601.decode(Event.self, from: data)
            },
            vote: { eventId, trackId, lat, lng in
                let url = settings.load().backendURL.appendingPathComponent(
                    "events/\(eventId.uuidString)/vote")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addCommonHeaders()
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                let body = VoteRequest(trackId: trackId, lat: lat, lng: lng)
                request.httpBody = try JSONEncoder().encode(body)

                let (data, _) = try await URLSession.shared.data(for: request)
                return try JSONDecoder().decode(VoteResponse.self, from: data)
            },
            tally: { eventId in
                let url = settings.load().backendURL.appendingPathComponent(
                    "events/\(eventId.uuidString)/tally")
                var request = URLRequest(url: url)
                request.addCommonHeaders()
                let (data, _) = try await URLSession.shared.data(for: request)
                return try JSONDecoder().decode([TallyItem].self, from: data)
            },
            search: { query in
                var url = settings.load().backendURL.appendingPathComponent("music/search")
                url.append(queryItems: [URLQueryItem(name: "query", value: query)])

                var request = URLRequest(url: url)
                request.addCommonHeaders()

                let (data, _) = try await URLSession.shared.data(for: request)
                struct SearchResponse: Decodable {
                    let items: [MusicSearchItem]
                }
                return try JSONDecoder().decode(SearchResponse.self, from: data).items
            },
            createEvent: { requestBody in
                let url = settings.load().backendURL.appendingPathComponent("events")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addCommonHeaders()
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                request.httpBody = try JSONEncoder.iso8601.encode(requestBody)

                let (data, _) = try await URLSession.shared.data(for: request)
                return try JSONDecoder.iso8601.decode(Event.self, from: data)
            },
            addTrack: { playlistId, trackReq in
                let url = settings.load().backendURL.appendingPathComponent(
                    "playlists/\(playlistId)/tracks")
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addCommonHeaders()
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")

                request.httpBody = try JSONEncoder().encode(trackReq)

                let (data, response) = try await URLSession.shared.data(for: request)
                if let httpResponse = response as? HTTPURLResponse,
                    !(200...299).contains(httpResponse.statusCode)
                {
                    throw MusicRoomAPIError.serverError(statusCode: httpResponse.statusCode)
                }
                return try JSONDecoder.iso8601.decode(Track.self, from: data)
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
                // Usually gateway routes /ws -> realtime-service. Assuming direct or gateway config.
                // Based on realtime-service.md: ws://localhost:3004/ws
                // If via gateway (port 8080), it might not proxy websockets correctly without config?
                // Let's assume gateway proxies /ws or we use direct port 3004 if running locally?
                // For MVP, let's use the gateway URL but change protocol.

                // Note: If using gateway, the path might be /ws
                var components = URLComponents()
                components.scheme = scheme
                components.host = host
                components.port = port  // Use same port as API for now (Gateway)
                components.path = "/ws"

                guard let url = components.url else { return AsyncStream { $0.finish() } }

                return AsyncStream { continuation in
                    let task = URLSession.shared.webSocketTask(with: url)
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
                request.addCommonHeaders()

                let (_, response) = try await URLSession.shared.data(for: request)
                if let httpResponse = response as? HTTPURLResponse,
                    !(200...299).contains(httpResponse.statusCode)
                {
                    throw MusicRoomAPIError.serverError(statusCode: httpResponse.statusCode)
                }
            },
            getPlaylist: { playlistId in
                let url = settings.load().backendURL.appendingPathComponent(
                    "playlists/\(playlistId)")
                var request = URLRequest(url: url)
                request.addCommonHeaders()
                let (data, _) = try await URLSession.shared.data(for: request)
                return try JSONDecoder.iso8601.decode(PlaylistResponse.self, from: data)
            }
        )
    }

    public static var previewValue: MusicRoomAPIClient {
        MusicRoomAPIClient(
            fetchSampleEvents: { [] },
            listEvents: { MockDataFactory.sampleEvents() },
            getEvent: { _ in MockDataFactory.sampleEvents().first! },
            vote: { _, _, _, _ in VoteResponse(status: "ok", trackId: "1", totalVotes: 5) },
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
            }
        )
    }

    public static var testValue: MusicRoomAPIClient {
        MusicRoomAPIClient(
            fetchSampleEvents: { [] },
            listEvents: { [] },
            getEvent: { _ in throw MusicRoomAPIError.networkError("Test unimplemented") },
            vote: { _, _, _, _ in VoteResponse(status: "ok", trackId: "1", totalVotes: 1) },
            tally: { _ in [] },
            search: { _ in [] },
            createEvent: { _ in throw MusicRoomAPIError.networkError("Test unimplemented") },
            addTrack: { _, _ in throw MusicRoomAPIError.networkError("Test unimplemented") },
            connectToRealtime: { AsyncStream { $0.finish() } },
            removeTrack: { _, _ in },
            getPlaylist: { _ in throw MusicRoomAPIError.networkError("Test unimplemented") }
        )
    }
}

public enum MusicRoomAPIError: Error, Equatable, LocalizedError {
    case networkError(String)
    case serverError(statusCode: Int)

    public var errorDescription: String? {
        switch self {
        case .networkError(let message): return "Network Error: \(message)"
        case .serverError(let code): return "Server Error: \(code)"
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

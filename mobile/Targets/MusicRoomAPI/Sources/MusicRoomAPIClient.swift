import Dependencies
import Foundation
import MusicRoomDomain

public struct MusicRoomAPIClient: Sendable {
    public var fetchSampleEvents: @Sendable () async throws -> [Event]

    public init(fetchSampleEvents: @escaping @Sendable () async throws -> [Event]) {
        self.fetchSampleEvents = fetchSampleEvents
    }
}

public enum MusicRoomAPIError: Error, Equatable, LocalizedError {
    // case offline // Removed for debugging
    case networkError(String)
    case serverError(statusCode: Int)

    public var errorDescription: String? {
        switch self {
        case .networkError(let message): return "Network Error: \(message)"
        case .serverError(let code): return "Server Error: \(code)"
        }
    }
}

extension MusicRoomAPIClient: DependencyKey {
    public static let liveValue = MusicRoomAPIClient {
        // TODO: Retrieve configured backend URL from dependencies or configuration
        // For now, we assume a standard path relative to a base URL, or we might need to inject the base URL.
        // Since this is a Dependency, we might need to access AppSettings.
        // However, usually the API client is configured with a base URL or retrieves it.
        // Given the current architecture, we'll use a placeholder or assume a default for now,
        // as connecting it to AppSettings might require a layer of indirection or configuration injection.

        // In a real app, we'd likely have a `APIConfiguration` dependency.
        // For this step, we'll implement the network call structure.

        let url = URL(string: "http://localhost:8080/mock/events")!
        var request = URLRequest(url: url)
        request.httpMethod = "GET"

        do {
            let (data, response) = try await URLSession.shared.data(for: request)
            guard let httpResponse = response as? HTTPURLResponse else {
                throw MusicRoomAPIError.networkError("Invalid response")
            }
            guard (200...299).contains(httpResponse.statusCode) else {
                throw MusicRoomAPIError.serverError(statusCode: httpResponse.statusCode)
            }

            let decoder = JSONDecoder()
            decoder.dateDecodingStrategy = .iso8601

            let backendEvents = try decoder.decode([BackendEvent].self, from: data)
            return backendEvents.map { $0.toDomain() }
        } catch {
            throw MusicRoomAPIError.networkError(error.localizedDescription)
        }
    }

    public static let previewValue = MusicRoomAPIClient {
        MockDataFactory.sampleEvents()
    }

    public static let testValue = MusicRoomAPIClient {
        []
    }
}

// MARK: - Backend DTOs

private struct BackendEvent: Decodable {
    let id: String
    let name: String
    let playlist: BackendPlaylist
    let startedAt: Date

    func toDomain() -> Event {
        Event(
            id: UUID(uuidString: id) ?? UUID(),
            name: name,
            location: "Unknown Location",  // Default
            visibility: .publicEvent,  // Default
            licenseTier: .everyone,  // Default
            startTime: startedAt,
            playlist: playlist.tracks.map { $0.toDomain() }
        )
    }
}

private struct BackendPlaylist: Decodable {
    let tracks: [BackendTrack]
}

private struct BackendTrack: Decodable {
    let title: String
    let artist: String
    let votes: Int?

    func toDomain() -> Track {
        Track(
            id: UUID(),
            title: title,
            artist: artist,
            votes: votes ?? 0
        )
    }
}

extension DependencyValues {
    public var musicRoomAPI: MusicRoomAPIClient {
        get { self[MusicRoomAPIClient.self] }
        set { self[MusicRoomAPIClient.self] = newValue }
    }
}

private enum MockDataFactory {
    static func sampleEvents() -> [Event] {
        let now = Date()
        let playlist = [
            Track(title: "Morning Light", artist: "Atlas Sky", votes: 12),
            Track(title: "Neon Drift", artist: "Pulse Theory", votes: 9),
            Track(title: "Velvet Dawn", artist: "Nova", votes: 7),
        ]
        return [
            Event(
                name: "Loft Sessions",
                location: "Brooklyn",
                visibility: .publicEvent,
                licenseTier: .everyone,
                startTime: now.addingTimeInterval(3600),
                playlist: playlist
            ),
            Event(
                name: "Secret Showcase",
                location: "LA",
                visibility: .privateInvite,
                licenseTier: .invitedOnly,
                startTime: now.addingTimeInterval(7200),
                playlist: playlist.shuffled()
            ),
        ]
    }
}

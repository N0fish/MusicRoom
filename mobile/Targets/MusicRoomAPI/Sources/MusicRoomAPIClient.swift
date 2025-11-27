import Foundation
import Dependencies
import MusicRoomDomain

public struct MusicRoomAPIClient: Sendable {
    public var fetchSampleEvents: @Sendable () async throws -> [Event]

    public init(fetchSampleEvents: @escaping @Sendable () async throws -> [Event]) {
        self.fetchSampleEvents = fetchSampleEvents
    }
}

public enum MusicRoomAPIError: Error, Equatable {
    case offline
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
        
        let url = URL(string: "https://api.musicroom.app/api/v1/events/sample")! // Placeholder
        var request = URLRequest(url: url)
        request.httpMethod = "GET"
        
        do {
            let (data, response) = try await URLSession.shared.data(for: request)
            guard let httpResponse = response as? HTTPURLResponse, (200...299).contains(httpResponse.statusCode) else {
                throw MusicRoomAPIError.offline // Map other errors appropriately
            }
            return try JSONDecoder().decode([Event].self, from: data)
        } catch {
            throw MusicRoomAPIError.offline
        }
    }

    public static let previewValue = MusicRoomAPIClient {
        MockDataFactory.sampleEvents()
    }

    public static let testValue = MusicRoomAPIClient {
        []
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
            Track(title: "Velvet Dawn", artist: "Nova", votes: 7)
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
            )
        ]
    }
}

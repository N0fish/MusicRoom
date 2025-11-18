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
        try await Task.sleep(nanoseconds: 150_000_000)
        return MockDataFactory.sampleEvents()
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

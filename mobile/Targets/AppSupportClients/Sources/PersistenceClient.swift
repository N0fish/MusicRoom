import Dependencies
import Foundation
import MusicRoomDomain

public struct PersistenceClient: Sendable {
    public var saveEvents: @Sendable ([Event]) async throws -> Void
    public var loadEvents: @Sendable () async throws -> [Event]
    public var savePlaylist: @Sendable (PlaylistResponse) async throws -> Void
    public var loadPlaylist: @Sendable () async throws -> PlaylistResponse
}

extension DependencyValues {
    public var persistence: PersistenceClient {
        get { self[PersistenceClient.self] }
        set { self[PersistenceClient.self] = newValue }
    }
}

extension PersistenceClient: DependencyKey {
    public static var liveValue: PersistenceClient {
        let actor = PersistenceActor()
        return PersistenceClient(
            saveEvents: { events in
                try await actor.save(events, to: "events_cache.json")
            },
            loadEvents: {
                try await actor.load([Event].self, from: "events_cache.json")
            },
            savePlaylist: { playlist in
                try await actor.save(playlist, to: "playlist_cache.json")
            },
            loadPlaylist: {
                try await actor.load(PlaylistResponse.self, from: "playlist_cache.json")
            }
        )
    }

    public static var testValue: PersistenceClient {
        PersistenceClient(
            saveEvents: { _ in },
            loadEvents: { [] },
            savePlaylist: { _ in throw PersistenceError.notFound },
            loadPlaylist: { throw PersistenceError.notFound }
        )
    }

    public static var previewValue: PersistenceClient {
        testValue
    }
}

private actor PersistenceActor {
    private let decoder: JSONDecoder
    private let encoder: JSONEncoder

    init() {
        self.decoder = JSONDecoder()
        self.decoder.dateDecodingStrategy = .iso8601
        self.encoder = JSONEncoder()
        self.encoder.dateEncodingStrategy = .iso8601
    }

    func save<T: Encodable>(_ value: T, to filename: String) throws {
        let data = try encoder.encode(value)
        let url = try fileURL(for: filename)
        try data.write(to: url)
    }

    func load<T: Decodable>(_ type: T.Type, from filename: String) throws -> T {
        let url = try fileURL(for: filename)
        let data = try Data(contentsOf: url)
        return try decoder.decode(type, from: data)
    }

    private func fileURL(for filename: String) throws -> URL {
        let paths = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)
        guard let docDir = paths.first else { throw PersistenceError.documentDirectoryNotFound }
        return docDir.appendingPathComponent(filename)
    }
}

public enum PersistenceError: Error {
    case documentDirectoryNotFound
    case notFound
}

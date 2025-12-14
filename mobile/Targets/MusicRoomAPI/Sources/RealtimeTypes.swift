import Foundation
import MusicRoomDomain

public struct RealtimeMessage: Decodable, Sendable, Equatable {
    public let type: String
    public let payload: AnyDecodable
}

// Wrapper to handle 'Any' decoding
public struct AnyDecodable: Decodable, @unchecked Sendable, Equatable {
    public let value: Any

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        if let x = try? container.decode(Int.self) {
            value = x
        } else if let x = try? container.decode(Double.self) {
            value = x
        } else if let x = try? container.decode(String.self) {
            value = x
        } else if let x = try? container.decode(Bool.self) {
            value = x
        } else if let x = try? container.decode([String: AnyDecodable].self) {
            value = x.mapValues { $0.value }
        } else if let x = try? container.decode([AnyDecodable].self) {
            value = x.map { $0.value }
        } else {
            throw DecodingError.dataCorruptedError(
                in: container, debugDescription: "AnyDecodable value cannot be decoded")
        }
    }

    public static func == (lhs: AnyDecodable, rhs: AnyDecodable) -> Bool {
        if let l = lhs.value as? Int, let r = rhs.value as? Int { return l == r }
        if let l = lhs.value as? Double, let r = rhs.value as? Double { return l == r }
        if let l = lhs.value as? String, let r = rhs.value as? String { return l == r }
        if let l = lhs.value as? Bool, let r = rhs.value as? Bool { return l == r }
        // Simplification for arrays/dicts: compare description or just false for complex types if needed
        // For MVP, if payload needs deep equality, we'd implementation recursive check.
        // As a fallback, string describing.
        return String(describing: lhs.value) == String(describing: rhs.value)
    }
}

public struct VoteCastPayload: Decodable, Sendable, Equatable {
    public let eventId: String
    public let trackId: String
    public let voterId: String
    public let totalVotes: Int
}

public struct TrackAddedPayload: Decodable, Sendable, Equatable {
    public let playlistId: String
    // Backend sends key "track" containing the Track object
    public let track: MusicRoomDomain.Track
}

public struct TrackDeletedPayload: Decodable, Sendable, Equatable {
    public let playlistId: String
    public let trackId: String
    public let position: Int
}

public struct TrackMovedPayload: Decodable, Sendable, Equatable {
    public let playlistId: String
    public let trackId: String
    public let from: Int
    public let to: Int
}

public struct PlaylistUpdatedPayload: Decodable, Sendable, Equatable {
    // Backend sends "playlist" object
    // We can decode minimally or fully depending on what we need.
    // For now, let's assuming we might want to refresh metadata.
    // However, the event payload structure from backend is:
    // payload: { "playlist": { ... } }
    // So we need a container or custom decoding if we want the nested object directly.
    // Let's mirror the backend structure:
    public let playlist: PlaylistMetadata
}

// Minimal playlist representation for updates
public struct PlaylistMetadata: Decodable, Sendable, Equatable {
    public let id: String
    public let name: String
    public let description: String
    public let isPublic: Bool
}

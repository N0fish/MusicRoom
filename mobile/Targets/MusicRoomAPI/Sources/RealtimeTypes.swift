import Foundation
import MusicRoomDomain

public struct RealtimeMessage: Decodable, Sendable, Equatable {
    public let type: String
    public let payload: AnyDecodable
}

// Wrapper to handle 'Any' decoding
public struct AnyDecodable: Codable, @unchecked Sendable, Equatable {
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

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        if let x = value as? Int {
            try container.encode(x)
        } else if let x = value as? Double {
            try container.encode(x)
        } else if let x = value as? String {
            try container.encode(x)
        } else if let x = value as? Bool {
            try container.encode(x)
        } else if let x = value as? [String: Any] {  // Map back to AnyDecodable for encoding recursion?
            // Since we have the value as Any, we need to wrap it if we want to use auto-encoding,
            // or manually encode dict.
            // Simplest is to cast to [String: AnyDecodable] if possible, or just build a wrapper.
            // But AnyDecodable structure is flat.
            // Let's iterate.
            let wrapped = x.mapValues { AnyDecodable($0) }
            try container.encode(wrapped)
        } else if let x = value as? [Any] {
            let wrapped = x.map { AnyDecodable($0) }
            try container.encode(wrapped)
        } else {
            throw EncodingError.invalidValue(
                value,
                EncodingError.Context(
                    codingPath: container.codingPath,
                    debugDescription: "AnyDecodable value cannot be encoded"))
        }
    }

    public init(_ value: Any) {
        self.value = value
    }

    public static func == (lhs: AnyDecodable, rhs: AnyDecodable) -> Bool {
        if let l = lhs.value as? Int, let r = rhs.value as? Int { return l == r }
        if let l = lhs.value as? Double, let r = rhs.value as? Double { return l == r }
        if let l = lhs.value as? String, let r = rhs.value as? String { return l == r }
        if let l = lhs.value as? Bool, let r = rhs.value as? Bool { return l == r }
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
    public let playlist: PlaylistMetadata
}

public struct PlaylistCreatedPayload: Decodable, Sendable, Equatable {
    public let playlist: PlaylistMetadata
}

public struct PlaylistMetadata: Decodable, Sendable, Equatable {
    public let id: String
    public let ownerId: String
    public let name: String
    public let description: String
    public let isPublic: Bool
}

public struct PlaylistInvitedPayload: Decodable, Sendable, Equatable {
    public let playlistId: String
    public let userId: String
}

public struct PlayerStateChangedPayload: Decodable, Sendable, Equatable {
    public let playlistId: String
    public let currentTrackId: String?
    public let playingStartedAt: Date?
    public let status: String
}

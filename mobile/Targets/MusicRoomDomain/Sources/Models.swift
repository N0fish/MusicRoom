import Foundation

public enum LicenseTier: String, Codable, CaseIterable, Sendable, Equatable {
    case everyone
    case invitedOnly
    case geoLocked

    public var label: String {
        switch self {
        case .everyone: return "Everyone"
        case .invitedOnly: return "Invited Only"
        case .geoLocked: return "Geo Locked"
        }
    }
}

public enum EventVisibility: String, Codable, Sendable, Equatable {
    case publicEvent
    case privateInvite

    public var label: String {
        switch self {
        case .publicEvent: return "Public"
        case .privateInvite: return "Private"
        }
    }
}

public struct Track: Identifiable, Equatable, Sendable {
    public let id: UUID
    public var title: String
    public var artist: String
    public var votes: Int

    public init(id: UUID = UUID(), title: String, artist: String, votes: Int) {
        self.id = id
        self.title = title
        self.artist = artist
        self.votes = votes
    }
}

public struct Event: Identifiable, Equatable, Sendable {
    public let id: UUID
    public var name: String
    public var location: String
    public var visibility: EventVisibility
    public var licenseTier: LicenseTier
    public var startTime: Date
    public var playlist: [Track]

    public init(
        id: UUID = UUID(),
        name: String,
        location: String,
        visibility: EventVisibility,
        licenseTier: LicenseTier,
        startTime: Date,
        playlist: [Track]
    ) {
        self.id = id
        self.name = name
        self.location = location
        self.visibility = visibility
        self.licenseTier = licenseTier
        self.startTime = startTime
        self.playlist = playlist
    }
}

public struct PlaylistUpdate: Equatable, Sendable {
    public let eventID: UUID
    public let updatedTrack: Track
    public let message: String

    public init(eventID: UUID, updatedTrack: Track, message: String) {
        self.eventID = eventID
        self.updatedTrack = updatedTrack
        self.message = message
    }
}

public struct PolicyDecision: Equatable, Sendable {
    public let isAllowed: Bool
    public let reason: String

    public init(isAllowed: Bool, reason: String) {
        self.isAllowed = isAllowed
        self.reason = reason
    }
}

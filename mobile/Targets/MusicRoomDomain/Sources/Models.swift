import Foundation

public enum EventLicenseMode: String, Codable, CaseIterable, Sendable, Equatable {
    case everyone
    case invitedOnly = "invited_only"
    case geoTime = "geo_time"

    public var label: String {
        switch self {
        case .everyone: return "Everyone"
        case .invitedOnly: return "Invited Only"
        case .geoTime: return "Geo + Time"
        }
    }
}

public enum EventVisibility: String, Codable, Sendable, Equatable {
    case publicEvent = "public"
    case privateEvent = "private"

    public var label: String {
        switch self {
        case .publicEvent: return "Public"
        case .privateEvent: return "Private"
        }
    }
}

public struct Track: Identifiable, Equatable, Sendable, Codable {
    public let id: String
    public var title: String
    public var artist: String
    public var provider: String
    public var providerTrackId: String
    public var thumbnailUrl: URL?
    public var votes: Int?  // Optional, filled by Tally if needed

    public init(
        id: String = UUID().uuidString,
        title: String,
        artist: String,
        provider: String,
        providerTrackId: String,
        thumbnailUrl: URL? = nil,
        votes: Int? = 0
    ) {
        self.id = id
        self.title = title
        self.artist = artist
        self.provider = provider
        self.providerTrackId = providerTrackId
        self.thumbnailUrl = thumbnailUrl
        self.votes = votes
    }
}

public struct Event: Identifiable, Equatable, Sendable, Codable {
    public let id: UUID
    public var name: String
    public var visibility: EventVisibility
    public var ownerId: String
    public var licenseMode: EventLicenseMode
    public var geoLat: Double?
    public var geoLng: Double?
    public var geoRadiusM: Int?
    public var voteStart: Date?
    public var voteEnd: Date?
    public var createdAt: Date
    public var updatedAt: Date

    public init(
        id: UUID,
        name: String,
        visibility: EventVisibility,
        ownerId: String,
        licenseMode: EventLicenseMode,
        geoLat: Double? = nil,
        geoLng: Double? = nil,
        geoRadiusM: Int? = nil,
        voteStart: Date? = nil,
        voteEnd: Date? = nil,
        createdAt: Date,
        updatedAt: Date
    ) {
        self.id = id
        self.name = name
        self.visibility = visibility
        self.ownerId = ownerId
        self.licenseMode = licenseMode
        self.geoLat = geoLat
        self.geoLng = geoLng
        self.geoRadiusM = geoRadiusM
        self.voteStart = voteStart
        self.voteEnd = voteEnd
        self.createdAt = createdAt
        self.updatedAt = updatedAt
    }
}

public struct CreateEventRequest: Codable, Sendable {
    public let name: String
    public let visibility: EventVisibility
    public let licenseMode: EventLicenseMode
    public let geoLat: Double?
    public let geoLng: Double?
    public let geoRadiusM: Int?
    public let voteStart: Date?
    public let voteEnd: Date?

    public init(
        name: String,
        visibility: EventVisibility = .publicEvent,
        licenseMode: EventLicenseMode = .everyone,
        geoLat: Double? = nil,
        geoLng: Double? = nil,
        geoRadiusM: Int? = nil,
        voteStart: Date? = nil,
        voteEnd: Date? = nil
    ) {
        self.name = name
        self.visibility = visibility
        self.licenseMode = licenseMode
        self.geoLat = geoLat
        self.geoLng = geoLng
        self.geoRadiusM = geoRadiusM
        self.voteStart = voteStart
        self.voteEnd = voteEnd
    }
}

public struct AddTrackRequest: Codable, Sendable {
    public let title: String
    public let artist: String
    public let provider: String
    public let providerTrackId: String
    public let thumbnailUrl: String

    public init(
        title: String,
        artist: String,
        provider: String,
        providerTrackId: String,
        thumbnailUrl: String
    ) {
        self.title = title
        self.artist = artist
        self.provider = provider
        self.providerTrackId = providerTrackId
        self.thumbnailUrl = thumbnailUrl
    }
}

public struct VoteRequest: Codable, Sendable {
    public let trackId: String
    public let lat: Double?
    public let lng: Double?

    public init(trackId: String, lat: Double? = nil, lng: Double? = nil) {
        self.trackId = trackId
        self.lat = lat
        self.lng = lng
    }
}

public struct VoteResponse: Codable, Sendable, Equatable {
    public let status: String
    public let trackId: String
    public let totalVotes: Int

    public init(status: String, trackId: String, totalVotes: Int) {
        self.status = status
        self.trackId = trackId
        self.totalVotes = totalVotes
    }
}

public struct MusicSearchItem: Codable, Sendable, Identifiable, Equatable {
    public var id: String { providerTrackId }
    public let title: String
    public let artist: String
    public let provider: String
    public let providerTrackId: String
    public let thumbnailUrl: URL?

    public init(
        title: String,
        artist: String,
        provider: String,
        providerTrackId: String,
        thumbnailUrl: URL? = nil
    ) {
        self.title = title
        self.artist = artist
        self.provider = provider
        self.providerTrackId = providerTrackId
        self.thumbnailUrl = thumbnailUrl
    }
}

public struct PolicyDecision: Sendable, Equatable {
    public let isAllowed: Bool
    public let reason: String

    public init(isAllowed: Bool, reason: String) {
        self.isAllowed = isAllowed
        self.reason = reason
    }
}

public struct PlaylistUpdate: Sendable, Equatable {
    public let eventID: UUID
    public let updatedTrack: Track
    public let message: String

    public init(eventID: UUID, updatedTrack: Track, message: String) {
        self.eventID = eventID
        self.updatedTrack = updatedTrack
        self.message = message
    }
}

public struct PlaylistResponse: Codable, Sendable, Equatable {
    // We can add the Playlist metadata struct if needed, but for now we might just want tracks
    // Backend returns { "playlist": ..., "tracks": ... }
    // Let's define a minimal Playlist struct inside or reuse if we had one.
    // We don't have a Playlist struct in Models.swift yet.

    public struct PlaylistMetadata: Codable, Sendable, Equatable {
        public let id: String
        public let ownerId: String
        public let name: String
        public let isPublic: Bool
        public let editMode: String

        public init(id: String, ownerId: String, name: String, isPublic: Bool, editMode: String) {
            self.id = id
            self.ownerId = ownerId
            self.name = name
            self.isPublic = isPublic
            self.editMode = editMode
        }
    }

    public let playlist: PlaylistMetadata
    public let tracks: [Track]

    public init(playlist: PlaylistMetadata, tracks: [Track]) {
        self.playlist = playlist
        self.tracks = tracks
    }
}

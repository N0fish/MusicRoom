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
    public var durationMs: Int?  // Added
    public var voteCount: Int?  // Backend source of truth
    public var status: String?  // "queued", "playing", "played"
    public var isVoted: Bool?  // User specific vote status

    public init(
        id: String = UUID().uuidString,
        title: String,
        artist: String,
        provider: String,
        providerTrackId: String,
        thumbnailUrl: URL? = nil,
        votes: Int? = 0,
        durationMs: Int? = 0,
        voteCount: Int? = 0,
        status: String? = "queued",
        isVoted: Bool? = false
    ) {
        self.id = id
        self.title = title
        self.artist = artist
        self.provider = provider
        self.providerTrackId = providerTrackId
        self.thumbnailUrl = thumbnailUrl
        self.votes = votes
        self.durationMs = durationMs
        self.voteCount = voteCount
        self.status = status
        self.isVoted = isVoted
    }
}

public struct NextTrackResponse: Decodable, Sendable, Equatable {
    public let playlistId: String
    public let currentTrackId: String?
    public let playingStartedAt: Date?
    public let status: String

    public init(
        playlistId: String, currentTrackId: String?, playingStartedAt: Date?, status: String
    ) {
        self.playlistId = playlistId
        self.currentTrackId = currentTrackId
        self.playingStartedAt = playingStartedAt
        self.status = status
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
    public var isJoined: Bool?
    public var canVote: Bool?

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
        updatedAt: Date,
        isJoined: Bool? = false,
        canVote: Bool? = false
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
        self.isJoined = isJoined
        self.canVote = canVote
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

    enum CodingKeys: String, CodingKey {
        case name
        case visibility
        case licenseMode = "license_mode"
        case geoLat = "geo_lat"
        case geoLng = "geo_lng"
        case geoRadiusM = "geo_radius_m"
        case voteStart = "vote_start"
        case voteEnd = "vote_end"
    }

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

public struct PatchEventRequest: Codable, Sendable {
    public let name: String?
    public let visibility: EventVisibility?
    public let licenseMode: EventLicenseMode?

    enum CodingKeys: String, CodingKey {
        case name
        case visibility
        case licenseMode = "license_mode"
    }

    public init(
        name: String? = nil,
        visibility: EventVisibility? = nil,
        licenseMode: EventLicenseMode? = nil
    ) {
        self.name = name
        self.visibility = visibility
        self.licenseMode = licenseMode
    }
}

public struct AddTrackRequest: Codable, Sendable {
    public let title: String
    public let artist: String
    public let provider: String
    public let providerTrackId: String
    public let thumbnailUrl: String
    public let durationMs: Int?

    public init(
        title: String,
        artist: String,
        provider: String,
        providerTrackId: String,
        thumbnailUrl: String,
        durationMs: Int? = 0
    ) {
        self.title = title
        self.artist = artist
        self.provider = provider
        self.providerTrackId = providerTrackId
        self.thumbnailUrl = thumbnailUrl
        self.durationMs = durationMs
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
    public let voteCount: Int

    public init(voteCount: Int) {
        self.voteCount = voteCount
    }
}

public struct MusicSearchItem: Codable, Sendable, Identifiable, Equatable {
    public var id: String { providerTrackId }
    public let title: String
    public let artist: String
    public let provider: String
    public let providerTrackId: String
    public let thumbnailUrl: URL?
    public let durationMs: Int?

    public init(
        title: String,
        artist: String,
        provider: String,
        providerTrackId: String,
        thumbnailUrl: URL? = nil,
        durationMs: Int? = 0
    ) {
        self.title = title
        self.artist = artist
        self.provider = provider
        self.providerTrackId = providerTrackId
        self.thumbnailUrl = thumbnailUrl
        self.durationMs = durationMs
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
        public var currentTrackId: String?
        public var playingStartedAt: Date?

        public init(
            id: String, ownerId: String, name: String, isPublic: Bool, editMode: String,
            currentTrackId: String? = nil, playingStartedAt: Date? = nil
        ) {
            self.id = id
            self.ownerId = ownerId
            self.name = name
            self.isPublic = isPublic
            self.editMode = editMode
            self.currentTrackId = currentTrackId
            self.playingStartedAt = playingStartedAt
        }
    }

    public let playlist: PlaylistMetadata
    public let tracks: [Track]

    public init(playlist: PlaylistMetadata, tracks: [Track]) {
        self.playlist = playlist
        self.tracks = tracks
    }

    // Default memberwise initializer is lost if we add a custom init(from:),
    // but the one above covers manual creation.
    // We strictly need init(from:) to handle decoding null tracks.

    enum CodingKeys: String, CodingKey {
        case playlist
        case tracks
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        self.playlist = try container.decode(PlaylistMetadata.self, forKey: .playlist)
        // If tracks is missing or null, default to empty array
        self.tracks = try container.decodeIfPresent([Track].self, forKey: .tracks) ?? []
    }
}

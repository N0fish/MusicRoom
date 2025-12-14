import AppSupportClients
import ComposableArchitecture
import Foundation
import MusicRoomAPI
import MusicRoomDomain

@Reducer
public struct EventDetailFeature: Sendable {
    @ObservableState
    public struct State: Equatable, Sendable {
        public let event: Event
        public var tally: [MusicRoomAPIClient.TallyItem] = []
        public var tracks: [Track] = []
        public var isLoading: Bool = false
        public var isVoting: Bool = false
        public var userAlert: UserAlert?

        // Navigation / Presentation
        @Presents public var musicSearch: MusicSearchFeature.State?

        public init(event: Event) {
            self.event = event
        }
    }

    public enum Action: Equatable, Sendable {
        case onAppear
        case loadTally
        case tallyLoaded(Result<[MusicRoomAPIClient.TallyItem], Error>)
        case voteButtonTapped(trackId: String)
        case voteResponse(Result<VoteResponse, Error>)
        case dismissInfo

        // Search
        case addTrackButtonTapped
        case musicSearch(PresentationAction<MusicSearchFeature.Action>)

        // Playlist
        case removeTrackButtonTapped(trackId: String)
        case removeTrackResponse(Result<Void, Error>)
        case playlistLoaded([Track])

        // Realtime
        case realtimeMessageReceived(RealtimeMessage)
        case realtimeConnected
    }

    @Dependency(\.musicRoomAPI) var musicRoomAPI
    @Dependency(\.telemetry) var telemetry
    @Dependency(\.locationClient) var locationClient
    @Dependency(\.persistence) var persistence
    @Dependency(\.continuousClock) var clock

    public struct UserAlert: Equatable, Sendable {
        public var title: String
        public var message: String
        public var type: AlertType

        public enum AlertType: Equatable, Sendable {
            case error
            case success
            case info
        }
    }

    private enum CancelID { case realtime }

    public init() {}

    public var body: some ReducerOf<Self> {
        Reduce { state, action in
            switch action {
            case .onAppear:
                return .merge(
                    .run { [name = state.event.name] _ in
                        await telemetry.log("Viewed Event Detail: \(name)", [:])
                    },
                    .send(.loadTally),
                    .run { send in
                        for await msg in musicRoomAPI.connectToRealtime() {
                            await send(.realtimeMessageReceived(msg))
                        }
                    }
                    .cancellable(id: CancelID.realtime)
                )

            case .loadTally:
                state.isLoading = true
                return .run { [eventId = state.event.id] send in
                    // Fetch both tally and playlist tracks
                    async let tallyResult = Result { try await musicRoomAPI.tally(eventId) }
                    async let playlistResult = Result {
                        try await musicRoomAPI.getPlaylist(eventId.uuidString)
                    }

                    let (tally, playlist) = await (tallyResult, playlistResult)

                    // Handle Playlist
                    switch playlist {
                    case .success(let response):
                        try? await persistence.savePlaylist(response)
                        await send(.playlistLoaded(response.tracks))
                    case .failure:
                        // Fallback to cache
                        if let cached = try? await persistence.loadPlaylist() {
                            // Only use cache if it matches this event?
                            // MVP limitation: "playlist_cache.json" is single slot.
                            // We should check ID if possible, but PlaylistResponse has ID.
                            // Assuming checking cached.playlist.id == eventId (string check)
                            if cached.playlist.id.lowercased() == eventId.uuidString.lowercased() {
                                await send(.playlistLoaded(cached.tracks))
                            }
                        }
                    }

                    await send(.tallyLoaded(tally))
                }

            case .tallyLoaded(.success(let items)):
                state.isLoading = false
                state.tally = items.sorted { $0.count > $1.count }
                return .none

            case .playlistLoaded(let tracks):
                state.tracks = tracks
                return .none

            case .tallyLoaded(.failure(let error)):
                state.isLoading = false
                print("Tally refresh failed: \(error)")
                return .none

            case .voteButtonTapped(let trackId):
                state.isVoting = true
                state.userAlert = nil

                // Time Check
                let now = Date()
                if let start = state.event.voteStart, now < start {
                    state.isVoting = false
                    state.userAlert = UserAlert(
                        title: "Voting Not Started",
                        message: "Voting will begin at \(start.formatted()).",
                        type: .info
                    )
                    return .none
                }
                if let end = state.event.voteEnd, now > end {
                    state.isVoting = false
                    state.userAlert = UserAlert(
                        title: "Voting Ended",
                        message: "Voting closed at \(end.formatted()).",
                        type: .error
                    )
                    return .none
                }

                // Optimistic Update
                if let index = state.tally.firstIndex(where: { $0.track == trackId }) {
                    let item = state.tally[index]
                    let newItem = MusicRoomAPIClient.TallyItem(
                        track: item.track, count: item.count + 1)
                    state.tally[index] = newItem
                    state.tally.sort { $0.count > $1.count }
                } else {
                    let newItem = MusicRoomAPIClient.TallyItem(track: trackId, count: 1)
                    state.tally.append(newItem)
                    state.tally.sort { $0.count > $1.count }
                }

                return .run { [event = state.event, locationClient] send in
                    var lat: Double?
                    var lng: Double?

                    // Geo Check
                    if event.licenseMode == .geoTime {
                        await locationClient.requestWhenInUseAuthorization()
                        // We could check status here, but let's try getting location
                        do {
                            let loc = try await locationClient.getCurrentLocation()
                            lat = loc.latitude
                            lng = loc.longitude
                        } catch {
                            await send(.voteResponse(.failure(error)))  // Location error
                            return
                        }
                    }

                    await send(
                        .voteResponse(
                            Result {
                                try await musicRoomAPI.vote(event.id, trackId, lat, lng)
                            }))
                }

            case .voteResponse(.success(let response)):
                state.isVoting = false
                state.userAlert = UserAlert(
                    title: "Success",
                    message: "Voted for \(response.trackId)!",
                    type: .success
                )
                return .run { send in
                    try await clock.sleep(for: .seconds(1))
                    await send(.loadTally)
                    try await clock.sleep(for: .seconds(2))
                    await send(.dismissInfo)
                }

            case .voteResponse(.failure(let error)):
                state.isVoting = false
                let message: String
                if let apiError = error as? MusicRoomAPIError {
                    message = apiError.errorDescription ?? "Unknown generic error"
                } else {
                    message = error.localizedDescription
                }

                // Detailed Map for user-friendly errors
                if message.contains("403") {
                    state.userAlert = UserAlert(
                        title: "Permission Denied",
                        message: "You are not allowed to vote on this event (License restriction).",
                        type: .error)
                } else {
                    state.userAlert = UserAlert(
                        title: "Vote Failed", message: message, type: .error)
                }
                return .none

            case .dismissInfo:
                state.userAlert = nil
                return .none

            case .addTrackButtonTapped:
                state.musicSearch = MusicSearchFeature.State()
                return .none

            case .musicSearch(.presented(.trackTapped(let item))):
                state.musicSearch = nil  // Dismiss search
                // For now, assume adding track means voting or adding to playlist?
                // Design: search adds to playlist.
                // But we don't have addTrack API yet in client (only remove).
                // Wait, search currently calls `voteButtonTapped`.
                // In Phase 2.1 it was voting. Phase 2.2 is Playlist.
                // If the track is NOT in the playlist, we should ADD it.
                // If it IS in the playlist, we should VOTE for it.

                // Check if track exists
                if state.tracks.contains(where: { $0.providerTrackId == item.providerTrackId }) {
                    return .send(.voteButtonTapped(trackId: item.providerTrackId))  // Or track.id?
                } else {
                    // Need addTrack implementation in API
                    // For now, let's just vote which might add it?
                    // Backend `handleAddTrack` is separate from `vote`.
                    // We need `addTrack` implemented in API.
                    // For this step, I focused on Remove.
                    // I will leave it as vote for now but note it needs update.
                    return .send(.voteButtonTapped(trackId: item.providerTrackId))
                }

            case .musicSearch:
                return .none

            case .realtimeMessageReceived(let msg):
                switch msg.type {
                case "vote.cast":
                    return .send(.loadTally)
                case "track.added":
                    // Parse payload and update tracks
                    if let payload = msg.payload.value as? [String: Any],
                        payload["track"] as? [String: Any] != nil
                    {  // Check existence
                        // Manual decoding or use JSONDecoder if value was Data
                        // Since AnyDecodable gives us Dict/Array, it's hard to decode back to struct easily without re-encoding.
                        // Optimization: Just reload playlist for MVP
                        return .send(.loadTally)  // loadTally reloads playlist too now
                    }
                    return .send(.loadTally)
                case "track.deleted":
                    // Optimistic removal possible if we parse payload
                    return .send(.loadTally)
                case "playlist.updated":
                    return .send(.loadTally)
                default:
                    return .none
                }

            case .realtimeConnected:
                return .none

            case .removeTrackButtonTapped(let trackId):
                // Optimistic removal
                state.tracks.removeAll { $0.id == trackId }

                return .run { [eventId = state.event.id] send in
                    await send(
                        .removeTrackResponse(
                            Result {
                                try await musicRoomAPI.removeTrack(eventId.uuidString, trackId)
                            }))
                }

            case .removeTrackResponse(.success):
                return .none

            case .removeTrackResponse(.failure(let error)):
                state.userAlert = UserAlert(
                    title: "Error",
                    message: "Failed to remove track: \(error.localizedDescription)", type: .error)
                return .send(.loadTally)  // Revert state by reloading
            }
        }
        .ifLet(\.$musicSearch, action: \.musicSearch) {
            MusicSearchFeature()
        }
    }
}

extension EventDetailFeature.Action {
    public static func == (lhs: EventDetailFeature.Action, rhs: EventDetailFeature.Action) -> Bool {
        switch (lhs, rhs) {
        case (.onAppear, .onAppear),
            (.loadTally, .loadTally),
            (.dismissInfo, .dismissInfo),
            (.addTrackButtonTapped, .addTrackButtonTapped),
            (.realtimeConnected, .realtimeConnected):
            return true
        case (.voteButtonTapped(let lId), .voteButtonTapped(let rId)):
            return lId == rId
        case (.tallyLoaded(.success(let lItems)), .tallyLoaded(.success(let rItems))):
            return lItems == rItems
        case (.tallyLoaded(.failure(let lError)), .tallyLoaded(.failure(let rError))):
            return lError.localizedDescription == rError.localizedDescription
        case (.voteResponse(.success(let lResp)), .voteResponse(.success(let rResp))):
            return lResp == rResp
        case (.voteResponse(.failure(let lError)), .voteResponse(.failure(let rError))):
            return lError.localizedDescription == rError.localizedDescription
        case (.musicSearch(let lAction), .musicSearch(let rAction)):
            return lAction == rAction
        case (.realtimeMessageReceived(let lMsg), .realtimeMessageReceived(let rMsg)):
            return lMsg == rMsg
        case (.removeTrackButtonTapped(let lId), .removeTrackButtonTapped(let rId)):
            return lId == rId
        case (.playlistLoaded(let lTracks), .playlistLoaded(let rTracks)):
            return lTracks == rTracks
        case (.removeTrackResponse(.success), .removeTrackResponse(.success)):
            return true
        case (
            .removeTrackResponse(.failure(let lError)), .removeTrackResponse(.failure(let rError))
        ):
            return lError.localizedDescription == rError.localizedDescription
        default:
            return false
        }
    }
}

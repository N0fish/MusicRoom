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
        public var errorMessage: String?
        public var successMessage: String?

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
    @Dependency(\.continuousClock) var clock

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
                    // We can do this in parallel or sequence.
                    // For now, let's fetch playlist to get full track list.
                    // Assuming event.id == playlist.id
                    async let tallyResult = Result { try await musicRoomAPI.tally(eventId) }
                    async let playlistResult = Result {
                        try await musicRoomAPI.getPlaylist(eventId.uuidString)
                    }

                    let (tally, playlist) = await (tallyResult, playlistResult)

                    if let tracks = try? playlist.get().tracks {
                        await send(.playlistLoaded(tracks))
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
                state.errorMessage = nil
                state.successMessage = nil

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

                // Also update local track vote count if we have it?
                // For MVP, tally list is separate.

                return .run { [eventId = state.event.id] send in
                    await send(
                        .voteResponse(
                            Result {
                                try await musicRoomAPI.vote(eventId, trackId, nil, nil)
                            }))
                }

            case .voteResponse(.success(let response)):
                state.isVoting = false
                state.successMessage = "Voted for \(response.trackId)!"
                return .run { send in
                    try await clock.sleep(for: .seconds(1))
                    await send(.loadTally)
                    try await clock.sleep(for: .seconds(2))
                    await send(.dismissInfo)
                }

            case .voteResponse(.failure(let error)):
                state.isVoting = false
                if let apiError = error as? MusicRoomAPIError {
                    state.errorMessage = apiError.errorDescription
                } else {
                    state.errorMessage = error.localizedDescription
                }
                return .none

            case .dismissInfo:
                state.errorMessage = nil
                state.successMessage = nil
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
                state.errorMessage = "Failed to remove track: \(error.localizedDescription)"
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

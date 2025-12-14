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
                    await send(
                        .tallyLoaded(
                            Result {
                                try await musicRoomAPI.tally(eventId)
                            }))
                }

            case .tallyLoaded(.success(let items)):
                state.isLoading = false
                state.tally = items.sorted { $0.count > $1.count }
                return .none

            case .tallyLoaded(.failure(let error)):
                state.isLoading = false
                // Silence tally errors for now or show unobtrusive alert
                print("Tally refresh failed: \(error)")
                return .none

            case .voteButtonTapped(let trackId):
                state.isVoting = true
                state.errorMessage = nil
                state.successMessage = nil

                // Optimistic Update
                if let index = state.tally.firstIndex(where: { $0.track == trackId }) {
                    let item = state.tally[index]
                    // Assume TallyItem is immutable property, so recreate
                    let newItem = MusicRoomAPIClient.TallyItem(
                        track: item.track, count: item.count + 1)
                    state.tally[index] = newItem
                    // Re-sort
                    state.tally.sort { $0.count > $1.count }
                } else {
                    // New item
                    let newItem = MusicRoomAPIClient.TallyItem(track: trackId, count: 1)
                    state.tally.append(newItem)
                    state.tally.sort { $0.count > $1.count }
                }

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

                // Re-fetch tally to ensure consistency after short delay
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
                return .send(.voteButtonTapped(trackId: item.providerTrackId))

            case .musicSearch:
                return .none

            case .realtimeMessageReceived(let msg):
                // Handle vote.cast
                if msg.type == "vote.cast" {
                    // Start simple: just reload tally to ensure consistency
                    // Optimization: Parse payload and update local array
                    return .send(.loadTally)
                }
                return .none

            case .realtimeConnected:
                return .none
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
        default:
            return false
        }
    }
}

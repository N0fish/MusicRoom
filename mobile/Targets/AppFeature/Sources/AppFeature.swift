import AppSupportClients
import ComposableArchitecture
import MusicRoomAPI
import MusicRoomDomain
import PolicyEngine
import RealtimeMocks
import SettingsFeature

@Reducer
public struct AppFeature {
    public struct State: Equatable {
        public var settings: SettingsFeature.State
        public var isSampleDataLoading: Bool
        public var sampleEvents: [Event]
        public var sampleDataError: String?
        public var policySummary: String
        public var latestStreamMessage: String
        public var hasBootstrapped: Bool

        public init(
            settings: SettingsFeature.State = SettingsFeature.State(),
            isSampleDataLoading: Bool = false,
            sampleEvents: [Event] = [],
            sampleDataError: String? = nil,
            policySummary: String = "Policy not evaluated",
            latestStreamMessage: String = "Stream idle",
            hasBootstrapped: Bool = false
        ) {
            self.settings = settings
            self.isSampleDataLoading = isSampleDataLoading
            self.sampleEvents = sampleEvents
            self.sampleDataError = sampleDataError
            self.policySummary = policySummary
            self.latestStreamMessage = latestStreamMessage
            self.hasBootstrapped = hasBootstrapped
        }
    }

    public enum Action: Equatable {
        case settings(SettingsFeature.Action)
        case task
        case sampleEventsLoaded([Event])
        case sampleEventsFailed(String)
        case policyEvaluated(PolicyDecision)
        case playlistUpdate(PlaylistUpdate)
        case playlistStreamCompleted
    }

    @Dependency(\.musicRoomAPI) var musicRoomAPI
    @Dependency(\.policyEngine) var policyEngine
    @Dependency(\.playlistStream) var playlistStream
    @Dependency(\.telemetry) var telemetry

    public init() {}

    public var body: some ReducerOf<Self> {
        Scope(state: \.settings, action: \.settings) {
            SettingsFeature()
        }
        Reduce { state, action in
            switch action {
            case .settings:
                return .none

            case .task:
                guard !state.hasBootstrapped else { return .none }
                state.hasBootstrapped = true
                state.isSampleDataLoading = true
                state.sampleDataError = nil
                return .run {
                    [
                        musicRoomAPI = self.musicRoomAPI, policyEngine = self.policyEngine,
                        playlistStream = self.playlistStream, telemetry = self.telemetry
                    ] send in
                    await telemetry.log("App Started", ["Platform": "iOS"])
                    do {
                        let events = try await musicRoomAPI.fetchSampleEvents()
                        await send(.sampleEventsLoaded(events))
                        guard let first = events.first else { return }
                        let decision = await policyEngine.evaluate(first)
                        await send(.policyEvaluated(decision))
                        for await update in playlistStream.startPreview(first) {
                            await send(.playlistUpdate(update))
                        }
                        await send(.playlistStreamCompleted)
                    } catch {
                        await telemetry.log(
                            "Sample Data Load Failed", ["Error": error.localizedDescription])
                        await send(.sampleEventsFailed(error.localizedDescription))
                    }
                }

            case .sampleEventsLoaded(let events):
                state.isSampleDataLoading = false
                state.sampleEvents = events
                state.sampleDataError = nil
                return .none

            case .sampleEventsFailed(let message):
                state.isSampleDataLoading = false
                state.sampleDataError = message
                state.sampleEvents = []
                return .none

            case .policyEvaluated(let decision):
                state.policySummary =
                    decision.isAllowed
                    ? "Allowed – \(decision.reason)" : "Blocked – \(decision.reason)"
                return .none

            case .playlistUpdate(let update):
                state.latestStreamMessage = update.message
                return .none

            case .playlistStreamCompleted:
                state.latestStreamMessage = "Stream completed"
                return .none
            }
        }
    }
}

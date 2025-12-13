import AppSupportClients
import AuthenticationFeature
import ComposableArchitecture
import Foundation
import MusicRoomAPI
import MusicRoomDomain
import PolicyEngine
import RealtimeMocks
import SettingsFeature

@Reducer
public struct AppFeature: Sendable {
    public struct State: Equatable {
        public enum Destination: Equatable {
            case login
            case app
        }
        public var destination: Destination = .login
        public var authentication = AuthenticationFeature.State()
        public var settings: SettingsFeature.State
        public var isSampleDataLoading: Bool
        public var sampleEvents: [Event]
        public var sampleDataError: String?
        public var policySummary: String
        public var latestStreamMessage: String
        public var hasBootstrapped: Bool

        public init() {
            self.settings = SettingsFeature.State(
                backendURLText: "http://localhost:8080",
                savedBackendURL: URL(string: "http://localhost:8080"),
                selectedPreset: .local,
                diagnosticsSummary: DiagnosticsSummary(
                    testedURL: URL(string: "http://localhost:8080")!,
                    status: .reachable,
                    latencyMs: 0,
                    measuredAt: Date()
                ),
                metadata: AppMetadata(
                    version: "1.0.0",
                    build: "1",
                    deviceModel: "Unknown",
                    systemVersion: "Unknown"
                )
            )
            self.isSampleDataLoading = false
            self.sampleEvents = []
            self.policySummary = "Initializing..."
            self.latestStreamMessage = "Waiting for stream..."
            self.hasBootstrapped = false
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
        case authentication(AuthenticationFeature.Action)
        case destinationChanged(State.Destination)
        case startApp
        case logoutButtonTapped
    }

    @Dependency(\.musicRoomAPI) var musicRoomAPI
    @Dependency(\.policyEngine) var policyEngine
    @Dependency(\.playlistStream) var playlistStream
    @Dependency(\.telemetry) var telemetry
    @Dependency(\.authentication) var authentication

    public init() {}

    public var body: some ReducerOf<Self> {
        Scope(state: \.settings, action: \.settings) {
            SettingsFeature()
        }

        Scope(state: \.authentication, action: \.authentication) {
            AuthenticationFeature()
        }

        Reduce { state, action in
            switch action {
            case .settings:
                return .none

            case .authentication(.authResponse(.success)):
                state.destination = .app
                return .send(.startApp)

            case .authentication:
                return .none

            case .task:
                return .run { [authentication = self.authentication] send in
                    if authentication.isAuthenticated() {
                        await send(.destinationChanged(.app))
                        await send(.startApp)
                    } else {
                        await send(.destinationChanged(.login))
                    }
                }

            case .destinationChanged(let destination):
                state.destination = destination
                return .none

            case .startApp:
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

            case .logoutButtonTapped:
                return .run { [authentication = self.authentication] send in
                    await authentication.logout()
                    await send(.destinationChanged(.login))
                }
            }
        }
    }
}

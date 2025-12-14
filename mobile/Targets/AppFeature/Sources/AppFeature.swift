import AppSupportClients
import AuthenticationFeature
import ComposableArchitecture
import EventFeature
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
        public var eventList = EventListFeature.State()

        // Legacy/Stream State (To be refactored into EventDetail later)
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
            self.latestStreamMessage = "Waiting for stream..."
            self.hasBootstrapped = false
        }
    }

    public enum Action: Equatable {
        case settings(SettingsFeature.Action)
        case authentication(AuthenticationFeature.Action)
        case eventList(EventListFeature.Action)
        case task
        case destinationChanged(State.Destination)
        case startApp
        case logoutButtonTapped
        case handleDeepLink(URL)
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

        Scope(state: \.eventList, action: \.eventList) {
            EventListFeature()
        }

        Reduce { state, action in
            switch action {
            case .settings:
                return .none

            case .eventList:
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

                // Trigger initial data load
                return .run { send in
                    await send(.eventList(.onAppear))
                }

            case .handleDeepLink(_):
                // Legacy: ASWebAuthenticationSession handles callbacks internally for Social Auth.
                // Keep this if we need to handle *other* deep links (e.g. Email Links)
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

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
            case splash
        }
        public var destination: Destination = .splash
        public var authentication = AuthenticationFeature.State()
        public var settings: SettingsFeature.State
        public var eventList = EventListFeature.State()
        public var profile = ProfileFeature.State()

        // Legacy/Stream State (To be refactored into EventDetail later)
        public var latestStreamMessage: String
        public var hasBootstrapped: Bool
        public var isOffline: Bool = false

        public init() {
            self.settings = SettingsFeature.State(
                backendURLText: "http://localhost:8080",
                savedBackendURL: URL(string: "http://localhost:8080"),
                selectedPreset: .local,
                diagnosticsSummary: DiagnosticsSummary(
                    testedURL: URL(string: "http://localhost:8080")!,
                    status: .reachable,
                    latencyMs: 0,
                    wsStatus: .reachable,
                    wsLatencyMs: 0,
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
        case profile(ProfileFeature.Action)
        case task
        case destinationChanged(State.Destination)
        case startApp
        case logoutButtonTapped
        case handleDeepLink(URL)
        case networkStatusChanged(NetworkStatus)
    }

    @Dependency(\.musicRoomAPI) var musicRoomAPI
    @Dependency(\.policyEngine) var policyEngine
    @Dependency(\.playlistStream) var playlistStream
    @Dependency(\.telemetry) var telemetry
    @Dependency(\.authentication) var authentication
    @Dependency(\.networkMonitor) var networkMonitor

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

        Scope(state: \.profile, action: \.profile) {
            ProfileFeature()
        }

        Reduce { state, action in
            switch action {
            case .settings:
                return .none

            case .eventList(.delegate(.sessionExpired)):
                return .send(.logoutButtonTapped)

            case .eventList:
                return .none

            case .profile(.logoutButtonTapped):
                // ProfileFeature handles the API call, we just need to switch destination if needed.
                // Actually ProfileFeature.logoutButtonTapped runs an effect to logout.
                // We also need to switch navigation.
                return .run { send in
                    await send(.destinationChanged(.login))
                }

            case .profile:
                return .none

            case .authentication(.authResponse(.success)):
                state.destination = .app
                return .send(.startApp)

            case .authentication:
                return .none

            case .task:
                return .run { [authentication = self.authentication] send in
                    // Show Splash for at least 2 seconds
                    try? await Task.sleep(for: .seconds(2))

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
                return .run { [telemetry, networkMonitor] send in
                    await telemetry.log(
                        "app.launch",
                        [
                            "version": Bundle.main.infoDictionary?["CFBundleShortVersionString"]
                                as? String ?? "unknown"
                        ])

                    // Start Network Monitor
                    for await status in networkMonitor.start() {
                        await send(.networkStatusChanged(status))
                    }
                }

            case .networkStatusChanged(let status):
                let isOffline = (status == .unsatisfied || status == .requiresConnection)
                state.isOffline = isOffline

                // Propagate to Profile (State update)
                state.profile.isOffline = isOffline

                // Propagate to EventList (Action for logic + State update handled by action potentially,
                // but we can set state here too to be sure, or let the action do it.
                // EventList.networkStatusChanged sets the state, so sending action is enough for the root list.)
                // However, we also need to update the Navigation Stack path which EventList owns.
                // We can iterate the stack here.
                for id in state.eventList.path.ids {
                    state.eventList.path[id: id]?.isOffline = isOffline
                }

                return .send(.eventList(.networkStatusChanged(status)))

            case .handleDeepLink(_):
                // Legacy: ASWebAuthenticationSession handles callbacks internally for Social Auth.
                return .none

            case .logoutButtonTapped:
                return .run { [authentication = self.authentication, telemetry] send in
                    await telemetry.log("user.logout", [:])
                    await authentication.logout()
                    await send(.destinationChanged(.login))
                }
            }
        }
    }
}

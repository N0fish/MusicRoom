import AppSettingsClient
import AppSupportClients
import AuthenticationFeature
import ComposableArchitecture
import EventFeature
import Foundation
import MusicRoomAPI
import MusicRoomDomain
import PlaylistFeature
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
        public var friends = FriendsFeature.State()
        public var playlistList = PlaylistListFeature.State()

        // Legacy/Stream State (To be refactored into EventDetail later)
        public var latestStreamMessage: String
        public var hasBootstrapped: Bool
        public var isOffline: Bool = false

        public init() {
            let defaultSettings = AppSettings.default
            self.settings = SettingsFeature.State(
                backendURLText: defaultSettings.backendURL.absoluteString,
                savedBackendURL: defaultSettings.backendURL,
                selectedPreset: defaultSettings.selectedPreset,
                lastLocalURLText: defaultSettings.localURL.absoluteString,
                lastHostedURLText: defaultSettings.hostedURL.absoluteString,
                diagnosticsSummary: DiagnosticsSummary(
                    testedURL: defaultSettings.backendURL,
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
        case friends(FriendsFeature.Action)
        case playlistList(PlaylistListFeature.Action)
        case task
        case destinationChanged(State.Destination)
        case startApp
        case logoutButtonTapped
        case handleDeepLink(URL)
        case networkStatusChanged(NetworkStatus)
        case checkInitialLoad
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

        Scope(state: \.friends, action: \.friends) {
            FriendsFeature()
        }

        Scope(state: \.playlistList, action: \.playlistList) {
            PlaylistListFeature()
        }

        Reduce { state, action in
            switch action {
            case .settings:
                return .none

            case .eventList(.delegate(.sessionExpired)):
                return .send(.logoutButtonTapped)

            case .eventList(.eventsLoaded), .eventList(.eventsLoadedFromCache):
                return .send(.checkInitialLoad)

            case .eventList:
                return .none

            case .profile(.logoutButtonTapped):
                // ProfileFeature handles the API call, we just need to switch destination if needed.
                // Actually ProfileFeature.logoutButtonTapped runs an effect to logout.
                // We also need to switch navigation.
                return .run { send in
                    await send(.destinationChanged(.login))
                }

            case .profile(.profileResponse):
                return .send(.checkInitialLoad)

            case .profile:
                return .none

            case .friends(.friendsLoaded), .friends(.requestsLoaded):
                return .send(.checkInitialLoad)

            case .friends:
                return .none

            case .playlistList:
                return .none

            case .authentication(.authResponse(.success)):
                state.destination = .app
                return .send(.startApp)

            case .authentication:
                return .none

            case .task:
                return .run { [authentication = self.authentication] send in
                    if authentication.isAuthenticated() {
                        // Trigger initial loads
                        await send(.eventList(.onAppear))
                        await send(.profile(.onAppear))
                        await send(.friends(.onAppear))
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

            case .checkInitialLoad:
                // We transition to .app only when all critical data is loaded
                if state.eventList.hasLoaded && state.profile.hasLoaded && state.friends.hasLoaded {
                    state.destination = .app
                }
                return .none

            case .handleDeepLink(_):
                // Legacy: ASWebAuthenticationSession handles callbacks internally for Social Auth.
                return .none

            case .logoutButtonTapped:
                return .run {
                    [
                        userId = state.eventList.currentUserId,
                        authentication = self.authentication, telemetry
                    ] send in
                    await telemetry.log("user.logout", userId.map { ["userId": $0] } ?? [:])
                    await authentication.logout()
                    await send(.destinationChanged(.login))
                }
            }
        }
    }
}

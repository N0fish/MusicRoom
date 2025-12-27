import ComposableArchitecture
import XCTest

@testable import AppFeature
@testable import AppSettingsClient
@testable import AppSupportClients
@testable import EventFeature
@testable import MusicRoomDomain
@testable import SettingsFeature

@MainActor
final class AppFeatureTests: XCTestCase {
    func testSessionExpired_TriggersLogout() async {
        let logoutCalled = LockIsolated(false)

        let store = TestStore(initialState: AppFeature.State()) {
            AppFeature()
        } withDependencies: {
            $0.authentication.logout = { logoutCalled.setValue(true) }
            $0.authentication.isAuthenticated = { true }
            $0.telemetry.log = { action, _ in
                XCTAssertEqual(action, "user.logout")
            }
        }
        store.exhaustivity = .off

        // Set initial state to logged in (app)
        await store.send(.destinationChanged(.app)) {
            $0.destination = .app
        }

        // Simulate event list delegate action
        await store.send(.eventList(.delegate(.sessionExpired)))

        // Verifying it triggers logout button tapped logic
        await store.receive(\.logoutButtonTapped)

        // Should trigger logout side effect
        XCTAssertTrue(logoutCalled.value)

        // And navigate to login
        await store.receive(\.destinationChanged) {
            $0.destination = .login
        }
    }


    func testDestinationChangedToLoginResetsStatePreservingSettings() async {
        let customURL = URL(string: "https://custom.musicroom.app")!
        var initialState = AppFeature.State()
        initialState.destination = .app
        initialState.eventList.hasLoaded = true
        initialState.profile.hasLoaded = true
        initialState.friends.hasLoaded = true
        initialState.settings = SettingsFeature.State(
            backendURLText: customURL.absoluteString,
            savedBackendURL: customURL,
            selectedPreset: .hosted,
            lastLocalURLText: BackendEnvironmentPreset.local.defaultURL.absoluteString,
            lastHostedURLText: customURL.absoluteString
        )

        let store = TestStore(initialState: initialState) {
            AppFeature()
        }

        await store.send(.destinationChanged(.login)) {
            var reset = AppFeature.State()
            reset.settings = initialState.settings
            reset.destination = .login
            $0 = reset
        }
    }
}

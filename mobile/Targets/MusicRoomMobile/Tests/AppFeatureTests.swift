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
                XCTAssertEqual(action, "user.session.expired")
            }
        }
        store.exhaustivity = .off

        // Set initial state to logged in (app)
        await store.send(.destinationChanged(.app)) {
            $0.destination = .app
        }

        // Simulate event list delegate action
        await store.send(.eventList(.delegate(.sessionExpired)))

        await store.receive(.sessionEvent(.expired)) {
            var reset = AppFeature.State()
            reset.settings = $0.settings
            reset.destination = .login
            reset.authentication.errorMessage = "Session expired. Please log in again."
            $0 = reset
        }

        XCTAssertTrue(logoutCalled.value)
    }

    func testShakePresentsSettingsSheet() async {
        let store = TestStore(initialState: AppFeature.State()) {
            AppFeature()
        }

        await store.send(.shakeDetected) {
            $0.isSettingsPresented = true
        }
    }

    func testSettingsSavedTriggersReloadsWhenInApp() async {
        var initialState = AppFeature.State()
        initialState.destination = .app
        initialState.eventList.hasLoaded = true
        initialState.profile.hasLoaded = true
        initialState.friends.hasLoaded = true

        let store = TestStore(initialState: initialState) {
            AppFeature()
        }
        store.exhaustivity = .off

        await store.send(.settings(.settingsSaved(AppSettings.default))) {
            $0.eventList.hasLoaded = false
            $0.profile.hasLoaded = false
            $0.friends.hasLoaded = false
        }

        await store.receive(.eventList(.loadEvents)) {
            $0.eventList.isLoading = true
            $0.eventList.errorMessage = nil
        }
        await store.receive(.eventList(.startRealtimeConnection))
        await store.receive(.playlistList(.loadPlaylists)) {
            $0.playlistList.isLoading = true
            $0.playlistList.errorMessage = nil
        }
        await store.receive(.playlistList(.startRealtimeConnection))
        await store.receive(.friends(.loadData)) {
            $0.friends.isLoading = true
            $0.friends.errorMessage = nil
        }
        await store.receive(.profile(.onAppear)) {
            $0.profile.isLoading = true
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

import ComposableArchitecture
import XCTest

@testable import AppFeature
@testable import AppSupportClients
@testable import EventFeature
@testable import MusicRoomDomain

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
}

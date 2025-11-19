import XCTest
import ComposableArchitecture
@testable import AppFeature
@testable import MusicRoomDomain

@MainActor
final class AppFeatureTests: XCTestCase {
    func testBootstrapsSampleDataAndStream() async {
        let events = [
            Event(
                name: "Test Event",
                location: "Remote",
                visibility: .publicEvent,
                licenseTier: .everyone,
                startTime: Date(),
                playlist: [Track(title: "One", artist: "Artist", votes: 1)]
            )
        ]
        let update = PlaylistUpdate(eventID: events[0].id, updatedTrack: events[0].playlist[0], message: "Track update")
        let decision = PolicyDecision(isAllowed: true, reason: "All good")

        let store = TestStore(initialState: AppFeature.State()) {
            AppFeature()
        } withDependencies: {
            $0.musicRoomAPI.fetchSampleEvents = { events }
            $0.policyEngine.evaluate = { _ in decision }
            $0.playlistStream.startPreview = { _ in
                AsyncStream { continuation in
                    continuation.yield(update)
                    continuation.finish()
                }
            }
        }

        await store.send(.task) {
            $0.hasBootstrapped = true
            $0.isSampleDataLoading = true
            $0.sampleDataError = nil
        }

        await store.receive(.sampleEventsLoaded(events)) {
            $0.isSampleDataLoading = false
            $0.sampleEvents = events
        }

        await store.receive(.policyEvaluated(decision)) {
            $0.policySummary = "Allowed – All good"
        }

        await store.receive(.playlistUpdate(update)) {
            $0.latestStreamMessage = "Track update"
        }

        await store.receive(.playlistStreamCompleted) {
            $0.latestStreamMessage = "Stream completed"
        }
    }
}

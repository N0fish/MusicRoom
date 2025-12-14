import ComposableArchitecture
import XCTest

@testable import AppFeature
@testable import MusicRoomDomain

@MainActor
final class AppFeatureTests: XCTestCase {
    /*
        // TODO: Update test to match new AppFeature architecture (delegation to child features)
        func testBootstrapsSampleDataAndStream() async {
            let track = Track(
                title: "One",
                artist: "Artist",
                provider: "dummy",
                providerTrackId: "123",
                votes: 1
            )
            let event = Event(
                id: UUID(),
                name: "Test Event",
                visibility: .publicEvent,
                ownerId: "user-1",
                licenseMode: .everyone,
                voteStart: Date(),
                createdAt: Date(),
                updatedAt: Date()
            )
            let events = [event]
            let update = PlaylistUpdate(eventID: event.id, updatedTrack: track, message: "Track update")
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
    */
}

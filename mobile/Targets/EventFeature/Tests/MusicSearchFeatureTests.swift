import ComposableArchitecture
import MusicRoomAPI
import MusicRoomDomain
import SearchFeature
import XCTest

@testable import EventFeature

final class MusicSearchFeatureTests: XCTestCase {
    @MainActor
    func testSearchSuccess() async {
        let results = [
            MusicSearchItem(
                title: "Test Song", artist: "Test Artist", provider: "deezer",
                providerTrackId: "123", thumbnailUrl: nil)
        ]

        let store = TestStore(initialState: MusicSearchFeature.State()) {
            MusicSearchFeature()
        } withDependencies: {
            $0.musicRoomAPI.search = { _ in results }
        }

        await store.send(.searchQueryChanged("Test")) { state in
            state.query = "Test"
        }

        await store.send(.search) { state in
            state.isLoading = true
        }

        await store.receive(\.searchResponse.success) { state in
            state.isLoading = false
            state.results = results
        }
    }

    @MainActor
    func testSearchFailure() async {
        let store = TestStore(initialState: MusicSearchFeature.State()) {
            MusicSearchFeature()
        } withDependencies: {
            $0.musicRoomAPI.search = { _ in throw MusicRoomAPIError.networkError("Failed") }
        }

        await store.send(.searchQueryChanged("Test")) { state in
            state.query = "Test"
        }

        await store.send(.search) { state in
            state.isLoading = true
        }

        await store.receive(\.searchResponse.failure) { state in
            state.isLoading = false
            state.errorMessage = "Network Error: Failed"
        }
    }
    @MainActor
    func testSearchCancellation() async {
        let store = TestStore(initialState: MusicSearchFeature.State()) {
            MusicSearchFeature()
        } withDependencies: {
            // Mock a long running search that effectively never returns (or returns after cancellation)
            $0.musicRoomAPI.search = { _ in
                try await Task.sleep(nanoseconds: 1_000_000_000)
                return []
            }
        }

        await store.send(.searchQueryChanged("Cancel Me")) {
            $0.query = "Cancel Me"
        }

        await store.send(.search) {
            $0.isLoading = true
        }

        // Clearing query should cancel the in-flight effect
        await store.send(.searchQueryChanged("")) {
            $0.query = ""
            $0.results = []
        }
    }

    @MainActor
    func testTrackTapped() async {
        let item = MusicSearchItem(
            title: "Tapped Song", artist: "Artist", provider: "deezer",
            providerTrackId: "999", thumbnailUrl: nil)

        var state = MusicSearchFeature.State()
        state.results = [item]

        let store = TestStore(initialState: state) {
            MusicSearchFeature()
        }

        // Track tapped is a "delegate" action (no state change, just notification)
        await store.send(MusicSearchFeature.Action.trackTapped(item))
        await store.receive(.delegate(.trackTapped(item)))
    }
}

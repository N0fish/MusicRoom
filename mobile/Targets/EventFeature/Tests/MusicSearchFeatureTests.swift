import ComposableArchitecture
import MusicRoomAPI
import MusicRoomDomain
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

        await store.send(.searchQueryChanged("Test")) {
            $0.query = "Test"
        }

        await store.send(.search) {
            $0.isLoading = true
        }

        await store.receive(\.searchResponse.success) {
            $0.isLoading = false
            $0.results = results
        }
    }

    @MainActor
    func testSearchFailure() async {
        let store = TestStore(initialState: MusicSearchFeature.State()) {
            MusicSearchFeature()
        } withDependencies: {
            $0.musicRoomAPI.search = { _ in throw MusicRoomAPIError.networkError("Failed") }
        }

        await store.send(.searchQueryChanged("Test")) {
            $0.query = "Test"
        }

        await store.send(.search) {
            $0.isLoading = true
        }

        await store.receive(\.searchResponse.failure) {
            $0.isLoading = false
            $0.errorMessage = "Network Error: Failed"
        }
    }
}

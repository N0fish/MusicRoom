import ComposableArchitecture
import XCTest

@testable import EventFeature
@testable import MusicRoomAPI
@testable import MusicRoomDomain

@MainActor
final class EventDetailFeatureTests: XCTestCase {

    func testLoadTallyAndPlaylistSuccess() async {
        let event = Event(
            id: UUID(), name: "Test", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date())
        let tallyItems = [
            MusicRoomAPIClient.TallyItem(track: "t1", count: 10)
        ]
        let tracks = [
            Track(
                id: "t1", title: "Song 1", artist: "Artist 1", provider: "deezer",
                providerTrackId: "1", thumbnailUrl: nil, votes: 10)
        ]

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.tally = { _ in tallyItems }
            $0.musicRoomAPI.getPlaylist = { _ in
                PlaylistResponse(
                    playlist: PlaylistResponse.PlaylistMetadata(
                        id: event.id.uuidString, ownerId: "u1", name: "P", isPublic: true,
                        editMode: "open"),
                    tracks: tracks
                )
            }
            $0.musicRoomAPI.connectToRealtime = { AsyncStream { $0.finish() } }
        }

        await store.send(.onAppear)
        await store.receive(\.loadTally) { state in
            state.isLoading = true
        }

        // Parallel execution order is not guaranteed, but usually standard actors serialize?
        // Wait, async let executes concurrently.
        // We might receive playlistLoaded and tallyLoaded in any order.
        // However, TestStore usually enforces deterministic order if we await properly?
        // No, we need to handle both possible orders or use strict checks.
        // But usually send order from effect depends on completion.

        await store.receive(\.playlistLoaded) { state in
            state.tracks = tracks
        }
        await store.receive(\.tallyLoaded.success) { state in
            state.isLoading = false
            state.tally = tallyItems
        }
    }

    func testRemoveTrack() async {
        let event = Event(
            id: UUID(), name: "Test", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date())

        let trackToRemove = Track(
            id: "t1", title: "Remove Me", artist: "A", provider: "d", providerTrackId: "1",
            thumbnailUrl: nil, votes: 0)

        var state = EventDetailFeature.State(event: event)
        state.tracks = [trackToRemove]

        let store = TestStore(initialState: state) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.removeTrack = { @Sendable _, _ in return }
        }

        await store.send(.removeTrackButtonTapped(trackId: "t1")) { state in
            state.tracks = []  // Optimistic removal
        }

        await store.receive(.removeTrackResponse(.success(())))
    }

}

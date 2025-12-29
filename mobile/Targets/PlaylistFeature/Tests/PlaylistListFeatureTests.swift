import ComposableArchitecture
import XCTest

@testable import AppSupportClients
@testable import MusicRoomAPI
@testable import MusicRoomDomain
@testable import PlaylistFeature

@MainActor
final class PlaylistListFeatureTests: XCTestCase {
    private struct MockError: Error, Equatable, LocalizedError {
        var errorDescription: String? { "Mock failed" }
    }

    func testDeletePlaylist_RemovesFromState() async {
        let first = Playlist(id: "pl-1", ownerId: "u1", name: "First", description: "")
        let second = Playlist(id: "pl-2", ownerId: "u1", name: "Second", description: "")

        var state = PlaylistListFeature.State()
        state.playlists = [first, second]

        let deletedIds = LockIsolated<[String]>([])

        let store = TestStore(initialState: state) {
            PlaylistListFeature()
        } withDependencies: {
            $0.playlistClient.delete = { id in
                deletedIds.setValue([id])
            }
        }

        await store.send(.deletePlaylist(first))
        await store.receive(.playlistDeleted(.success(first.id))) {
            $0.playlists = [second]
        }

        XCTAssertEqual(deletedIds.value, [first.id])
    }

    func testDeletePlaylist_FailureSetsError() async {
        let playlist = Playlist(id: "pl-1", ownerId: "u1", name: "First", description: "")

        var state = PlaylistListFeature.State()
        state.playlists = [playlist]

        let store = TestStore(initialState: state) {
            PlaylistListFeature()
        } withDependencies: {
            $0.playlistClient.delete = { _ in
                throw MockError()
            }
        }

        await store.send(.deletePlaylist(playlist))
        await store.receive(.playlistDeleted(.failure(MockError()))) {
            $0.errorMessage = "Failed to delete playlist: Mock failed"
        }
    }

    func testRealtimeDelete_RemovesPlaylist() async {
        let playlist = Playlist(id: "pl-1", ownerId: "u1", name: "First", description: "")

        var state = PlaylistListFeature.State()
        state.playlists = [playlist]

        let store = TestStore(initialState: state) {
            PlaylistListFeature()
        }

        let message = RealtimeMessage(
            type: "playlist.deleted",
            payload: AnyDecodable(["playlistId": playlist.id])
        )

        await store.send(.realtimeMessageReceived(message)) {
            $0.playlists = []
        }
    }
}

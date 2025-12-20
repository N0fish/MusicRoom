import ComposableArchitecture
import MusicRoomAPI
import XCTest

@testable import MusicRoomDomain
@testable import PlaylistFeature

@MainActor
final class PlaylistDetailFeatureTests: XCTestCase {

    func testPlaybackToggle() async {
        let playlist = Playlist(
            id: "p1", ownerId: "u1", name: "Test Playlist", isPublic: true, editMode: "open")
        let track1 = Track(
            id: "t1", title: "Song 1", artist: "Artist 1", provider: "youtube",
            providerTrackId: "v1", thumbnailUrl: nil)

        let store = TestStore(initialState: PlaylistDetailFeature.State(playlist: playlist)) {
            PlaylistDetailFeature()
        } withDependencies: {
            $0.playlistClient.get = { _ in
                PlaylistResponse(
                    playlist: Playlist(
                        id: "p1", ownerId: "u1", name: "Test Playlist", isPublic: true,
                        editMode: "open"), tracks: [])
            }
        }

        // initial state
        await store.send(.togglePlayback(track1)) {
            $0.playingTrackId = "t1"
            $0.isPlaying = true
        }

        // pause
        await store.send(.togglePlayback(track1)) {
            $0.isPlaying = false
        }

        // resume
        await store.send(.togglePlayback(track1)) {
            $0.isPlaying = true
        }
    }

    func testPlaybackSwitch() async {
        let playlist = Playlist(
            id: "p1", ownerId: "u1", name: "Test Playlist", isPublic: true, editMode: "open")
        let track1 = Track(
            id: "t1", title: "Song 1", artist: "Artist 1", provider: "youtube",
            providerTrackId: "v1", thumbnailUrl: nil)
        let track2 = Track(
            id: "t2", title: "Song 2", artist: "Artist 2", provider: "youtube",
            providerTrackId: "v2", thumbnailUrl: nil)

        let store = TestStore(initialState: PlaylistDetailFeature.State(playlist: playlist)) {
            PlaylistDetailFeature()
        } withDependencies: {
            $0.playlistClient.get = { _ in
                PlaylistResponse(
                    playlist: Playlist(
                        id: "p1", ownerId: "u1", name: "Test Playlist", isPublic: true,
                        editMode: "open"), tracks: [])
            }
        }

        // Play track 1
        await store.send(.togglePlayback(track1)) {
            $0.playingTrackId = "t1"
            $0.isPlaying = true
        }

        // Switch to track 2
        await store.send(.togglePlayback(track2)) {
            $0.playingTrackId = "t2"
            $0.isPlaying = true
        }
    }

    func testPauseResumeActions() async {
        let playlist = Playlist(
            id: "p1", ownerId: "u1", name: "Test Playlist", isPublic: true, editMode: "open")
        let track1 = Track(
            id: "t1", title: "Song 1", artist: "Artist 1", provider: "youtube",
            providerTrackId: "v1", thumbnailUrl: nil)

        let store = TestStore(initialState: PlaylistDetailFeature.State(playlist: playlist)) {
            PlaylistDetailFeature()
        } withDependencies: {
            $0.playlistClient.get = { _ in
                PlaylistResponse(
                    playlist: Playlist(
                        id: "p1", ownerId: "u1", name: "Test Playlist", isPublic: true,
                        editMode: "open"), tracks: [])
            }
        }

        // Play track 1
        await store.send(.togglePlayback(track1)) {
            $0.playingTrackId = "t1"
            $0.isPlaying = true
        }

        // Directly Pause
        await store.send(.pauseTrack) {
            $0.isPlaying = false
        }

        // Directly Resume
        await store.send(.resumeTrack) {
            $0.isPlaying = true
        }
    }

    func testAutoAdvance() async {
        let playlist = Playlist(
            id: "p1", ownerId: "u1", name: "Test Playlist", isPublic: true, editMode: "open")
        let track1 = Track(
            id: "t1", title: "Song 1", artist: "Artist 1", provider: "youtube",
            providerTrackId: "v1", thumbnailUrl: nil)
        let track2 = Track(
            id: "t2", title: "Song 2", artist: "Artist 2", provider: "youtube",
            providerTrackId: "v2", thumbnailUrl: nil)

        var state = PlaylistDetailFeature.State(playlist: playlist)
        state.tracks = [track1, track2]

        let store = TestStore(initialState: state) {
            PlaylistDetailFeature()
        } withDependencies: {
            $0.playlistClient.get = { _ in
                PlaylistResponse(
                    playlist: playlist,
                    tracks: [track1, track2])
            }
        }

        // Play track 1
        await store.send(.togglePlayback(track1)) {
            $0.playingTrackId = "t1"
            $0.isPlaying = true
        }

        // Track finishes -> Auto-advance to track 2
        await store.send(.playbackFinished) {
            $0.playingTrackId = "t2"
            $0.isPlaying = true
        }

        // Track finishes -> Stop (end of list)
        await store.send(.playbackFinished) {
            $0.playingTrackId = nil
            $0.isPlaying = false
        }
    }
}

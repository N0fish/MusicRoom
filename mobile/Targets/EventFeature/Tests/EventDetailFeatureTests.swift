import ComposableArchitecture
import CoreLocation
import XCTest

@testable import AppSupportClients
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
            // Mock persistence to do nothing
            $0.persistence.savePlaylist = { _ in }
            $0.persistence.loadPlaylist = { throw PersistenceError.notFound }
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

        await store.receive(.removeTrackResponse(.success("t1")))
    }

    func testVoteWithLocation_Success() async {
        let event = Event(
            id: UUID(), name: "Geo Event", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .geoTime,  // Geo restricted
            createdAt: Date(), updatedAt: Date())

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.locationClient.requestWhenInUseAuthorization = {}
            $0.locationClient.getCurrentLocation = {
                CLLocationCoordinate2D(latitude: 48.8966, longitude: 2.3183)
            }
            $0.musicRoomAPI.vote = { _, _, lat, lng in
                // Verify lat/lng passed
                XCTAssertEqual(lat, 48.8966)
                XCTAssertEqual(lng, 2.3183)
                return VoteResponse(status: "ok", trackId: "t1", totalVotes: 1)
            }
            $0.musicRoomAPI.tally = { _ in [] }
            $0.musicRoomAPI.getPlaylist = { _ in
                PlaylistResponse(
                    playlist: PlaylistResponse.PlaylistMetadata(
                        id: "1", ownerId: "u", name: "P", isPublic: true, editMode: "o"),
                    tracks: []
                )
            }
            $0.persistence.savePlaylist = { _ in }
            $0.continuousClock = ImmediateClock()
            $0.telemetry.log = { action, metadata in
                if action == "event.vote.attempt" {
                    XCTAssertEqual(metadata["eventId"], event.id.uuidString)
                    XCTAssertEqual(metadata["trackId"], "t1")
                }
            }
        }

        await store.send(.voteButtonTapped(trackId: "t1")) {
            $0.isVoting = true
            // Optimistic add to tally
            $0.tally = [MusicRoomAPIClient.TallyItem(track: "t1", count: 1)]
            $0.userAlert = nil
        }

        await store.receive(
            .voteResponse(.success(VoteResponse(status: "ok", trackId: "t1", totalVotes: 1)))
        ) {
            $0.isVoting = false
            $0.userAlert = EventDetailFeature.UserAlert(
                title: "Success", message: "Voted for t1!", type: .success)
        }

        // Wait for clock sleep 1s
        await store.receive(\.loadTally) {
            $0.isLoading = true
        }

        // Parallel loads
        await store.receive(.playlistLoaded([]))
        await store.receive(.tallyLoaded(.success([]))) {
            $0.isLoading = false
            $0.tally = []  // Mock returns empty
        }

        // Wait for clock sleep 2s then dismiss
        await store.receive(\.dismissInfo) {
            $0.userAlert = nil
        }
    }

    func testVote_TimeRestricted() async {
        // Event started in future
        let future = Date().addingTimeInterval(3600)
        let event = Event(
            id: UUID(), name: "Future Event", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .everyone,
            voteStart: future,
            createdAt: Date(), updatedAt: Date())

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        }

        await store.send(.voteButtonTapped(trackId: "t1")) {
            // State checks should reflect that NO change happens to isVoting (or it reverts immediately)
            // But since my reducer returns .none and sets userAlert in the same block,
            // the state mutation passed to assertion closure must match the FINAL state.
            $0.isVoting = false
            $0.userAlert = EventDetailFeature.UserAlert(
                title: "Voting Not Started",
                message: "Voting will begin at \(future.formatted()).",
                type: .info
            )
        }
    }

    func testAddTrack_Success() async {
        let event = Event(
            id: UUID(), name: "Add Track Event", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date())

        let newTrackItem = MusicSearchItem(
            title: "New Song", artist: "New Artist", provider: "youtube",
            providerTrackId: "new1", thumbnailUrl: URL(string: "http://thumb.url")
        )

        let addedTrack = Track(
            id: "t_new", title: "New Song", artist: "New Artist", provider: "youtube",
            providerTrackId: "new1", thumbnailUrl: URL(string: "http://thumb.url"), votes: 0
        )

        var state = EventDetailFeature.State(event: event)
        // Simulate search is OPEN
        state.musicSearch = MusicSearchFeature.State()

        let store = TestStore(initialState: state) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.addTrack = { _, req in
                // Verify request
                print("DEBUG: addTrack called with \(req.title)")
                XCTAssertEqual(req.title, "New Song")
                XCTAssertEqual(req.provider, "youtube")
                return addedTrack
            }
            $0.musicRoomAPI.tally = { _ in [] }
            $0.musicRoomAPI.getPlaylist = { _ in
                PlaylistResponse(
                    playlist: PlaylistResponse.PlaylistMetadata(
                        id: event.id.uuidString, ownerId: "u1", name: "P", isPublic: true,
                        editMode: "o"),
                    tracks: [addedTrack]
                )
            }
            // Mock persistence
            $0.persistence.savePlaylist = { _ in }
            $0.continuousClock = ImmediateClock()
        }

        // Simulate search result selection
        await store.send(.musicSearch(.presented(.trackTapped(newTrackItem)))) {
            // Debugging the state passed to closure
            XCTAssertNotNil($0.musicSearch, "Start state musicSearch should be non-nil")
            // With deferred dismissal, musicSearch remains non-nil here
            $0.isLoading = true
        }

        // Dismiss happens first now (immediate await)
        await store.receive(.dismissMusicSearch) {
            $0.musicSearch = nil
        }

        await store.receive(.addTrackResponse(.success(addedTrack))) {
            $0.isLoading = false
            $0.userAlert = EventDetailFeature.UserAlert(
                title: "Success",
                message: "Added New Song to playlist",
                type: .success
            )
            $0.tracks.append(addedTrack)
        }

        // Then expecting loadTally
        await store.receive(.loadTally) {
            $0.isLoading = true
        }

        // loadTally triggers playlistLoaded AND tallyLoaded
        await store.receive(.playlistLoaded([addedTrack]))
        // No modification expected as tracks already updated

        // Final action from loadTally
        await store.receive(.tallyLoaded(.success([]))) {
            $0.isLoading = false
            // Tally sorted
            $0.tally = []
        }

    }
}

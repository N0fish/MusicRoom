import Clocks
import ComposableArchitecture
import CoreLocation
import SearchFeature
import XCTest

@testable import AppSupportClients
@testable import EventFeature
@testable import MusicRoomAPI
@testable import MusicRoomDomain

@MainActor
final class EventDetailFeatureTests: XCTestCase {

    func testOnAppear_LoadsData() async {
        let clock = TestClock()
        let event = Event(
            id: UUID(), name: "Test Event", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date())

        let tracks = [
            Track(
                id: "t1", title: "Song 1", artist: "Artist 1", provider: "deezer",
                providerTrackId: "1", thumbnailUrl: nil, votes: 10)
        ]

        let fixedDate = Date(timeIntervalSince1970: 0)

        let currentUser = UserProfile(
            id: "u1",
            userId: "u1",
            username: "user",
            displayName: "User",
            avatarUrl: nil,
            hasCustomAvatar: false,
            bio: nil,
            visibility: "public",
            preferences: UserPreferences(),
            isPremium: false,
            linkedProviders: [],
            email: "test@example.com"
        )

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.getPlaylist = { _ in
                PlaylistResponse(
                    playlist: Playlist(
                        id: event.id.uuidString, ownerId: "u1", name: "P", isPublic: true,
                        editMode: "open", createdAt: fixedDate),
                    tracks: tracks
                )
            }
            $0.musicRoomAPI.getEvent = { _ in event }
            $0.musicRoomAPI.connectToRealtime = { AsyncStream { $0.finish() } }
            $0.user.me = { currentUser }

            $0.persistence.savePlaylist = { _ in }
            $0.persistence.loadPlaylist = { throw PersistenceError.notFound }
            $0.continuousClock = clock
        }
        store.exhaustivity = .off

        await store.send(EventDetailFeature.Action.onAppear)

        await store.receive(EventDetailFeature.Action.loadPlaylist) {
            $0.isLoading = true
        }
        await store.receive(EventDetailFeature.Action.loadEvent)

        await store.receive(EventDetailFeature.Action.currentUserLoaded(.success(currentUser))) {
            $0.currentUserId = "u1"
            $0.event.isJoined = true
        }

        await store.receive(
            EventDetailFeature.Action.playlistLoaded(
                .success(
                    PlaylistResponse(
                        playlist: Playlist(
                            id: event.id.uuidString, ownerId: "u1", name: "P", isPublic: true,
                            editMode: "open", createdAt: fixedDate),
                        tracks: tracks
                    )))
        ) {
            $0.isLoading = false
            $0.tracks = tracks
            $0.metadata = Playlist(
                id: event.id.uuidString, ownerId: "u1", name: "P", isPublic: true,
                editMode: "open", createdAt: fixedDate)
        }

        await store.receive(EventDetailFeature.Action.eventLoaded(.success(event)))

        // 4. Advance time to trigger timer
        await clock.advance(by: .seconds(1))
        await store.receive(EventDetailFeature.Action.timerTick)

        await store.send(EventDetailFeature.Action.onDisappear)
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

        await store.send(EventDetailFeature.Action.removeTrackButtonTapped(trackId: "t1")) {
            state in
            state.tracks = []  // Optimistic removal
        }

        await store.receive(EventDetailFeature.Action.removeTrackResponse(.success("t1")))
    }

    func testVoteWithLocation_Success() async {
        let clock = TestClock()
        let event = Event(
            id: UUID(),
            name: "Geo Event",
            visibility: .publicEvent,
            ownerId: "u1",
            licenseMode: .geoTime,
            createdAt: Date(),
            updatedAt: Date()
        )

        let track = Track(
            id: "t1", title: "Song 1", artist: "A",
            provider: "y", providerTrackId: "p1",
            durationMs: 1000,
            voteCount: 0,
            isVoted: false
        )

        var state = EventDetailFeature.State(event: event)
        state.tracks = [track]

        let fixedDate = Date(timeIntervalSince1970: 0)
        let store = TestStore(initialState: state) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.vote = { _, _, _, _ in VoteResponse(voteCount: 1) }
            $0.locationClient.getCurrentLocation = {
                CLLocationCoordinate2D(latitude: 0, longitude: 0)
            }
            $0.musicRoomAPI.getPlaylist = { _ in
                PlaylistResponse(
                    playlist: Playlist(
                        id: "1", ownerId: "u", name: "P", description: "", isPublic: true,
                        editMode: "o", createdAt: fixedDate),
                    tracks: [track]
                )
            }
            $0.continuousClock = clock
        }

        await store.send(EventDetailFeature.Action.voteButtonTapped(trackId: "t1")) {
            $0.isVoting = true
            $0.tracks[0].isVoted = true
            $0.tracks[0].voteCount = 1
        }

        await store.receive(
            EventDetailFeature.Action.voteResponse(
                .success(VoteResponse(voteCount: 1)), trackId: "t1")
        ) {
            $0.isVoting = false
            $0.userAlert = EventDetailFeature.UserAlert(
                title: "Success", message: "Voted for track!", type: .success)
        }

        // Wait for clock sleep 1s then loadPlaylist
        await clock.advance(by: .seconds(1))
        await store.receive(EventDetailFeature.Action.loadPlaylist) {
            $0.isLoading = true
        }

        // Parallel loads
        await store.receive(
            EventDetailFeature.Action.playlistLoaded(
                .success(
                    PlaylistResponse(
                        playlist: Playlist(
                            id: "1", ownerId: "u", name: "P", description: "", isPublic: true,
                            editMode: "o", createdAt: fixedDate),
                        tracks: [track]
                    )))
        ) {
            $0.isLoading = false
            $0.tracks = [track]
            $0.metadata = Playlist(
                id: "1", ownerId: "u", name: "P", description: "", isPublic: true,
                editMode: "o", createdAt: fixedDate)
        }

        // Wait for clock sleep 2s then dismiss
        await clock.advance(by: .seconds(2))
        await store.receive(EventDetailFeature.Action.dismissInfo) {
            $0.userAlert = nil
        }
    }

    func testVote_TimeRestricted() async {
        // Event started in future
        let future = Date().addingTimeInterval(3600)
        let event = Event(
            id: UUID(),
            name: "Future Event",
            visibility: .publicEvent,
            ownerId: "u1",
            licenseMode: .everyone,
            voteStart: future,
            createdAt: Date(),
            updatedAt: Date()
        )

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        }

        await store.send(EventDetailFeature.Action.voteButtonTapped(trackId: "t1")) {
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

        let clock = TestClock()
        let addedTrack = Track(
            id: "t_new", title: "New Song", artist: "New Artist", provider: "youtube",
            providerTrackId: "new1", thumbnailUrl: URL(string: "http://thumb.url"), votes: 0
        )

        // let fixedDate = Date(timeIntervalSince1970: 0)

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
                    playlist: Playlist(
                        id: event.id.uuidString, ownerId: "u1", name: "P", isPublic: true,
                        editMode: "o"),
                    tracks: [addedTrack]
                )
            }
            // Mock persistence
            $0.persistence.savePlaylist = { _ in }
            $0.continuousClock = clock
        }
        store.exhaustivity = .off

        // Simulate search result selection
        await store.send(
            EventDetailFeature.Action.musicSearch(.presented(.trackTapped(newTrackItem)))
        ) {
            // Debugging the state passed to closure
            XCTAssertNotNil($0.musicSearch, "Start state musicSearch should be non-nil")
            // With deferred dismissal, musicSearch remains non-nil here
            $0.isLoading = true
        }

        // Dismiss happens first now (immediate await)
        await store.receive(EventDetailFeature.Action.dismissMusicSearch) {
            $0.musicSearch = nil
        }

        await store.receive(EventDetailFeature.Action.addTrackResponse(.success(addedTrack))) {
            $0.isLoading = false
        }

        /*
        // Then expecting loadPlaylist
        await clock.advance(by: .milliseconds(1))
        await store.receive(EventDetailFeature.Action.loadPlaylist) {
            $0.isLoading = true
        }
        
        // loadPlaylist triggers playlistLoaded
        await store.receive(
            EventDetailFeature.Action.playlistLoaded(
                .success(
                    PlaylistResponse(
                        playlist: Playlist(
                            id: event.id.uuidString, ownerId: "u1", name: "P", description: "",
                            isPublic: true, editMode: "o", createdAt: fixedDate),
                        tracks: [addedTrack]
                    )))
        ) {
            $0.isLoading = false
        }
        // No modification expected as tracks already updated
        
        await clock.advance(by: .seconds(4))
        await store.receive(EventDetailFeature.Action.dismissInfo) {
            $0.userAlert = nil
        }
        */

    }

    func testTransferOwnership_Success() async {
        let event = Event(
            id: UUID(), name: "Transfer Event", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date())

        let newOwner = PublicUserProfile(
            userId: "u2", username: "next_owner", displayName: "Next Owner",
            avatarUrl: nil, isPremium: false, bio: nil, visibility: "public",
            preferences: nil
        )

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.transferOwnership = { _, newOwnerId in
                XCTAssertEqual(newOwnerId, "u2")
            }
            $0.musicRoomAPI.getEvent = { _ in
                // Return updated event? Or same event?
                // Usually getEvent is called after success.
                var updated = event
                updated.ownerId = "u2"
                return updated
            }
            // Stubs for onAppear checks if triggered, but we are just testing the action flow
            $0.musicRoomAPI.getPlaylist = { _ in
                PlaylistResponse(
                    playlist: Playlist(
                        id: event.id.uuidString, ownerId: "u1", name: "P", isPublic: true,
                        editMode: "o"),
                    tracks: []
                )
            }
        }
        store.exhaustivity = .off  // Focus on transfer flow

        // 1. Request Transfer
        await store.send(EventDetailFeature.Action.requestTransferOwnership(newOwner)) {
            $0.confirmationDialog = ConfirmationDialogState {
                TextState("Transfer Ownership?")
            } actions: {
                ButtonState(role: .cancel) {
                    TextState("Cancel")
                }
                ButtonState(role: .destructive, action: .transferOwnership(newOwner)) {
                    TextState("Transfer to \(newOwner.username)")
                }
            } message: {
                TextState(
                    "Are you sure you want to transfer ownership to \(newOwner.username)? You will lose control of this event."
                )
            }
        }

        // 2. Confirm Transfer
        await store.send(EventDetailFeature.Action.transferOwnership(newOwner)) {
            $0.confirmationDialog = nil  // Dialog dismissed automatically?
            // Ideally tapping button in dialog triggers action and dismisses it.
            // In TCA test, we send the action that the button would send.
            // But does it clear confirmationDialog state automatically in test? YES.
        }

        // 3. Handle Response
        await store.receive(
            EventDetailFeature.Action.transferOwnershipResponse(.success("Success"))
        ) {
            $0.userAlert = EventDetailFeature.UserAlert(
                title: "Success", message: "Ownership transferred.", type: .success)
        }

        await store.receive(EventDetailFeature.Action.loadEvent)
    }

    func testInviteFriendFlow() async {
        let clock = TestClock()
        let event = Event(
            id: UUID(), name: "Invite Event", visibility: .privateEvent, ownerId: "u1",
            licenseMode: .invitedOnly, createdAt: Date(), updatedAt: Date()
        )
        let friend = Friend(
            id: "f1",
            userId: "u2",
            username: "friend",
            displayName: "Friend Name",
            avatarUrl: nil,
            isPremium: false
        )

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.friendsClient.listFriends = { [friend] }
            $0.musicRoomAPI.inviteUser = { _, _ in }
            $0.continuousClock = clock
        }

        await store.send(.inviteButtonTapped) {
            $0.isInvitingFriends = true
        }

        await store.receive(.inviteFriendsLoaded(.success([friend]))) {
            $0.isInvitingFriends = false
            $0.friends = [friend]
            $0.isShowingInviteSheet = true
        }

        await store.send(.inviteFriendTapped(friend)) {
            $0.isShowingInviteSheet = false
        }

        await store.receive(.inviteFriendResponse(.success(friend))) {
            $0.userAlert = EventDetailFeature.UserAlert(
                title: "Invite Sent",
                message: "Invited Friend Name to this event.",
                type: .success
            )
        }

        await clock.advance(by: .seconds(2))
        await store.receive(.dismissInfo) {
            $0.userAlert = nil
        }
    }

    func testCanVote_PropertyCheck() {
        var state = EventDetailFeature.State(
            event: .init(
                id: UUID(), name: "E", visibility: .publicEvent, ownerId: "o",
                licenseMode: .everyone, createdAt: Date(), updatedAt: Date()))

        // Default from init is false
        XCTAssertFalse(state.canVote)

        // Set to true
        state.event.canVote = true
        XCTAssertTrue(state.canVote)

        // Set to false
        state.event.canVote = false
        XCTAssertFalse(state.canVote)
    }

    func testJoin_Optimistic_Everyone_SetsCanVote() async {
        let event = Event(
            id: UUID(), name: "Public Open", visibility: .publicEvent, ownerId: "o",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date(), isJoined: false,
            canVote: false)

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.joinEvent = { _ in }
        }

        await store.send(EventDetailFeature.Action.joinEventTapped) {
            $0.event.isJoined = true
            $0.event.canVote = true  // Optimistic update for Everyone mode
        }

        await store.receive(EventDetailFeature.Action.delegate(.eventJoined))
    }

    func testJoin_Optimistic_PublicInvited_GuestOnly() async {
        let event = Event(
            id: UUID(), name: "Public Invited", visibility: .publicEvent, ownerId: "o",
            licenseMode: .invitedOnly, createdAt: Date(), updatedAt: Date(), isJoined: false,
            canVote: false)

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.joinEvent = { _ in }
        }

        await store.send(EventDetailFeature.Action.joinEventTapped) {
            $0.event.isJoined = true
            $0.event.canVote = false  // Should NOT act optimistically for invitedOnly (remains Guest)
        }

        await store.receive(EventDetailFeature.Action.delegate(.eventJoined))
    }

    func testAddTrack_RestrictedIfCannotVote() async {
        let event = Event(
            id: UUID(), name: "Restricted", visibility: .publicEvent, ownerId: "o",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date(), isJoined: true,
            canVote: false)  // User joined but lost voting rights or restricted

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        }

        // Tapping add track should do NOTHING (no state change, no check logic)
        // Note: Logic for blocking this should vary.
        // If Logic is in View (disabled button), Reducer might still handle it if sent?
        // Ideally Reducer should ALSO guard it.
        // Let's check Reducer implementation...
        // Reducer doesn't currently guard it. Let's add the guard in Reducer via this test failure-driven dev.
        await store.send(EventDetailFeature.Action.addTrackButtonTapped)
        // If reducer has no guard, this will trigger navigation to music search.
        // We expect it to NOT TRIGGER anything.
    }
}

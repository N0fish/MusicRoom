import ComposableArchitecture
import XCTest

@testable import EventFeature
@testable import MusicRoomAPI
@testable import MusicRoomDomain

@MainActor
final class EventDetailFeatureTests: XCTestCase {
    func testLoadTallySuccess() async {
        let event = Event(
            id: UUID(), name: "Test", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date())
        let tallyItems = [
            MusicRoomAPIClient.TallyItem(track: "t1", count: 10),
            MusicRoomAPIClient.TallyItem(track: "t2", count: 5),
        ]

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.tally = { _ in tallyItems }
        }

        await store.send(.onAppear)
        await store.receive(\.loadTally) {
            $0.isLoading = true
        }
        await store.receive(\.tallyLoaded.success) {
            $0.isLoading = false
            $0.tally = tallyItems
        }
    }

    func testVoteSuccess() async {
        let event = Event(
            id: UUID(), name: "Test", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date())

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.vote = { _, _, _, _ in
                VoteResponse(status: "ok", trackId: "t1", totalVotes: 11)
            }
            $0.musicRoomAPI.tally = { _ in [] }  // Mock tally refresh
        }

        await store.send(.voteButtonTapped(trackId: "t1")) {
            $0.isVoting = true
        }

        await store.receive(\.voteResponse.success) {
            $0.isVoting = false
            $0.successMessage = "Vote registered!"
        }

        await store.receive(\.loadTally) {
            $0.isLoading = true
        }

        await store.receive(\.tallyLoaded.success) {
            $0.isLoading = false
            $0.tally = []
        }
    }
}

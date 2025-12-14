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
        await store.receive(\.loadTally) { state in
            state.isLoading = true
        }
        await store.receive(\.tallyLoaded.success) { state in
            state.isLoading = false
            state.tally = tallyItems
        }
    }

    func testVoteSuccess() async {
        let clock = TestClock()
        let event = Event(
            id: UUID(), name: "Test", visibility: .publicEvent, ownerId: "u1",
            licenseMode: .everyone, createdAt: Date(), updatedAt: Date())

        let store = TestStore(initialState: EventDetailFeature.State(event: event)) {
            EventDetailFeature()
        } withDependencies: {
            $0.musicRoomAPI.vote = { _, _, _, _ in
                VoteResponse(status: "ok", trackId: "t1", totalVotes: 11)
            }
            $0.musicRoomAPI.tally = { _ in [] }
            $0.continuousClock = clock
        }

        await store.send(.voteButtonTapped(trackId: "t1")) { state in
            state.isVoting = true
        }

        await store.receive(\.voteResponse.success) { state in
            state.isVoting = false
            state.successMessage = "Voted for t1!"
        }

        await clock.advance(by: .seconds(2))

        await store.receive(\.dismissInfo) {
            $0.errorMessage = nil
            $0.successMessage = nil
        }
    }
}

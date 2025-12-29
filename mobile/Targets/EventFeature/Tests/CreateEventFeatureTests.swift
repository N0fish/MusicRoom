import ComposableArchitecture
import XCTest

@testable import EventFeature

@MainActor
final class CreateEventFeatureTests: XCTestCase {
    func testVoteStartBindingClampsToNowPlusMinute() async {
        let now = Date(timeIntervalSince1970: 1_735_689_600)  // 2025-01-01 00:00:00 UTC
        var state = CreateEventFeature.State()
        state.voteStart = now.addingTimeInterval(60)
        state.voteEnd = now.addingTimeInterval(120)

        let store = TestStore(initialState: state) {
            CreateEventFeature()
        } withDependencies: {
            $0.date = .constant(now)
        }

        await store.send(.binding(.set(\.voteStart, now))) {
            $0.voteStart = now.addingTimeInterval(60)
            $0.voteEnd = now.addingTimeInterval(120)
        }
    }

    func testVoteEndBindingClampsToStartPlusMinute() async {
        let now = Date(timeIntervalSince1970: 1_735_689_600)  // 2025-01-01 00:00:00 UTC
        var state = CreateEventFeature.State()
        state.voteStart = now.addingTimeInterval(60)
        state.voteEnd = now.addingTimeInterval(120)

        let store = TestStore(initialState: state) {
            CreateEventFeature()
        } withDependencies: {
            $0.date = .constant(now)
        }

        await store.send(.binding(.set(\.voteEnd, now.addingTimeInterval(90)))) {
            $0.voteEnd = now.addingTimeInterval(120)
        }
    }

    func testVoteStartBindingBumpsEndIfNeeded() async {
        let now = Date(timeIntervalSince1970: 1_735_689_600)  // 2025-01-01 00:00:00 UTC
        var state = CreateEventFeature.State()
        state.voteStart = now.addingTimeInterval(60)
        state.voteEnd = now.addingTimeInterval(120)

        let store = TestStore(initialState: state) {
            CreateEventFeature()
        } withDependencies: {
            $0.date = .constant(now)
        }

        await store.send(.binding(.set(\.voteStart, now.addingTimeInterval(600)))) {
            $0.voteStart = now.addingTimeInterval(600)
            $0.voteEnd = now.addingTimeInterval(660)
        }
    }
}

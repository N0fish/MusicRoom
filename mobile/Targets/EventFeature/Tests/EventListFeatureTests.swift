import ComposableArchitecture
import XCTest

@testable import EventFeature
@testable import MusicRoomAPI
@testable import MusicRoomDomain

@MainActor
final class EventListFeatureTests: XCTestCase {
    func testLoadEventsSuccess() async {
        let events = [
            Event(
                id: UUID(),
                name: "Test Party",
                visibility: .publicEvent,
                ownerId: "u1",
                licenseMode: .everyone,
                createdAt: Date(),
                updatedAt: Date()
            )
        ]

        let store = TestStore(initialState: EventListFeature.State()) {
            EventListFeature()
        } withDependencies: {
            $0.musicRoomAPI.listEvents = { events }
            $0.telemetry.log = { _, _ in }
        }

        await store.send(.onAppear) {
            // onAppear triggers loadEvents if empty
        }

        await store.receive(\.loadEvents) {
            $0.isLoading = true
        }

        await store.receive(\.eventsLoaded.success) {
            $0.isLoading = false
            $0.events = events
        }
    }

    func testLoadEventsFailure() async {
        struct MockError: Error, Equatable {}

        let store = TestStore(initialState: EventListFeature.State()) {
            EventListFeature()
        } withDependencies: {
            $0.musicRoomAPI.listEvents = { throw MockError() }
            $0.telemetry.log = { _, _ in }
        }

        await store.send(.onAppear)

        await store.receive(\.loadEvents) {
            $0.isLoading = true
        }

        await store.receive(\.eventsLoaded.failure) {
            $0.isLoading = false
            $0.errorMessage =
                "The operation couldn’t be completed. (EventListFeatureTests.MockError error 1.)"  // Default description
            $0.events = []
        }
    }
}

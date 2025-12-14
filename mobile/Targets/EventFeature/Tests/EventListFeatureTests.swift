import ComposableArchitecture
import XCTest

@testable import AppSupportClients
@testable import EventFeature
@testable import MusicRoomAPI
@testable import MusicRoomDomain

@MainActor
final class EventListFeatureTests: XCTestCase {

    // Moved mockEvents inside tests to avoid non-Sendable capture

    func testOnlineLoad_SavesToCache() async {
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
            $0.networkMonitor.start = {
                AsyncStream {
                    $0.yield(.satisfied)
                    $0.finish()
                }
            }
            $0.persistence.saveEvents = { _ in }
            $0.persistence.loadEvents = { [] }
        }

        await store.send(.onAppear)
        // Also .loadEvents triggered by onAppear
        await store.receive(.loadEvents) {
            $0.isLoading = true
        }

        await store.receive(\.eventsLoaded.success) {
            $0.isLoading = false
            $0.events = events
        }

        // Network monitor yields satisfied
        await store.receive(.networkStatusChanged(.satisfied))
    }

    func testOfflineLoad_ReadsFromCache() async {
        let events = [
            Event(
                id: UUID(),
                name: "Cached Party",
                visibility: .publicEvent,
                ownerId: "u1",
                licenseMode: .everyone,
                createdAt: Date(),
                updatedAt: Date()
            )
        ]

        struct MockError: Error {}

        var state = EventListFeature.State()
        state.isOffline = true  // Start offline

        let offlineStore = TestStore(initialState: state) {
            EventListFeature()
        } withDependencies: {
            $0.musicRoomAPI.listEvents = { fatalError("Should not call API when offline") }
            $0.telemetry.log = { _, _ in }
            $0.networkMonitor.start = {
                AsyncStream {
                    $0.yield(.unsatisfied)
                    $0.finish()
                }
            }
            $0.persistence.loadEvents = { events }
        }

        await offlineStore.send(.onAppear)

        await offlineStore.receive(.loadEvents) {
            $0.isLoading = true
            $0.errorMessage = nil
        }

        // Should load from cache
        await offlineStore.receive(\.eventsLoadedFromCache.success) {
            $0.isLoading = false
            $0.events = events
            $0.errorMessage = nil
        }

        await offlineStore.receive(.networkStatusChanged(.unsatisfied))
    }

    func testLoadEventsFailure_FallbackToCache() async {
        let events = [
            Event(
                id: UUID(),
                name: "Fallback Party",
                visibility: .publicEvent,
                ownerId: "u1",
                licenseMode: .everyone,
                createdAt: Date(),
                updatedAt: Date()
            )
        ]

        struct MockError: Error, Equatable, LocalizedError {
            var errorDescription: String? { "Mock failed" }
        }

        let store = TestStore(initialState: EventListFeature.State()) {
            EventListFeature()
        } withDependencies: {
            $0.musicRoomAPI.listEvents = { throw MockError() }
            $0.telemetry.log = { _, _ in }
            $0.networkMonitor.start = { AsyncStream { $0.finish() } }
            $0.persistence.loadEvents = { events }
        }

        await store.send(.onAppear)

        await store.receive(.loadEvents) {
            $0.isLoading = true
        }

        await store.receive(\.eventsLoaded.failure)
        // Does NOT update state, returns fallback effect

        await store.receive(\.eventsLoadedFromCache.success) {
            $0.isLoading = false
            $0.events = events
            $0.errorMessage = "Loaded from cache (Offline)"
        }
    }
}

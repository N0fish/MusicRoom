import ComposableArchitecture
import XCTest

@testable import AppSupportClients
@testable import EventFeature
@testable import MusicRoomAPI
@testable import MusicRoomDomain

@MainActor
final class EventListFeatureTests: XCTestCase {
    private struct MockError: Error, Equatable, LocalizedError {
        var errorDescription: String? { "Mock failed" }
    }

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

        await store.send(EventListFeature.Action.onAppear)
        // Also .loadEvents triggered by onAppear
        await store.receive(EventListFeature.Action.loadEvents) {
            $0.isLoading = true
        }
        await store.receive(EventListFeature.Action.fetchCurrentUser)
        await store.receive(EventListFeature.Action.startRealtimeConnection)

        await store.receive(EventListFeature.Action.currentUserLoaded(.success("user1"))) {
            $0.currentUserId = "user1"
        }

        await store.receive(EventListFeature.Action.eventsLoaded(.success(events))) {
            $0.isLoading = false
            $0.events = events
            $0.hasLoaded = true
        }

        // Network monitor handled by AppFeature now
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

        var state = EventListFeature.State()
        state.isOffline = true  // Start offline

        let offlineStore = TestStore(initialState: state) {
            EventListFeature()
        } withDependencies: {
            $0.musicRoomAPI.listEvents = { fatalError("Should not call API when offline") }
            $0.telemetry.log = { _, _ in }
            $0.networkMonitor.start = { AsyncStream.never }
            $0.persistence.loadEvents = { events }
        }

        await offlineStore.send(EventListFeature.Action.onAppear)

        await offlineStore.receive(EventListFeature.Action.loadEvents) {
            $0.isLoading = true
            $0.errorMessage = nil
        }
        await offlineStore.receive(EventListFeature.Action.fetchCurrentUser)
        await offlineStore.receive(EventListFeature.Action.startRealtimeConnection)

        // Should load from cache first due to order? Or concurrent?
        // Error log showed eventsLoadedFromCache came BEFORE currentUserLoaded
        // BUT wait, onAppear -> loadEvents (cache) AND fetchCurrentUser.
        // Cache load is usually faster than network, but here persistence is mocked to return immediately?
        // And fetchCurrentUser is mocked immediately?
        // Wait, currentUserLoaded is async result of fetchCurrentUser.
        // eventsLoadedFromCache is result of loadEvents (if offline/failed).

        // Assert in observed order:
        await offlineStore.receive(EventListFeature.Action.eventsLoadedFromCache(.success(events)))
        {
            $0.isLoading = false
            $0.events = events
            $0.errorMessage = nil
            $0.hasLoaded = true
        }

        await offlineStore.receive(EventListFeature.Action.currentUserLoaded(.success("user1"))) {
            $0.currentUserId = "user1"
        }
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

        let store = TestStore(initialState: EventListFeature.State()) {
            EventListFeature()
        } withDependencies: {
            $0.musicRoomAPI.listEvents = { throw MockError() }
            $0.telemetry.log = { _, _ in }
            $0.networkMonitor.start = { AsyncStream.never }
            $0.persistence.loadEvents = { events }
        }

        await store.send(EventListFeature.Action.onAppear)

        await store.receive(EventListFeature.Action.loadEvents) {
            $0.isLoading = true
        }
        await store.receive(EventListFeature.Action.fetchCurrentUser)
        await store.receive(EventListFeature.Action.startRealtimeConnection)

        await store.receive(EventListFeature.Action.currentUserLoaded(.success("user1"))) {
            $0.currentUserId = "user1"
        }

        await store.receive(
            EventListFeature.Action.eventsLoaded(
                .failure(MockError())))
        // Does NOT update state, returns fallback effect

        await store.receive(EventListFeature.Action.eventsLoadedFromCache(.success(events))) {
            $0.isLoading = false
            $0.events = events
            $0.errorMessage = "Loaded from cache (API Failed)"
            $0.hasLoaded = true
        }
    }

    func testRealtimeInvite_ReloadsEvents() async {
        let (stream, continuation) = AsyncStream.makeStream(of: RealtimeMessage.self)
        let userId = "user123"
        let event = Event(
            id: UUID(),
            name: "Initial Party",
            visibility: .publicEvent,
            ownerId: "other",
            licenseMode: .everyone,
            createdAt: Date(),
            updatedAt: Date()
        )

        let store = TestStore(initialState: EventListFeature.State()) {
            EventListFeature()
        } withDependencies: {
            $0.musicRoomAPI.authMe = {
                .init(
                    userId: userId, email: "x", emailVerified: true, isPremium: false,
                    linkedProviders: [])
            }
            $0.musicRoomAPI.connectToRealtime = { stream }
            $0.musicRoomAPI.listEvents = { [event] }
            $0.telemetry.log = { _, _ in }
            $0.networkMonitor.start = { AsyncStream.never }
            $0.persistence.saveEvents = { _ in }
            $0.persistence.loadEvents = { [] }
        }

        await store.send(EventListFeature.Action.onAppear)

        // Parallel effects
        await store.receive(EventListFeature.Action.loadEvents) { $0.isLoading = true }
        await store.receive(EventListFeature.Action.fetchCurrentUser)
        await store.receive(EventListFeature.Action.startRealtimeConnection)

        await store.receive(EventListFeature.Action.currentUserLoaded(.success(userId))) {
            $0.currentUserId = userId
        }

        await store.receive(EventListFeature.Action.eventsLoaded(.success([event]))) {
            $0.isLoading = false
            $0.events = [event]
            $0.hasLoaded = true
        }

        // Simulate invitation
        let payload: [String: Any] = ["playlistId": "new_event_id", "userId": userId]
        let message = RealtimeMessage(
            type: "playlist.invited",
            payload: AnyDecodable(payload)
        )

        continuation.yield(message)

        await store.receive(EventListFeature.Action.realtimeMessageReceived(message))

        // Trigger reload
        await store.receive(EventListFeature.Action.loadEvents) {
            $0.isLoading = true
            $0.errorMessage = nil
        }

        await store.receive(EventListFeature.Action.eventsLoaded(.success([event]))) {
            $0.isLoading = false
            $0.hasLoaded = true
            // events didn't change in this mock test because listEvents mocked to return same
        }

        await store.send(EventListFeature.Action.onDisappear)
        continuation.finish()
    }

    func testRealtimeCreation_ReloadsEvents() async {
        let (stream, continuation) = AsyncStream.makeStream(of: RealtimeMessage.self)
        let userId = "user123"
        // Setup initial event list
        let event = Event(
            id: UUID(),
            name: "Initial Party",
            visibility: .publicEvent,
            ownerId: "other",
            licenseMode: .everyone,
            createdAt: Date(),
            updatedAt: Date()
        )

        let store = TestStore(initialState: EventListFeature.State()) {
            EventListFeature()
        } withDependencies: {
            $0.musicRoomAPI.authMe = {
                .init(
                    userId: userId, email: "x", emailVerified: true, isPremium: false,
                    linkedProviders: [])
            }
            $0.musicRoomAPI.connectToRealtime = { stream }
            $0.musicRoomAPI.listEvents = { [event] }
            $0.telemetry.log = { _, _ in }
            $0.networkMonitor.start = { AsyncStream.never }
            $0.persistence.saveEvents = { _ in }
            $0.persistence.loadEvents = { [] }
        }

        await store.send(EventListFeature.Action.onAppear)

        await store.receive(EventListFeature.Action.loadEvents) { $0.isLoading = true }
        await store.receive(EventListFeature.Action.fetchCurrentUser)
        await store.receive(EventListFeature.Action.startRealtimeConnection)

        await store.receive(EventListFeature.Action.currentUserLoaded(.success(userId))) {
            $0.currentUserId = userId
        }
        await store.receive(EventListFeature.Action.eventsLoaded(.success([event]))) {
            $0.isLoading = false
            $0.events = [event]
            $0.hasLoaded = true
        }

        // Simulate Creation (My Event on other device)
        let playlistPayload: [String: Any] = [
            "id": UUID().uuidString,
            "ownerId": userId,
            "name": "My New Event",
            "description": "Desc",
            "isPublic": false,
        ]
        let payload: [String: Any] = ["playlist": playlistPayload]
        let message = RealtimeMessage(
            type: "playlist.created",
            payload: AnyDecodable(payload)
        )

        continuation.yield(message)
        await store.receive(EventListFeature.Action.realtimeMessageReceived(message))

        // Trigger reload
        await store.receive(EventListFeature.Action.loadEvents) {
            $0.isLoading = true
            $0.errorMessage = nil
        }
        await store.receive(EventListFeature.Action.eventsLoaded(.success([event]))) {
            $0.isLoading = false
            $0.events = [event]
            $0.hasLoaded = true
        }

        await store.send(EventListFeature.Action.onDisappear)
        continuation.finish()
    }
}

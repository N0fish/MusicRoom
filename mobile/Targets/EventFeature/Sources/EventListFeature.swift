import AppSupportClients
import ComposableArchitecture
import Foundation
import MusicRoomAPI
import MusicRoomDomain

@Reducer
public struct EventListFeature: Sendable {
    @ObservableState
    public struct State: Equatable, Sendable {
        public var events: [Event] = []
        public var isLoading: Bool = false
        public var errorMessage: String?
        public var path = StackState<EventDetailFeature.State>()
        @Presents public var createEvent: CreateEventFeature.State?

        public var isOffline: Bool = false
        public var currentUserId: String?

        public init() {}
    }

    public enum Action: Equatable, Sendable {
        case onAppear
        case loadEvents
        case eventsLoaded(Result<[Event], Error>)
        case eventsLoadedFromCache(Result<[Event], Error>)  // Distinct action for cache
        case eventTapped(Event)
        case createEventButtonTapped
        case retryButtonTapped
        case createEvent(PresentationAction<CreateEventFeature.Action>)
        case path(StackAction<EventDetailFeature.State, EventDetailFeature.Action>)
        case networkStatusChanged(NetworkStatus)
        case delegate(Delegate)
        case fetchCurrentUser
        case currentUserLoaded(Result<String, Error>)
        case startRealtimeConnection
        case realtimeMessageReceived(RealtimeMessage)
        case deleteEvent(Event)
        case eventDeleted(Result<String, Error>)

        public enum Delegate: Equatable, Sendable {
            case sessionExpired
        }
    }

    private enum CancelID { case realtime }

    // Dependencies
    @Dependency(\.musicRoomAPI) var musicRoomAPI
    @Dependency(\.telemetry) var telemetry
    @Dependency(\.persistence) var persistence
    @Dependency(\.networkMonitor) var networkMonitor

    public init() {}

    public var body: some ReducerOf<Self> {
        Reduce { state, action in
            switch action {
            case .onAppear:
                // Start network monitor
                return .merge(
                    .send(.loadEvents),
                    .send(.fetchCurrentUser),
                    .send(.startRealtimeConnection)
                )

            case .networkStatusChanged(let status):
                state.isOffline = (status == .unsatisfied || status == .requiresConnection)
                if !state.isOffline && state.events.isEmpty {
                    return .send(.loadEvents)  // Auto-retry when coming back online
                }
                return .none

            case .retryButtonTapped:
                return .send(.loadEvents)

            case .loadEvents:
                state.isLoading = true
                state.errorMessage = nil

                if state.isOffline {
                    // Load from cache immediately
                    return .run { send in
                        do {
                            let events = try await persistence.loadEvents()
                            await send(.eventsLoadedFromCache(.success(events)))
                        } catch {
                            await send(.eventsLoadedFromCache(.failure(error)))
                        }
                    }
                }

                return .run { send in
                    await telemetry.log("Fetching Events", [:])
                    do {
                        let events = try await musicRoomAPI.listEvents()
                        // Save to cache on success
                        try? await persistence.saveEvents(events)
                        await send(.eventsLoaded(.success(events)))
                    } catch {
                        // Fallback to cache on error
                        await send(.eventsLoaded(.failure(error)))
                    }
                }

            case .eventsLoaded(.success(let events)):
                state.isLoading = false
                state.events = events
                state.errorMessage = nil
                return .none

            case .eventsLoaded(.failure(let error)):
                // Check for session expired
                if let apiError = error as? MusicRoomAPIError, apiError == .sessionExpired {
                    return .send(.delegate(.sessionExpired))
                }

                // Try loading from cache as fallback
                return .run { send in
                    await telemetry.log(
                        "Fetch Events Failed, trying cache", ["Error": error.localizedDescription])
                    do {
                        let events = try await persistence.loadEvents()
                        await send(.eventsLoadedFromCache(.success(events)))
                    } catch {
                        // Both failed
                        await send(.eventsLoadedFromCache(.failure(error)))
                    }
                }

            case .eventsLoadedFromCache(.success(let events)):
                state.isLoading = false
                state.events = events
                if state.isOffline {
                    state.errorMessage = nil  // clean UI in offline mode, Banner handles it
                } else {
                    state.errorMessage = "Loaded from cache (API Failed)"
                }
                return .none

            case .eventsLoadedFromCache(.failure(let error)):
                state.isLoading = false
                state.errorMessage = "Failed to load events: \(error.localizedDescription)"
                state.events = []
                return .none

            case .eventTapped(let event):
                state.path.append(EventDetailFeature.State(event: event))
                return .run { _ in
                    await telemetry.log("Event Tapped", ["EventID": event.id.uuidString])
                }

            case .createEventButtonTapped:
                state.createEvent = CreateEventFeature.State()
                return .none

            case .createEvent(.presented(.createResponse(.success(let event)))):
                state.createEvent = nil  // Dismiss on success
                state.events.append(event)  // Optimistic add or full reload
                return .none

            case .createEvent:
                return .none

            case .path(.element(id: _, action: .delegate(.sessionExpired))):
                return .send(.delegate(.sessionExpired))

            case .delegate:
                return .none

            case .path:
                return .none

            case .fetchCurrentUser:
                return .run { send in
                    do {
                        let response = try await musicRoomAPI.authMe()
                        await send(.currentUserLoaded(.success(response.userId)))
                    } catch {
                        await send(.currentUserLoaded(.failure(error)))
                    }
                }

            case .currentUserLoaded(.success(let userId)):
                state.currentUserId = userId
                return .none

            case .currentUserLoaded(.failure):
                // Silently fail, maybe retry later or rely on existing auth flows
                return .none

            case .startRealtimeConnection:
                return .run { send in
                    for await message in musicRoomAPI.connectToRealtime() {
                        await send(.realtimeMessageReceived(message))
                    }
                }
                .cancellable(id: CancelID.realtime, cancelInFlight: true)

            case .realtimeMessageReceived(let message):
                guard let currentUserId = state.currentUserId else { return .none }

                if message.type == "playlist.invited" {
                    if let payload = try? JSONDecoder().decode(
                        PlaylistInvitedPayload.self, from: JSONEncoder().encode(message.payload))
                    {
                        if payload.userId == currentUserId {
                            return .send(.loadEvents)
                        }
                    }
                } else if message.type == "playlist.created" {
                    if let payload = try? JSONDecoder().decode(
                        PlaylistCreatedPayload.self, from: JSONEncoder().encode(message.payload))
                    {
                        if payload.playlist.ownerId == currentUserId || payload.playlist.isPublic {
                            return .send(.loadEvents)
                        }
                    }
                }
                return .none

            case .deleteEvent(let event):
                guard let currentUserId = state.currentUserId else { return .none }
                return .run { send in
                    do {
                        if event.ownerId == currentUserId {
                            try await musicRoomAPI.deleteEvent(event.id)
                        } else {
                            try await musicRoomAPI.leaveEvent(event.id, currentUserId)
                        }
                        await send(.eventDeleted(.success(event.id.uuidString)))
                    } catch {
                        await send(.eventDeleted(.failure(error)))
                    }
                }

            case .eventDeleted(.success(let eventId)):
                if let uuid = UUID(uuidString: eventId) {
                    state.events.removeAll { $0.id == uuid }
                }
                return .none

            case .eventDeleted(.failure(let error)):
                state.errorMessage = "Failed to remove event: \(error.localizedDescription)"
                return .none
            }
        }
        .forEach(\.path, action: \.path) {
            EventDetailFeature()
        }
        .ifLet(\.$createEvent, action: \.createEvent) {
            CreateEventFeature()
        }
    }
}

// Error conformances for Actions
extension EventListFeature.Action {
    public static func == (lhs: EventListFeature.Action, rhs: EventListFeature.Action) -> Bool {
        switch (lhs, rhs) {
        case (.onAppear, .onAppear),
            (.loadEvents, .loadEvents),
            (.createEventButtonTapped, .createEventButtonTapped),
            (.retryButtonTapped, .retryButtonTapped):
            return true
        case (.eventsLoaded(let lhsResult), .eventsLoaded(let rhsResult)):
            switch (lhsResult, rhsResult) {
            case (.success(let lhsEvents), .success(let rhsEvents)):
                return lhsEvents == rhsEvents
            case (.failure(let lhsError), .failure(let rhsError)):
                return lhsError.localizedDescription == rhsError.localizedDescription
            default:
                return false
            }
        case (.eventTapped(let lhsEvent), .eventTapped(let rhsEvent)):
            return lhsEvent == rhsEvent
        case (.createEvent(let lhsAction), .createEvent(let rhsAction)):
            return lhsAction == rhsAction
        case (.eventsLoadedFromCache(let lhsResult), .eventsLoadedFromCache(let rhsResult)):
            switch (lhsResult, rhsResult) {
            case (.success(let lhsEvents), .success(let rhsEvents)):
                return lhsEvents == rhsEvents
            case (.failure(let lhsError), .failure(let rhsError)):
                return lhsError.localizedDescription == rhsError.localizedDescription
            default:
                return false
            }
        case (.networkStatusChanged(let lStatus), .networkStatusChanged(let rStatus)):
            return lStatus == rStatus
        case (.delegate(let lDelegate), .delegate(let rDelegate)):
            return lDelegate == rDelegate
        case (.fetchCurrentUser, .fetchCurrentUser),
            (.startRealtimeConnection, .startRealtimeConnection):
            return true
        case (.currentUserLoaded(let lResult), .currentUserLoaded(let rResult)):
            switch (lResult, rResult) {
            case (.success(let lUser), .success(let rUser)):
                return lUser == rUser
            case (.failure(let lError), .failure(let rError)):
                return lError.localizedDescription == rError.localizedDescription
            default:
                return false
            }
        case (.realtimeMessageReceived(let lMsg), .realtimeMessageReceived(let rMsg)):
            return lMsg == rMsg
        case (.deleteEvent(let lEvent), .deleteEvent(let rEvent)):
            return lEvent == rEvent
        case (.eventDeleted(let lResult), .eventDeleted(let rResult)):
            switch (lResult, rResult) {
            case (.success(let lId), .success(let rId)):
                return lId == rId
            case (.failure(let lError), .failure(let rError)):
                return lError.localizedDescription == rError.localizedDescription
            default:
                return false
            }
        default:
            return false
        }
    }
}

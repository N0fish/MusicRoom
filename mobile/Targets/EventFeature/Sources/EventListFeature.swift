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
    }

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
                    .run { send in
                        for await status in networkMonitor.start() {
                            await send(.networkStatusChanged(status))
                        }
                    },
                    .send(.loadEvents)
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
                    state.errorMessage = nil  // clean UI in offline mode
                } else {
                    state.errorMessage = "Loaded from cache (Offline)"
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

            case .path:
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
        default:
            return false
        }
    }
}

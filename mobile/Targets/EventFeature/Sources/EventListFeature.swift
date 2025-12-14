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

        public init() {}
    }

    public enum Action: Equatable, Sendable {
        case onAppear
        case loadEvents
        case eventsLoaded(Result<[Event], Error>)
        case eventTapped(Event)
        case createEventButtonTapped
        case retryButtonTapped
        case createEvent(PresentationAction<CreateEventFeature.Action>)
        case path(StackAction<EventDetailFeature.State, EventDetailFeature.Action>)
    }

    // Dependencies
    @Dependency(\.musicRoomAPI) var musicRoomAPI
    @Dependency(\.telemetry) var telemetry

    public init() {}

    public var body: some ReducerOf<Self> {
        Reduce { state, action in
            switch action {
            case .onAppear:
                // Only load if empty or if we want aggressive refresh
                if state.events.isEmpty {
                    return .send(.loadEvents)
                }
                return .none

            case .retryButtonTapped:
                return .send(.loadEvents)

            case .loadEvents:
                state.isLoading = true
                state.errorMessage = nil
                return .run { send in
                    await telemetry.log("Fetching Events", [:])
                    do {
                        let events = try await musicRoomAPI.listEvents()
                        await send(.eventsLoaded(.success(events)))
                    } catch {
                        await send(.eventsLoaded(.failure(error)))
                    }
                }

            case .eventsLoaded(.success(let events)):
                state.isLoading = false
                state.events = events
                return .none

            case .eventsLoaded(.failure(let error)):
                state.isLoading = false
                state.errorMessage = error.localizedDescription
                state.events = []  // or keep old ones? Clear for now.
                return .run { _ in
                    await telemetry.log(
                        "Fetch Events Failed", ["Error": error.localizedDescription])
                }

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
        default:
            return false
        }
    }
}

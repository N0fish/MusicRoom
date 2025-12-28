import ComposableArchitecture
import Foundation
import MusicRoomAPI
import MusicRoomDomain

@Reducer
public struct MusicSearchFeature: Sendable {
    public init() {}

    @ObservableState
    public struct State: Equatable, Sendable, Identifiable {
        public let id = UUID()
        public var query: String = ""
        public var results: [MusicSearchItem] = []
        public var isLoading: Bool = false
        public var errorMessage: String?

        public init() {}
    }

    public enum Action: BindableAction, Sendable, Equatable {
        case binding(BindingAction<State>)
        case searchQueryChanged(String)
        case search
        case searchResponse(Result<[MusicSearchItem], Error>)
        case trackTapped(MusicSearchItem)
        case delegate(Delegate)

        public enum Delegate: Equatable, Sendable {
            case trackTapped(MusicSearchItem)
        }

        public static func == (lhs: Action, rhs: Action) -> Bool {
            switch (lhs, rhs) {
            case (.binding(let l), .binding(let r)):
                return l == r
            case (.searchQueryChanged(let l), .searchQueryChanged(let r)):
                return l == r
            case (.search, .search):
                return true
            case (.searchResponse(.success(let l)), .searchResponse(.success(let r))):
                return l == r
            case (.searchResponse(.failure(let l)), .searchResponse(.failure(let r))):
                return l.localizedDescription == r.localizedDescription
            case (.trackTapped(let l), .trackTapped(let r)):
                return l == r
            case (.delegate(let l), .delegate(let r)):
                return l == r
            default:
                return false
            }
        }
    }

    @Dependency(\.musicRoomAPI) var musicRoomAPI

    private enum CancelID { case search }

    public var body: some ReducerOf<Self> {
        BindingReducer()

        Reduce { state, action in
            switch action {
            case .binding(\.query):
                // Trigger search after debounce handled in view or side effect
                // For simplicity, we can trigger an effect here with debounce
                return .none

            case .searchQueryChanged(let query):
                state.query = query
                if query.isEmpty {
                    state.results = []
                    return .cancel(id: CancelID.search)
                }
                return .run { send in
                    // Debounce logic could be here, or expected to be triggered by user "Enter"
                    // Let's implement active search on "Enter" or "Button" for now to save API calls
                }

            case .search:
                guard !state.query.isEmpty else { return .none }
                state.isLoading = true
                state.errorMessage = nil

                return .run { [query = state.query] send in
                    await send(
                        .searchResponse(
                            Result {
                                try await musicRoomAPI.search(query)
                            }))
                }
                .cancellable(id: CancelID.search)

            case .searchResponse(.success(let items)):
                state.isLoading = false
                state.results = items
                return .none

            case .searchResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = error.localizedDescription
                return .none

            case .trackTapped(let item):
                return .send(.delegate(.trackTapped(item)))

            case .delegate:
                return .none

            case .binding:
                return .none
            }
        }
    }
}

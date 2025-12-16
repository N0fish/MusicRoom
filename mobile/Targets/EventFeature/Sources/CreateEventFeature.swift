import ComposableArchitecture
import Foundation
import MusicRoomAPI
import MusicRoomDomain

@Reducer
public struct CreateEventFeature: Sendable {
    @ObservableState
    public struct State: Equatable, Sendable {
        public var name: String = ""
        public var visibility: EventVisibility = .publicEvent
        public var licenseMode: EventLicenseMode = .everyone
        public var isLoading: Bool = false
        public var errorMessage: String?

        public init() {}
    }

    public enum Action: BindableAction, Sendable, Equatable {
        case binding(BindingAction<State>)
        case createButtonTapped
        case cancelButtonTapped
        case createResponse(Result<Event, Error>)
    }

    @Dependency(\.musicRoomAPI) var musicRoomAPI
    @Dependency(\.dismiss) var dismiss

    public init() {}

    public var body: some ReducerOf<Self> {
        BindingReducer()
        Reduce { state, action in
            switch action {
            case .binding:
                return .none

            case .createButtonTapped:
                guard !state.name.isEmpty else {
                    state.errorMessage = "Event name cannot be empty."
                    return .none
                }
                state.isLoading = true
                state.errorMessage = nil

                return .run {
                    [
                        name = state.name, visibility = state.visibility,
                        licenseMode = state.licenseMode
                    ] send in
                    let request = CreateEventRequest(
                        name: name,
                        visibility: visibility,
                        licenseMode: licenseMode
                    )
                    await send(
                        .createResponse(
                            Result {
                                try await musicRoomAPI.createEvent(request)
                            }))
                }

            case .createResponse(.success):
                state.isLoading = false
                return .run { _ in await dismiss() }

            case .createResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = error.localizedDescription
                state.errorMessage = error.localizedDescription
                return .none

            case .cancelButtonTapped:
                return .run { _ in await dismiss() }
            }
        }
    }
}

extension CreateEventFeature.Action {
    public static func == (lhs: CreateEventFeature.Action, rhs: CreateEventFeature.Action) -> Bool {
        switch (lhs, rhs) {
        case (.binding(let l), .binding(let r)):
            return l == r
        case (.createButtonTapped, .createButtonTapped):
            return true
        case (.cancelButtonTapped, .cancelButtonTapped):
            return true
        case (.createResponse(.success(let l)), .createResponse(.success(let r))):
            return l == r
        case (.createResponse(.failure(let l)), .createResponse(.failure(let r))):
            return l.localizedDescription == r.localizedDescription
        default:
            return false
        }
    }
}

import AppSupportClients
import ComposableArchitecture
import Foundation

@Reducer
public struct ForgotPasswordFeature: Sendable {
    @ObservableState
    public struct State: Equatable {
        public var email: String = ""
        public var isLoading: Bool = false
        public var isSuccess: Bool = false
        public var errorMessage: String?

        public init() {}
    }

    public enum Action: BindableAction, Equatable {
        case binding(BindingAction<State>)
        case submitButtonTapped
        case forgotPasswordResponse(TaskResult<Bool>)
        case backButtonTapped
    }

    @Dependency(\.authentication) var authenticationClient
    @Dependency(\.dismiss) var dismiss

    public init() {}

    public var body: some ReducerOf<Self> {
        BindingReducer()
        Reduce { state, action in
            switch action {
            case .binding:
                return .none

            case .submitButtonTapped:
                guard !state.email.isEmpty else {
                    state.errorMessage = "Please enter your email."
                    return .none
                }
                state.isLoading = true
                state.errorMessage = nil
                state.isSuccess = false

                return .run { [email = state.email, authenticationClient] send in
                    await send(
                        .forgotPasswordResponse(
                            TaskResult {
                                try await authenticationClient.forgotPassword(email)
                                return true
                            }))
                }

            case .forgotPasswordResponse(.success):
                state.isLoading = false
                state.isSuccess = true
                state.errorMessage = nil
                // Optionally dismiss automatically or wait for user to go back
                return .none

            case .forgotPasswordResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = error.localizedDescription
                return .none

            case .backButtonTapped:
                return .run { [dismiss] _ in await dismiss() }
            }
        }
    }
}

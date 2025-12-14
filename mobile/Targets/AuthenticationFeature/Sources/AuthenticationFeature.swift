import AppSettingsClient
import AppSupportClients
import ComposableArchitecture
import Foundation

@Reducer
public struct AuthenticationFeature: Sendable {
    @ObservableState
    public struct State: Equatable {
        public var email = ""
        public var password = ""
        public var isRegistering = false
        public var isLoading = false
        public var errorMessage: String?
        @Presents public var forgotPassword: ForgotPasswordFeature.State?

        public init() {}
    }

    public enum Action: BindableAction, Equatable {
        case binding(BindingAction<State>)
        case toggleModeButtonTapped
        case submitButtonTapped
        case socialLoginButtonTapped(SocialProvider)
        case authResponse(Result<Bool, AuthenticationError>)
        case forgotPasswordButtonTapped
        case forgotPassword(PresentationAction<ForgotPasswordFeature.Action>)
    }

    public enum SocialProvider: String, Sendable {
        case google
        case intra42 = "42"
    }

    @Dependency(\.authentication) var authentication
    @Dependency(\.webAuthenticationSession) var webAuthenticationSession
    @Dependency(\.appSettings) var appSettings

    public init() {}

    public var body: some ReducerOf<Self> {
        BindingReducer()
        Reduce { state, action in
            switch action {
            case .binding:
                return .none

            case .toggleModeButtonTapped:
                state.isRegistering.toggle()
                state.errorMessage = nil
                return .none

            case .forgotPasswordButtonTapped:
                state.forgotPassword = ForgotPasswordFeature.State()
                return .none

            case .forgotPassword:
                return .none

            case .socialLoginButtonTapped(let provider):
                return .run {
                    [
                        appSettings = self.appSettings, webAuth = self.webAuthenticationSession,
                        authentication = self.authentication
                    ] send in
                    let settings = appSettings.load()
                    let authURL = AuthenticationClient.SocialHelper.authURL(
                        for: .init(rawValue: provider.rawValue)!, baseURL: settings.backendURL)

                    do {
                        let callbackURL = try await webAuth.authenticate(authURL, "musicroom")

                        if let tokens = AuthenticationClient.SocialHelper.parseCallback(
                            url: callbackURL)
                        {
                            await authentication.saveTokens(tokens.accessToken, tokens.refreshToken)
                            await send(.authResponse(.success(true)))
                        } else {
                            await send(.authResponse(.failure(.unknown)))
                        }
                    } catch {
                        // User likely cancelled or network error
                        await send(.authResponse(.failure(.unknown)))
                    }
                }

            case .submitButtonTapped:
                guard !state.email.isEmpty, !state.password.isEmpty else {
                    state.errorMessage = "Please fill in all fields."
                    return .none
                }

                state.isLoading = true
                state.errorMessage = nil

                return .run {
                    [
                        email = state.email, password = state.password,
                        isRegistering = state.isRegistering, authentication = self.authentication
                    ] send in
                    do {
                        if isRegistering {
                            try await authentication.register(email, password)
                        } else {
                            try await authentication.login(email, password)
                        }
                        await send(.authResponse(.success(true)))
                    } catch let error as AuthenticationError {
                        await send(.authResponse(.failure(error)))
                    } catch {
                        await send(.authResponse(.failure(.unknown)))
                    }
                }

            case .authResponse(.success):
                state.isLoading = false
                return .none  // Parent will handle navigation on success (token presence)

            case .authResponse(.failure(let error)):
                state.isLoading = false
                switch error {
                case .invalidCredentials:
                    state.errorMessage = "Invalid email or password."
                case .networkError(let message):
                    state.errorMessage = message
                case .unknown:
                    state.errorMessage = "An unknown error occurred."
                }
                return .none
            }
        }
        .ifLet(\.$forgotPassword, action: \.forgotPassword) {
            ForgotPasswordFeature()
        }
    }
}

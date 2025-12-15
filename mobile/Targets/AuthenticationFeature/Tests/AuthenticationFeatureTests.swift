import ComposableArchitecture
import XCTest

@testable import AppSettingsClient
@testable import AppSupportClients
@testable import AuthenticationFeature

@MainActor
final class AuthenticationFeatureTests: XCTestCase {

    func testLoginSuccess() async {
        let store = TestStore(initialState: AuthenticationFeature.State()) {
            AuthenticationFeature()
        } withDependencies: {
            $0.authentication.login = { _, _ in }  // Success
            $0.telemetry.log = { action, metadata in
                XCTAssertEqual(action, "user.auth.login.success")
            }
        }

        await store.send(.binding(.set(\.email, "test@example.com"))) {
            $0.email = "test@example.com"
        }
        await store.send(.binding(.set(\.password, "password"))) {
            $0.password = "password"
        }

        await store.send(.submitButtonTapped) {
            $0.isLoading = true
            $0.errorMessage = nil
        }

        await store.receive(.authResponse(.success(true))) {
            $0.isLoading = false
        }
    }

    func testLoginFailure() async {
        let store = TestStore(initialState: AuthenticationFeature.State()) {
            AuthenticationFeature()
        } withDependencies: {
            $0.authentication.login = { _, _ in throw AuthenticationError.invalidCredentials }
        }

        await store.send(.binding(.set(\.email, "wrong@example.com"))) {
            $0.email = "wrong@example.com"
        }
        await store.send(.binding(.set(\.password, "wrong"))) {
            $0.password = "wrong"
        }

        await store.send(.submitButtonTapped) {
            $0.isLoading = true
        }

        await store.receive(.authResponse(.failure(.invalidCredentials))) {
            $0.isLoading = false
            $0.errorMessage = "Invalid email or password."
        }
    }

    func testRegistrationSuccess() async {
        let store = TestStore(initialState: AuthenticationFeature.State()) {
            AuthenticationFeature()
        } withDependencies: {
            $0.authentication.register = { _, _ in }  // Success
        }

        await store.send(.toggleModeButtonTapped) {
            $0.isRegistering = true
        }

        await store.send(.binding(.set(\.email, "new@example.com"))) {
            $0.email = "new@example.com"
        }
        await store.send(.binding(.set(\.password, "password"))) {
            $0.password = "password"
        }

        await store.send(.submitButtonTapped) {
            $0.isLoading = true
        }

        await store.receive(.authResponse(.success(true))) {
            $0.isLoading = false
        }
    }

    func testValidation() async {
        let store = TestStore(initialState: AuthenticationFeature.State()) {
            AuthenticationFeature()
        }

        await store.send(.submitButtonTapped) {
            $0.errorMessage = "Please fill in all fields."
        }
    }

    func testSocialLoginSuccess() async {
        let store = TestStore(initialState: AuthenticationFeature.State()) {
            AuthenticationFeature()
        } withDependencies: {
            $0.webAuthenticationSession.authenticate = { url, scheme in
                // Assert URL is correct
                XCTAssertEqual(scheme, "musicroom")
                XCTAssertTrue(url.absoluteString.contains("/auth/google/login"))
                return URL(
                    string:
                        "musicroom://auth/callback?accessToken=mockAccess&refreshToken=mockRefresh")!
            }
            $0.authentication.saveTokens = { _, _ in }
        }

        await store.send(.socialLoginButtonTapped(.google))

        await store.receive(.authResponse(.success(true)))
    }

    func testSocialLoginCancellation() async {
        struct MockError: Error, Equatable {}

        let store = TestStore(initialState: AuthenticationFeature.State()) {
            AuthenticationFeature()
        } withDependencies: {
            $0.webAuthenticationSession.authenticate = { _, _ in
                throw MockError()
            }
        }

        await store.send(.socialLoginButtonTapped(.google))

        await store.receive(.authResponse(.failure(.unknown))) {
            $0.errorMessage = "An unknown error occurred."  // or default error handling
        }
    }
    func testRegister_UserAlreadyExists() async {
        let store = TestStore(initialState: AuthenticationFeature.State()) {
            AuthenticationFeature()
        } withDependencies: {
            $0.authentication.register = { _, _ in throw AuthenticationError.userAlreadyExists }
        }

        await store.send(.toggleModeButtonTapped) {
            $0.isRegistering = true
        }

        await store.send(.binding(.set(\.email, "exists@example.com"))) {
            $0.email = "exists@example.com"
        }
        await store.send(.binding(.set(\.password, "password"))) {
            $0.password = "password"
        }

        await store.send(.submitButtonTapped) {
            $0.isLoading = true
        }

        await store.receive(.authResponse(.failure(.userAlreadyExists))) {
            $0.isLoading = false
            $0.errorMessage = "This email is already registered."
        }
    }

    func testRegister_BadRequest() async {
        let store = TestStore(initialState: AuthenticationFeature.State()) {
            AuthenticationFeature()
        } withDependencies: {
            $0.authentication.register = { _, _ in
                throw AuthenticationError.badRequest("Password too short")
            }
        }

        await store.send(.toggleModeButtonTapped) {
            $0.isRegistering = true
        }

        await store.send(.binding(.set(\.email, "new@example.com"))) {
            $0.email = "new@example.com"
        }
        await store.send(.binding(.set(\.password, "123"))) {
            $0.password = "123"
        }

        await store.send(.submitButtonTapped) {
            $0.isLoading = true
        }

        await store.receive(.authResponse(.failure(.badRequest("Password too short")))) {
            $0.isLoading = false
            $0.errorMessage = "Password too short"
        }
    }

    func testRegister_ServerError() async {
        let store = TestStore(initialState: AuthenticationFeature.State()) {
            AuthenticationFeature()
        } withDependencies: {
            $0.authentication.register = { _, _ in
                throw AuthenticationError.serverError("Internal Error")
            }
        }

        await store.send(.toggleModeButtonTapped) {
            $0.isRegistering = true
        }

        await store.send(.binding(.set(\.email, "new@example.com"))) {
            $0.email = "new@example.com"
        }
        await store.send(.binding(.set(\.password, "password"))) {
            $0.password = "password"
        }

        await store.send(.submitButtonTapped) {
            $0.isLoading = true
        }

        await store.receive(.authResponse(.failure(.serverError("Internal Error")))) {
            $0.isLoading = false
            $0.errorMessage = "Server error. Please try again later."
        }
    }
}

import ComposableArchitecture
import XCTest

@testable import AppSupportClients
@testable import AuthenticationFeature

@MainActor
final class ForgotPasswordFeatureTests: XCTestCase {
    func testForgotPassword_Success() async {
        let store = TestStore(initialState: ForgotPasswordFeature.State()) {
            ForgotPasswordFeature()
        } withDependencies: {
            $0.authentication.forgotPassword = { _ in }
        }

        await store.send(.set(\.email, "test@example.com")) {
            $0.email = "test@example.com"
        }

        await store.send(.submitButtonTapped) {
            $0.isLoading = true
            $0.errorMessage = nil
            $0.isSuccess = false
        }

        await store.receive(\.forgotPasswordResponse.success) {
            $0.isLoading = false
            $0.isSuccess = true
            $0.errorMessage = nil
        }
    }

    func testForgotPassword_Failure() async {
        let error = AuthenticationError.networkError("Failed")
        let store = TestStore(initialState: ForgotPasswordFeature.State()) {
            ForgotPasswordFeature()
        } withDependencies: {
            $0.authentication.forgotPassword = { _ in throw error }
        }

        await store.send(.set(\.email, "test@example.com")) {
            $0.email = "test@example.com"
        }

        await store.send(.submitButtonTapped) {
            $0.isLoading = true
            $0.errorMessage = nil
            $0.isSuccess = false
        }

        await store.receive(\.forgotPasswordResponse.failure) {
            $0.isLoading = false
            $0.errorMessage = error.localizedDescription
        }
    }

    func testValidation() async {
        let store = TestStore(initialState: ForgotPasswordFeature.State()) {
            ForgotPasswordFeature()
        }

        await store.send(.submitButtonTapped) {
            $0.errorMessage = "Please enter your email."
        }
    }
}

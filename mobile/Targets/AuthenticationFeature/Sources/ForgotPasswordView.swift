import ComposableArchitecture
import MusicRoomUI
import SwiftUI

public struct ForgotPasswordView: View {
    @Bindable var store: StoreOf<ForgotPasswordFeature>

    public init(store: StoreOf<ForgotPasswordFeature>) {
        self.store = store
    }

    public var body: some View {
        ZStack {
            LiquidBackground()

            VStack(spacing: 30) {
                // Header
                VStack(spacing: 12) {
                    Image(systemName: "lock.rotation")
                        .font(.system(size: 60))
                        .foregroundStyle(
                            LinearGradient(
                                colors: [.liquidPrimary, .liquidSecondary],
                                startPoint: .topLeading,
                                endPoint: .bottomTrailing
                            )
                        )
                        .shadow(color: .liquidSecondary.opacity(0.6), radius: 25, x: 0, y: 10)

                    Text("Account Recovery")
                        .font(.liquidTitle)
                        .foregroundColor(.white)
                }
                .padding(.top, 40)

                // Form
                GlassView(cornerRadius: 32) {
                    VStack(spacing: 24) {
                        Text("Enter your email to receive recovery instructions.")
                            .font(.liquidBody)
                            .foregroundColor(.white.opacity(0.8))
                            .multilineTextAlignment(.center)
                            .padding(.horizontal)

                        // Email Field
                        HStack {
                            Image(systemName: "envelope.fill")
                                .foregroundColor(.liquidPrimary)
                                .frame(width: 24)
                            TextField("Email Address", text: $store.email)
                                .textContentType(.emailAddress)
                                .keyboardType(.emailAddress)
                                .autocapitalization(.none)
                                .foregroundColor(.white)
                                .font(.liquidBody)
                        }
                        .padding()
                        .background(Color.black.opacity(0.3))
                        .cornerRadius(16)
                        .overlay(
                            RoundedRectangle(cornerRadius: 16)
                                .stroke(Color.liquidPrimary.opacity(0.5), lineWidth: 1)
                        )

                        if let error = store.errorMessage {
                            Text(error)
                                .foregroundColor(.red)
                                .font(.liquidCaption)
                                .multilineTextAlignment(.center)
                        }

                        if store.isSuccess {
                            Text(
                                "If an account exists for this email, you will receive recovery instructions shortly."
                            )
                            .foregroundColor(.green)
                            .font(.liquidBody)
                            .multilineTextAlignment(.center)
                        }

                        Button(action: {
                            store.send(.submitButtonTapped)
                        }) {
                            ZStack {
                                if store.isLoading {
                                    ProgressView()
                                        .progressViewStyle(CircularProgressViewStyle(tint: .white))
                                } else {
                                    Text("SEND INSTRUCTIONS")
                                        .font(.liquidButton)
                                        .foregroundColor(.white)
                                }
                            }
                            .frame(maxWidth: .infinity)
                            .padding()
                            .background(
                                LinearGradient(
                                    colors: [.liquidSecondary, .liquidPrimary],
                                    startPoint: .leading,
                                    endPoint: .trailing
                                )
                            )
                            .cornerRadius(16)
                            .shadow(color: .liquidSecondary.opacity(0.5), radius: 15, x: 0, y: 5)
                        }
                        .disabled(store.isLoading || store.isSuccess)
                    }
                    .padding(24)
                }
                .padding(.horizontal, 24)

                Spacer()

                Button(action: {
                    store.send(.backButtonTapped)
                }) {
                    Text("Back to Login")
                        .font(.liquidBody)
                        .foregroundColor(.white.opacity(0.8))
                }
                .padding(.bottom, 40)
            }
        }
        .navigationBarHidden(true)
    }
}

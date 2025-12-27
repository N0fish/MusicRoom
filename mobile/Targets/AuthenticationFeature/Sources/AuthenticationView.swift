import ComposableArchitecture
import MusicRoomUI
import SwiftUI

public struct AuthenticationView: View {
    @Bindable var store: StoreOf<AuthenticationFeature>
    @State private var start = UnitPoint(x: 0, y: -2)
    @State private var end = UnitPoint(x: 4, y: 0)
    @FocusState private var focusedField: Field?

    private enum Field {
        case email
        case password
    }

    public init(store: StoreOf<AuthenticationFeature>) {
        self.store = store
    }

    public var body: some View {
        ZStack {
            // New "Liquid Glass" Background
            LiquidBackground()

            ScrollView {
                scrollViewContent
            }
            .scrollIndicators(.hidden)
        }
        .onAppear {
            focusedField = .email
        }
    }

    private var scrollViewContent: some View {
        VStack(spacing: 30) {
            // MARK: - Logo / Title
            VStack(spacing: 12) {
                Image(systemName: "music.note.house.fill")
                    .font(.system(size: 80))
                    .symbolEffect(.bounce, value: store.isRegistering)  // iOS 17+ animation
                    .foregroundStyle(
                        LinearGradient(
                            colors: [.liquidPrimary, .liquidSecondary],
                            startPoint: .topLeading,
                            endPoint: .bottomTrailing
                        )
                    )
                    .shadow(color: .liquidSecondary.opacity(0.6), radius: 25, x: 0, y: 10)

                Text("Music Room")
                    .font(.liquidTitle)
                    .foregroundColor(.white)
                    .shadow(color: .liquidPrimary.opacity(0.5), radius: 10)
            }
            .padding(.top, 60)

            // MARK: - Form Container (Glass)
            GlassView(cornerRadius: 32) {
                VStack(spacing: 24) {
                    Text(store.isRegistering ? "Create Account" : "Access")
                        .font(.liquidH2)
                        .foregroundColor(.white)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .shadow(radius: 5)

                    // Email Field
                    HStack {
                        Image(systemName: "envelope.fill")
                            .foregroundColor(.liquidPrimary)
                            .frame(width: 24)
                        ZStack(alignment: .leading) {
                            if store.email.isEmpty {
                                Text("Email Credentials")
                                    .font(.liquidBody)
                                    .foregroundColor(.white.opacity(0.6))
                            }
                            TextField("", text: $store.email)
                                .focused($focusedField, equals: .email)
                                .textContentType(.emailAddress)
                                .keyboardType(.emailAddress)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled(true)
                                .submitLabel(.next)
                                .foregroundColor(.white)
                                .font(.liquidBody)
                                .onSubmit {
                                    focusedField = .password
                                }
                        }
                    }
                    .padding()
                    .background(Color.black.opacity(0.3))
                    .cornerRadius(16)
                    .overlay(
                        RoundedRectangle(cornerRadius: 16)
                            .stroke(
                                Color.liquidPrimary.opacity(
                                    store.email.isEmpty ? 0.3 : 0.8), lineWidth: 1)
                    )
                    .onTapGesture {
                        focusedField = .email
                    }

                    // Password Field
                    HStack {
                        Image(systemName: "lock.fill")
                            .foregroundColor(.liquidAccent)
                            .frame(width: 24)
                        ZStack(alignment: .leading) {
                            if store.password.isEmpty {
                                Text("Passcode")
                                    .font(.liquidBody)
                                    .foregroundColor(.white.opacity(0.6))
                            }
                            SecureField("", text: $store.password)
                                .focused($focusedField, equals: .password)
                                .textContentType(store.isRegistering ? .newPassword : .password)
                                .autocorrectionDisabled(true)
                                .submitLabel(.go)
                                .foregroundColor(.white)
                                .font(.liquidBody)
                                .onSubmit {
                                    store.send(.submitButtonTapped)
                                }
                        }
                    }
                    .padding()
                    .background(Color.black.opacity(0.3))
                    .cornerRadius(16)
                    .overlay(
                        RoundedRectangle(cornerRadius: 16)
                            .stroke(
                                Color.liquidAccent.opacity(
                                    store.password.isEmpty ? 0.3 : 0.8), lineWidth: 1)
                    )
                    .onTapGesture {
                        focusedField = .password
                    }

                    // Forgot Password Button
                    if !store.isRegistering {
                        Button(action: {
                            store.send(.forgotPasswordButtonTapped)
                        }) {
                            Text("Forgot Password?")
                                .font(.liquidCaption)
                                .foregroundColor(.white.opacity(0.8))
                                .frame(maxWidth: .infinity, alignment: .trailing)
                        }
                    }

                    if let error = store.errorMessage {
                        Text(error)
                            .foregroundColor(.red)
                            .font(.liquidCaption)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .transition(.opacity)
                    }

                    // Submit Button
                    Button(action: {
                        store.send(.submitButtonTapped)
                    }) {
                        ZStack {
                            if store.isLoading {
                                ProgressView()
                                    .progressViewStyle(
                                        CircularProgressViewStyle(tint: .white))
                            } else {
                                Text(store.isRegistering ? "REGISTER" : "CONNECT")
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
                        .shadow(
                            color: .liquidSecondary.opacity(0.5), radius: 15, x: 0, y: 5)
                    }
                    .disabled(store.isLoading)
                }
                .padding(24)
            }
            .padding(.horizontal, 24)

            // MARK: - Social Login
            if !store.isLoading {
                VStack(spacing: 16) {
                    HStack {
                        Rectangle().frame(height: 1).foregroundColor(.glassBorder)
                        Text("OR LINK IDENTITY").font(.liquidCaption).foregroundColor(
                            .white.opacity(0.6))
                        Rectangle().frame(height: 1).foregroundColor(.glassBorder)
                    }
                    .padding(.horizontal, 40)

                    HStack(spacing: 16) {
                        // Google
                        Button(action: {
                            store.send(.socialLoginButtonTapped(.google))
                        }) {
                            Image(systemName: "globe")
                                .font(.system(size: 24))
                                .foregroundColor(.white)
                                .frame(width: 60, height: 60)
                                .background(Color.white.opacity(0.1))
                                .clipShape(Circle())
                                .overlay(Circle().stroke(Color.glassBorder, lineWidth: 1))
                        }

                        // 42
                        Button(action: {
                            store.send(.socialLoginButtonTapped(.intra42))
                        }) {
                            Image(systemName: "building.2.fill")
                                .font(.system(size: 24))
                                .foregroundColor(.white)
                                .frame(width: 60, height: 60)
                                .background(Color.white.opacity(0.1))
                                .clipShape(Circle())
                                .overlay(Circle().stroke(Color.glassBorder, lineWidth: 1))
                        }
                    }
                }
            }

            // MARK: - Toggle Mode
            Button(action: {
                _ = withAnimation(.spring()) {
                    store.send(.toggleModeButtonTapped)
                }
            }) {
                Text(
                    store.isRegistering
                        ? "Existing User? Connect"
                        : "New Identity? Register"
                )
                .font(.liquidCaption)
                .foregroundColor(.liquidPrimary)
                .padding(.bottom, 40)
            }
        }
        .fullScreenCover(item: $store.scope(state: \.forgotPassword, action: \.forgotPassword)) {
            store in
            ForgotPasswordView(store: store)
        }
    }
}

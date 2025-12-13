import ComposableArchitecture
import SwiftUI

public struct AuthenticationView: View {
    @Bindable var store: StoreOf<AuthenticationFeature>

    public init(store: StoreOf<AuthenticationFeature>) {
        self.store = store
    }

    public var body: some View {
        ZStack {
            // Background Gradient
            LinearGradient(
                gradient: Gradient(colors: [Color(hex: "1A1A2E"), Color(hex: "16213E")]),
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            .ignoresSafeArea()

            ScrollView {
                VStack(spacing: 30) {
                    // Logo / Title
                    VStack(spacing: 10) {
                        Image(systemName: "music.note.house.fill")
                            .font(.system(size: 60))
                            .foregroundStyle(
                                LinearGradient(
                                    colors: [.pink, .purple],
                                    startPoint: .topLeading,
                                    endPoint: .bottomTrailing
                                )
                            )
                            .shadow(color: .purple.opacity(0.5), radius: 10, x: 0, y: 5)

                        Text("Music Room")
                            .font(.system(size: 32, weight: .bold, design: .rounded))
                            .foregroundColor(.white)
                            .shadow(radius: 2)
                    }
                    .padding(.top, 50)

                    // Form Container
                    VStack(spacing: 20) {
                        Text(store.isRegistering ? "Create Account" : "Welcome Back")
                            .font(.title2)
                            .fontWeight(.semibold)
                            .foregroundColor(.white.opacity(0.9))
                            .frame(maxWidth: .infinity, alignment: .leading)

                        // Email Field
                        HStack {
                            Image(systemName: "envelope.fill")
                                .foregroundColor(.gray)
                            TextField("Email", text: $store.email)
                                .textContentType(.emailAddress)
                                .keyboardType(.emailAddress)
                                .autocapitalization(.none)
                                .foregroundColor(.white)
                        }
                        .padding()
                        .background(Color.white.opacity(0.1))
                        .cornerRadius(12)
                        .overlay(
                            RoundedRectangle(cornerRadius: 12)
                                .stroke(Color.white.opacity(0.2), lineWidth: 1)
                        )

                        // Password Field
                        HStack {
                            Image(systemName: "lock.fill")
                                .foregroundColor(.gray)
                            SecureField("Password", text: $store.password)
                                .textContentType(store.isRegistering ? .newPassword : .password)
                                .foregroundColor(.white)
                        }
                        .padding()
                        .background(Color.white.opacity(0.1))
                        .cornerRadius(12)
                        .overlay(
                            RoundedRectangle(cornerRadius: 12)
                                .stroke(Color.white.opacity(0.2), lineWidth: 1)
                        )

                        if let error = store.errorMessage {
                            Text(error)
                                .foregroundColor(.red)
                                .font(.caption)
                                .frame(maxWidth: .infinity, alignment: .leading)
                                .transition(.opacity)
                        }

                        // Submit Button
                        Button(action: {
                            store.send(.submitButtonTapped)
                        }) {
                            if store.isLoading {
                                ProgressView()
                                    .progressViewStyle(CircularProgressViewStyle(tint: .white))
                            } else {
                                Text(store.isRegistering ? "Sign Up" : "Log In")
                                    .fontWeight(.bold)
                                    .foregroundColor(.white)
                                    .frame(maxWidth: .infinity)
                            }
                        }
                        .padding()
                        .background(
                            LinearGradient(
                                colors: [.pink, .purple],
                                startPoint: .leading,
                                endPoint: .trailing
                            )
                        )
                        .cornerRadius(12)
                        .shadow(color: .purple.opacity(0.4), radius: 8, x: 0, y: 4)
                        .disabled(store.isLoading)
                    }
                    .padding(24)
                    .background(
                        VisualEffectBlur(blurStyle: .systemUltraThinMaterialDark)
                            .cornerRadius(24)
                    )
                    .padding(.horizontal)

                    // Toggle Mode Button
                    Button(action: {
                        store.send(.toggleModeButtonTapped)
                    }) {
                        Text(
                            store.isRegistering
                                ? "Already have an account? Log In"
                                : "Don't have an account? Sign Up"
                        )
                        .font(.subheadline)
                        .foregroundColor(.white.opacity(0.7))
                    }

                    Spacer()
                }
                .padding(.bottom, 20)
            }
            .scrollIndicators(.hidden)
        }
    }
}

// Helper for Blur Effect
struct VisualEffectBlur: UIViewRepresentable {
    var blurStyle: UIBlurEffect.Style

    func makeUIView(context: Context) -> UIVisualEffectView {
        return UIVisualEffectView(effect: UIBlurEffect(style: blurStyle))
    }

    func updateUIView(_ uiView: UIVisualEffectView, context: Context) {
        uiView.effect = UIBlurEffect(style: blurStyle)
    }
}

// Helper for Hex Color
extension Color {
    init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a: UInt64
        let r: UInt64
        let g: UInt64
        let b: UInt64
        switch hex.count {
        case 3:  // RGB (12-bit)
            (a, r, g, b) = (255, (int >> 8) * 17, (int >> 4 & 0xF) * 17, (int & 0xF) * 17)
        case 6:  // RGB (24-bit)
            (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)
        case 8:  // ARGB (32-bit)
            (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)
        default:
            (a, r, g, b) = (1, 1, 1, 0)
        }

        self.init(
            .sRGB,
            red: Double(r) / 255,
            green: Double(g) / 255,
            blue: Double(b) / 255,
            opacity: Double(a) / 255
        )
    }
}

import AppSupportClients
import ComposableArchitecture
import ImagePlayground
import MusicRoomAPI
import MusicRoomUI
import SwiftUI

public struct ProfileView: View {
    @Bindable var store: StoreOf<ProfileFeature>
    @Environment(\.supportsImagePlayground) var supportsImagePlayground

    public init(store: StoreOf<ProfileFeature>) {
        self.store = store
    }

    public var body: some View {
        // NavigationStack removed as it is provided by AppView
        ZStack {
            LiquidBackground()
                .ignoresSafeArea()

            if store.isLoading && !store.isAvatarLoading {
                // Do not show full screen loader if only avatar is loading (randomize)
                ProgressView()
                    .tint(.white)
            } else if let errorMessage = store.errorMessage {
                VStack {
                    Image(systemName: "exclamationmark.triangle.fill")
                        .foregroundStyle(.yellow)
                        .font(.largeTitle)
                    Text(errorMessage)
                        .foregroundStyle(.white)
                        .multilineTextAlignment(.center)
                }
                .padding()
            } else if let profile = store.userProfile {
                ScrollView {
                    VStack(spacing: 24) {
                        // Header
                        VStack(spacing: 16) {
                            if store.isAvatarLoading {
                                ShimmerView()
                                    .frame(width: 120, height: 120)
                                    .clipShape(Circle())
                            } else {
                                PremiumAvatarView(
                                    url: profile.avatarUrl,
                                    isPremium: profile.isPremium,
                                    size: 120
                                )
                            }

                            if store.isEditing {
                                Button(action: { store.send(.generateRandomAvatarTapped) }) {
                                    HStack(spacing: 4) {
                                        if store.isAvatarLoading {
                                            ProgressView().controlSize(.mini).tint(.white)
                                        } else {
                                            Image(systemName: "dice.fill")
                                        }
                                        Text("Randomize")
                                    }
                                    .font(.caption)
                                    .padding(.horizontal, 12)
                                    .padding(.vertical, 6)
                                    .background(Color.blue)
                                    .foregroundColor(.white)
                                    .clipShape(Capsule())
                                }
                                .disabled(store.isAvatarLoading)
                                .padding(.top, 4)

                                if profile.isPremium && supportsImagePlayground {
                                    Button(action: { store.send(.toggleImagePlayground(true)) }) {
                                        HStack(spacing: 4) {
                                            Image(systemName: "sparkles")
                                            Text("Generate with AI")
                                        }
                                        .font(.caption)
                                        .padding(.horizontal, 12)
                                        .padding(.vertical, 6)
                                        .background(Color.purple)
                                        .foregroundColor(.white)
                                        .clipShape(Capsule())
                                    }
                                    .disabled(
                                        store.isAvatarLoading || store.isImagePlaygroundPresented
                                    )
                                    .padding(.top, 4)
                                }
                            }

                            VStack(spacing: 4) {
                                Text(profile.displayName)
                                    .font(.liquidH2)
                                    .foregroundStyle(.white)

                                Text("@\(profile.username)")
                                    .font(.liquidBody)
                                    .foregroundStyle(.white.opacity(0.6))
                            }

                        }
                        .padding(.top, 20)

                        // Stats Grid
                        if let stats = store.userStats {
                            HStack(spacing: 12) {
                                statCard(
                                    title: "Events Hosted", value: "\(stats.eventsHosted)",
                                    icon: "music.note.house.fill")
                                statCard(
                                    title: "Votes Cast", value: "\(stats.votesCast)",
                                    icon: "arrow.up.heart.fill")
                            }
                            .padding(.horizontal)
                        }

                        // Public Info
                        profileSection("Public Information") {
                            if store.isEditing {
                                LiquidTextField(
                                    "Display Name", text: $store.editableDisplayName)
                                // Username is generally not editable easily as verified by user request to keep it fixed,
                                // but backend `ensureUsername` suggests it MIGHT be.
                                // User said: "Сменять юзернейм мы не будем, пускай это будет невозможно."
                                // So I will make it READ ONLY in edit mode too, or just don't show it as editable.
                                infoRow("Username", value: store.editableUsername)
                                    .opacity(0.6)

                                VStack(alignment: .leading) {
                                    Text("Bio")
                                        .foregroundStyle(.white.opacity(0.8))
                                        .font(.caption)
                                    ZStack(alignment: .topLeading) {
                                        if store.editableBio.isEmpty {
                                            Text("Tell us about yourself...")
                                                .foregroundStyle(.white.opacity(0.4))
                                                .padding(.top, 8)
                                                .padding(.leading, 5)
                                        }
                                        TextField(
                                            "", text: $store.editableBio,
                                            axis: .vertical
                                        )
                                        .foregroundStyle(.white)
                                    }
                                    .padding()
                                    .background(Color.white.opacity(0.1))
                                    .cornerRadius(8)
                                    .lineLimit(3...6)
                                }

                                VStack(alignment: .leading) {
                                    Text("Visibility")
                                        .foregroundStyle(.white.opacity(0.8))
                                        .font(.caption)
                                    Picker("Visibility", selection: $store.editableVisibility) {
                                        Text("Public").tag("public")
                                        Text("Friends Only").tag("friends")
                                        Text("Private").tag("private")
                                    }
                                    .pickerStyle(.segmented)
                                    .environment(\.colorScheme, .dark)
                                }
                            } else {
                                infoRow("Display Name", value: profile.displayName)
                                infoRow("Username", value: profile.username)

                                VStack(alignment: .leading, spacing: 4) {
                                    Text("Bio")
                                        .foregroundStyle(.white.opacity(0.6))
                                        .font(.caption)
                                    Text(
                                        profile.bio?.isEmpty == false
                                            ? profile.bio! : "No bio set"
                                    )
                                    .foregroundStyle(
                                        profile.bio?.isEmpty == false
                                            ? .white : .white.opacity(0.4)
                                    )
                                    .italic(profile.bio?.isEmpty != false)
                                    .fixedSize(horizontal: false, vertical: true)
                                }
                                .padding(.top, 4)
                                .frame(maxWidth: .infinity, alignment: .leading)

                                infoRow("Visibility", value: profile.visibility.capitalized)
                            }
                        }

                        if !profile.isPremium && !store.isEditing {
                            profileSection("Membership") {
                                VStack(spacing: 16) {
                                    Text(
                                        "Unlock exclusive features like an animated avatar halo and special badges."
                                    )
                                    .foregroundStyle(.white.opacity(0.8))
                                    .font(.subheadline)
                                    .multilineTextAlignment(.center)

                                    Button {
                                        store.send(.becomePremiumTapped)
                                    } label: {
                                        HStack {
                                            Image(systemName: "star.fill")
                                            Text("Become Premium")
                                        }
                                        .font(.headline)
                                        .foregroundStyle(.black)
                                        .frame(maxWidth: .infinity)
                                        .padding()
                                        .background(
                                            LinearGradient(
                                                colors: [.liquidPrimary, .liquidSecondary],
                                                startPoint: .leading,
                                                endPoint: .trailing
                                            )
                                        )
                                        .cornerRadius(12)
                                    }
                                }
                            }
                        }

                        // Private Info
                        profileSection("Private Information") {
                            // Email is not editable via profile update
                            infoRow("Email", value: profile.email ?? "Not set")
                                .opacity(store.isEditing ? 0.6 : 1.0)
                        }

                        // Preferences
                        profileSection("Preferences") {
                            if store.isEditing {
                                LiquidTextField(
                                    "Genres (comma separated)",
                                    text: $store.editableMusicPreferences)
                            } else {
                                infoRow(
                                    "Music Genres",
                                    value: profile.preferences.genres?.joined(separator: ", ")
                                        ?? "None"
                                )
                            }
                        }

                        // Security & Linked Accounts
                        profileSection("Security & Accounts") {
                            VStack(spacing: 12) {
                                Button(action: {
                                    store.send(.forgotPasswordButtonTapped)
                                }) {
                                    Text("Forgot Password?")
                                        .font(.liquidCaption)
                                        .foregroundColor(.white.opacity(0.8))
                                        .frame(maxWidth: .infinity, alignment: .trailing)
                                }

                                Divider().background(.white.opacity(0.2))

                                ForEach(
                                    [
                                        AuthenticationClient.SocialHelper.SocialProvider.google,
                                        .intra42,
                                    ], id: \.self
                                ) { provider in
                                    HStack {
                                        Text(
                                            provider == .intra42
                                                ? "Intra 42" : provider.rawValue.capitalized
                                        )
                                        .foregroundStyle(.white)
                                        Spacer()
                                        if profile.linkedProviders.contains(provider.rawValue) {
                                            Button("Unlink", role: .destructive) {
                                                store.send(.unlinkAccount(provider))
                                            }
                                            .foregroundStyle(.red)
                                        } else {
                                            Button("Link") {
                                                store.send(.linkAccount(provider))
                                            }
                                            .foregroundStyle(.blue)
                                        }
                                    }
                                }
                            }
                        }

                        // Logout
                        if !store.isEditing {
                            Button {
                                store.send(.logoutButtonTapped)
                            } label: {
                                Text("Log Out")
                                    .font(.headline)
                                    .foregroundStyle(.red)
                                    .frame(maxWidth: .infinity)
                                    .padding()
                                    .background(Color.white.opacity(0.1))
                                    .cornerRadius(12)
                            }
                            .padding(.top, 20)
                        }
                    }
                    .padding()
                    .padding(.bottom, 80)
                }
                .animation(.easeInOut, value: store.isEditing)
                .transition(
                    .asymmetric(
                        insertion: .opacity.combined(with: .move(edge: .bottom)),
                        removal: .opacity
                    ))
            }

            if store.isAvatarLoading {
                Color.black.opacity(0.35)
                    .ignoresSafeArea()

                VStack(spacing: 12) {
                    ProgressView()
                        .tint(.white)
                    Text("Updating avatar...")
                        .font(.caption)
                        .foregroundStyle(.white.opacity(0.9))
                }
                .padding(20)
                .background(Color.black.opacity(0.6))
                .clipShape(RoundedRectangle(cornerRadius: 12))
            }
        }
        .navigationTitle("Profile")
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button(store.isEditing ? "Save" : "Edit") {
                    withAnimation {
                        if store.isEditing {
                            _ = store.send(.saveButtonTapped)
                        } else {
                            _ = store.send(.toggleEditMode)
                        }
                    }
                }
                .foregroundStyle(.white)
            }
            if store.isEditing {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Cancel") {
                        withAnimation {
                            _ = store.send(.toggleEditMode)
                        }
                    }
                    .foregroundStyle(.white)
                }
            }
        }
        .onAppear {
            store.send(.onAppear)
        }
        .imagePlaygroundSheet(
            isPresented: $store.isImagePlaygroundPresented.sending(\.toggleImagePlayground),
            concept: "Music lover, dj, cool avatar"
        ) { url in
            store.send(.imagePlaygroundResponse(url))
        }
        .alert($store.scope(state: \.alert, action: \.alert))
        .preferredColorScheme(.dark)
    }

    private func profileSection<Content: View>(_ title: String, @ViewBuilder content: () -> Content)
        -> some View
    {
        VStack(alignment: .leading, spacing: 12) {
            Text(title)
                .font(.headline)
                .foregroundStyle(.white.opacity(0.8))
                .padding(.leading, 4)

            VStack(spacing: 12) {
                content()
            }
            .padding()
            .background(Color.black.opacity(0.3))
            .cornerRadius(16)
        }
    }

    private func infoRow(_ label: String, value: String) -> some View {
        HStack {
            Text(label)
                .foregroundStyle(.white.opacity(0.6))
            Spacer()
            Text(value)
                .foregroundStyle(.white)
                .multilineTextAlignment(.trailing)
        }
    }

    struct LiquidTextField: View {
        let title: String
        @Binding var text: String

        init(_ title: String, text: Binding<String>) {
            self.title = title
            self._text = text
        }

        var body: some View {
            ZStack(alignment: .leading) {
                if text.isEmpty {
                    Text(title)
                        .foregroundStyle(.white.opacity(0.4))
                }
                TextField("", text: $text)
                    .foregroundStyle(.white)
            }
            .padding()
            .background(Color.white.opacity(0.1))
            .cornerRadius(8)
        }
    }

    struct ShimmerView: View {
        @State private var phase: CGFloat = 0

        var body: some View {
            Rectangle()
                .fill(Color.white.opacity(0.1))
                .overlay {
                    GeometryReader { geometry in
                        LinearGradient(
                            colors: [
                                .clear,
                                .white.opacity(0.2),
                                .clear,
                            ],
                            startPoint: .topLeading,
                            endPoint: .bottomTrailing
                        )
                        .frame(width: geometry.size.width * 2)
                        .offset(x: phase)
                        .onAppear {
                            withAnimation(
                                .linear(duration: 1.5)
                                    .repeatForever(autoreverses: false)
                            ) {
                                phase = geometry.size.width
                            }
                        }
                    }
                }
                .mask(Rectangle())
        }
    }

    private func statCard(title: String, value: String, icon: String) -> some View {
        VStack(spacing: 8) {
            HStack {
                Image(systemName: icon)
                    .foregroundStyle(.blue)
                Spacer()
            }

            Text(value)
                .font(.title2)
                .fontWeight(.bold)
                .foregroundStyle(.white)
                .frame(maxWidth: .infinity, alignment: .leading)

            Text(title)
                .font(.caption)
                .foregroundStyle(.white.opacity(0.6))
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding()
        .background(Color.black.opacity(0.3))
        .cornerRadius(16)
    }
}

#Preview {
    ProfileView(
        store: Store(
            initialState: ProfileFeature.State(),
            reducer: { ProfileFeature() }
        )
    )
}

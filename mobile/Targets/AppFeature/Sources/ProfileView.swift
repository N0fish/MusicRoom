import AppSupportClients
import ComposableArchitecture
import MusicRoomAPI
import MusicRoomUI
import SwiftUI

public struct ProfileView: View {
    @Bindable var store: StoreOf<ProfileFeature>

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
                            ZStack {
                                if store.isAvatarLoading {
                                    ShimmerView()
                                        .frame(width: 120, height: 120)
                                        .clipShape(Circle())
                                } else {
                                    AsyncImage(url: URL(string: profile.avatarUrl ?? "")) { image in
                                        image
                                            .resizable()
                                            .aspectRatio(contentMode: .fill)
                                    } placeholder: {
                                        Image(systemName: "person.circle.fill")
                                            .resizable()
                                            .foregroundStyle(.white.opacity(0.3))
                                    }
                                    .frame(width: 120, height: 120)
                                    .clipShape(Circle())
                                }
                            }
                            .overlay(Circle().stroke(.white.opacity(0.2), lineWidth: 2))

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
                                if !store.isChangingPassword {
                                    Button("Change Password") {
                                        store.send(.toggleChangePasswordMode)
                                    }
                                    .buttonStyle(.bordered)
                                    .tint(.white)
                                } else {
                                    SecureField(
                                        "Current Password", text: $store.currentPassword
                                    )
                                    .textFieldStyle(.roundedBorder)
                                    SecureField("New Password", text: $store.newPassword)
                                        .textFieldStyle(.roundedBorder)
                                    SecureField("Confirm New", text: $store.confirmNewPassword)
                                        .textFieldStyle(.roundedBorder)

                                    HStack {
                                        Button("Cancel") {
                                            store.send(.toggleChangePasswordMode)
                                        }
                                        .foregroundStyle(.red)
                                        Spacer()
                                        Button("Update") {
                                            store.send(.changePasswordButtonTapped)
                                        }
                                    }
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

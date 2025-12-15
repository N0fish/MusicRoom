import AppSupportClients
import ComposableArchitecture
import MusicRoomUI
import SwiftUI

public struct ProfileView: View {
    @Bindable var store: StoreOf<ProfileFeature>

    public init(store: StoreOf<ProfileFeature>) {
        self.store = store
    }

    public var body: some View {
        NavigationStack {
            ZStack {
                LiquidBackground()
                    .ignoresSafeArea()

                if store.isLoading {
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
                                .overlay(Circle().stroke(.white.opacity(0.2), lineWidth: 2))

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

                            // Public Info
                            profileSection("Public Information") {
                                if store.isEditing {
                                    LiquidTextField(
                                        "Display Name", text: $store.editableDisplayName)
                                    LiquidTextField("Username", text: $store.editableUsername)
                                } else {
                                    infoRow("Display Name", value: profile.displayName)
                                    infoRow("Username", value: profile.username)
                                }
                            }

                            // Private Info
                            profileSection("Private Information") {
                                if store.isEditing {
                                    LiquidTextField("Email", text: $store.editableEmail)
                                        .textInputAutocapitalization(.never)
                                        .keyboardType(.emailAddress)
                                } else {
                                    infoRow("Email", value: profile.email ?? "Not set")
                                }
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
                        .padding()
                        .padding(.bottom, 80)
                    }
                }
            }
            .navigationTitle("Profile")
            .toolbarColorScheme(.dark, for: .navigationBar)
            .toolbarBackground(.hidden, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button(store.isEditing ? "Save" : "Edit") {
                        if store.isEditing {
                            store.send(.saveButtonTapped)
                        } else {
                            store.send(.toggleEditMode)
                        }
                    }
                    .foregroundStyle(.white)
                }
                if store.isEditing {
                    ToolbarItem(placement: .topBarLeading) {
                        Button("Cancel") { store.send(.toggleEditMode) }
                            .foregroundStyle(.white)
                    }
                }
            }
            .onAppear {
                store.send(.onAppear)
            }
        }
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
            TextField(title, text: $text)
                .padding()
                .background(Color.white.opacity(0.1))
                .cornerRadius(8)
                .foregroundStyle(.white)
        }
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

import AppSupportClients
import ComposableArchitecture
import SwiftUI

public struct ProfileView: View {
    @Bindable var store: StoreOf<ProfileFeature>

    public init(store: StoreOf<ProfileFeature>) {
        self.store = store
    }

    public var body: some View {
        NavigationStack {
            Form {
                if store.isLoading {
                    ProgressView().frame(maxWidth: .infinity, alignment: .center)
                } else if let errorMessage = store.errorMessage {
                    Text(errorMessage)
                        .foregroundColor(.red)
                } else if let profile = store.userProfile {

                    // Header
                    Section {
                        HStack {
                            Spacer()
                            VStack {
                                AsyncImage(url: URL(string: profile.avatarUrl)) { image in
                                    image.resizable().scaledToFill()
                                } placeholder: {
                                    Image(systemName: "person.circle.fill")
                                        .resizable()
                                        .foregroundColor(.gray)
                                }
                                .frame(width: 100, height: 100)
                                .clipShape(Circle())

                                Text(profile.displayName)
                                    .font(.title)
                                    .bold()

                                Text("@\(profile.username)")
                                    .font(.subheadline)
                                    .foregroundColor(.secondary)
                            }
                            Spacer()
                        }
                    }
                    .listRowBackground(Color.clear)

                    // Public Information
                    Section(header: Text("Public Information")) {
                        if store.isEditing {
                            TextField("Display Name", text: $store.editableDisplayName)
                            TextField("Username", text: $store.editableUsername)
                        } else {
                            LabeledContent("Display Name", value: profile.displayName)
                            LabeledContent("Username", value: profile.username)
                        }
                    }

                    // Private Information
                    Section(header: Text("Private Information")) {
                        if store.isEditing {
                            TextField("Email", text: $store.editableEmail)
                                .textInputAutocapitalization(.never)
                                .keyboardType(.emailAddress)
                        } else {
                            LabeledContent("Email", value: profile.email ?? "Not set")
                        }
                    }

                    // Preferences
                    Section(header: Text("Preferences")) {
                        if store.isEditing {
                            TextField(
                                "Music Genres (comma separated)",
                                text: $store.editableMusicPreferences)
                        } else {
                            LabeledContent(
                                "Music Genres", value: profile.preferences?["genres"] ?? "None")
                        }
                    }

                    // Security
                    if let providers = store.userProfile?.linkedProviders,
                        providers.isEmpty || providers.contains("email")
                    {
                        Section(header: Text("Security")) {
                            if store.isChangingPassword {
                                SecureField("Current Password", text: $store.currentPassword)
                                SecureField("New Password", text: $store.newPassword)
                                    .textContentType(.newPassword)
                                SecureField("Confirm New Password", text: $store.confirmNewPassword)
                                    .textContentType(.newPassword)

                                Button("Update Password") {
                                    store.send(.changePasswordButtonTapped)
                                }
                                .disabled(store.isLoading)

                                Button("Cancel", role: .cancel) {
                                    store.send(.toggleChangePasswordMode)
                                }
                            } else {
                                Button("Change Password") {
                                    store.send(.toggleChangePasswordMode)
                                }
                            }

                            if let success = store.passwordChangeSuccessMessage {
                                Text(success)
                                    .foregroundColor(.green)
                                    .font(.caption)
                            }
                        }
                    }

                    // Linked Accounts
                    Section(header: Text("Linked Accounts")) {
                        ForEach(
                            [AuthenticationClient.SocialHelper.SocialProvider.google, .intra42],
                            id: \.self
                        ) {
                            provider in
                            HStack {
                                Text(
                                    provider == .intra42
                                        ? "Intra 42" : provider.rawValue.capitalized)
                                Spacer()
                                if profile.linkedProviders.contains(provider.rawValue) {
                                    Button("Unlink", role: .destructive) {
                                        store.send(.unlinkAccount(provider))
                                    }
                                    .disabled(store.isLoading)
                                } else {
                                    Button("Link") {
                                        store.send(.linkAccount(provider))
                                    }
                                    .disabled(store.isLoading)
                                }
                            }
                        }
                    }

                    // Actions
                    Section {
                        Button(action: { store.send(.logoutButtonTapped) }) {
                            Text("Log Out")
                                .foregroundColor(.red)
                                .frame(maxWidth: .infinity)
                        }
                    }
                } else {
                    // Not loaded yet? Should be covered by isLoading or onAppear
                    Text("No profile data")
                }
            }
            .navigationTitle("Profile")
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    Button(store.isEditing ? "Save" : "Edit") {
                        if store.isEditing {
                            store.send(.saveButtonTapped)
                        } else {
                            store.send(.toggleEditMode)
                        }
                    }
                }
                if store.isEditing {
                    ToolbarItem(placement: .navigationBarLeading) {
                        Button("Cancel") {
                            store.send(.toggleEditMode)
                        }
                    }
                }
            }
            .onAppear {
                store.send(.onAppear)
            }
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

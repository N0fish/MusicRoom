import ComposableArchitecture
import MusicRoomUI
import SwiftUI

public struct FriendProfileView: View {
    @Bindable var store: StoreOf<FriendProfileFeature>

    public init(store: StoreOf<FriendProfileFeature>) {
        self.store = store
    }

    public var body: some View {
        ZStack {
            // Background
            LiquidBackground()
                .ignoresSafeArea()

            if store.isLoading && store.profile == nil {
                ProgressView()
            } else if let profile = store.profile {
                ScrollView {
                    VStack(spacing: 24) {
                        // Avatar Section
                        VStack(spacing: 16) {
                            PremiumAvatarView(
                                url: profile.avatarUrl,
                                isPremium: profile.isPremium,
                                size: 120
                            )

                        VStack(spacing: 4) {
                            let displayName = profile.displayName.isEmpty
                                ? profile.username
                                : profile.displayName

                            Text(displayName)
                                .font(.title2)
                                .fontWeight(.bold)
                                .foregroundStyle(.white)

                            Text("@\(profile.username)")
                                .font(.subheadline)
                                .foregroundStyle(.white.opacity(0.6))

                            if store.isFriend && !store.isMe {
                                HStack(spacing: 6) {
                                    Image(systemName: "checkmark.seal.fill")
                                        .font(.caption)
                                    Text("Friends")
                                        .font(.caption.bold())
                                }
                                .foregroundStyle(Color.liquidAccent)
                                .padding(.horizontal, 10)
                                .padding(.vertical, 4)
                                .background(Color.white.opacity(0.15))
                                .clipShape(Capsule())
                                .padding(.top, 6)
                            }
                        }
                        }
                        .padding(.top, 40)

                        // Info Section
                        VStack(alignment: .leading, spacing: 16) {
                            if let bio = profile.bio, !bio.isEmpty {
                                infoRow(icon: "text.quote", title: "About", value: bio)
                            }

                            infoRow(
                                icon: "lock.shield", title: "Visibility",
                                value: profile.visibility.capitalized)

                            // Add more fields if available (e.g. music prefs)
                        }
                        .padding()
                        .background(Color.black.opacity(0.2))
                        .cornerRadius(16)
                        .padding(.horizontal)
                    }
                }
            }
        }
        .onAppear {
            store.send(.view(.onAppear))
        }
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                if store.isLoading || store.isCheckingFriend {
                    ProgressView()
                } else if !store.isMe {
                    if store.isFriend {
                        Button(role: .destructive) {
                            store.send(.view(.removeFriendTapped))
                        } label: {
                            Image(systemName: "person.badge.minus")  // fixed icon
                                .foregroundStyle(.red)
                        }
                    } else {
                        Button {
                            store.send(.view(.addFriendTapped))
                        } label: {
                            Image(systemName: "person.badge.plus")  // fixed icon
                                .foregroundStyle(.blue)
                        }
                    }
                }
            }
        }
        .alert($store.scope(state: \.alert, action: \.alert))
    }

    // Helper View for Info Rows
    @ViewBuilder
    private func infoRow(icon: String, title: String, value: String) -> some View {
        HStack(alignment: .top, spacing: 12) {
            Image(systemName: icon)
                .foregroundStyle(.white.opacity(0.6))
                .frame(width: 20)

            VStack(alignment: .leading, spacing: 4) {
                Text(title)
                    .font(.caption)
                    .foregroundStyle(.white.opacity(0.6))
                Text(value)
                    .font(.body)
                    .foregroundStyle(.white)
            }
        }
    }
}

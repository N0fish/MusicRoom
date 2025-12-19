import AppSupportClients  // For Friend/FriendRequest models if strictly needed, or use Feature.State
import ComposableArchitecture
import EventFeature
import MusicRoomUI
import SwiftUI

public struct FriendsView: View {
    let store: StoreOf<FriendsFeature>

    public init(store: StoreOf<FriendsFeature>) {
        self.store = store
    }

    public var body: some View {
        @Bindable var store = store
        NavigationStack(path: $store.scope(state: \.path, action: \.path)) {
            ZStack {
                LiquidBackground()
                    .ignoresSafeArea()

                VStack {
                    Picker("Segment", selection: $store.selectedSegment.sending(\.segmentChanged)) {
                        ForEach(FriendsFeature.Segment.allCases) { segment in
                            Text(segment.rawValue).tag(segment)
                        }
                    }
                    .pickerStyle(.segmented)
                    .padding()

                    switch store.selectedSegment {
                    case .friends:
                        if store.friends.isEmpty && !store.isLoading {
                            ContentUnavailableView(
                                "No Friends Yet", systemImage: "person.2.slash",
                                description: Text("Search and add friends to see them here.")
                            )
                        } else {
                            friendsList(store: store, friends: store.friends)
                                .scrollContentBackground(.hidden)
                        }
                    case .requests:
                        if store.incomingRequests.isEmpty && !store.isLoading {
                            ContentUnavailableView(
                                "No Requests", systemImage: "tray",
                                description: Text("You have no incoming friend requests.")
                            )
                        } else {
                            requestsList(store: store, requests: store.incomingRequests)
                                .scrollContentBackground(.hidden)
                        }
                    case .search:
                        searchView(store: store)
                    }
                }
                .padding(.top)
            }
            .navigationTitle("Friends")
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackground(.hidden, for: .navigationBar)
            .preferredColorScheme(.dark)
            .onAppear {
                store.send(.onAppear)
            }
        } destination: { store in
            FriendProfileView(store: store)
        }
    }

    private func friendsList(store: StoreOf<FriendsFeature>, friends: [AppSupportClients.Friend])
        -> some View
    {
        List {
            ForEach(friends) { friend in
                Button(action: {
                    store.send(.friendTapped(friend))
                }) {
                    HStack {
                        AsyncImage(url: URL(string: friend.avatarUrl ?? "")) { image in
                            image.resizable()
                        } placeholder: {
                            Image(systemName: "person.circle.fill")
                                .resizable()
                                .foregroundStyle(.gray)
                        }
                        .frame(width: 40, height: 40)
                        .clipShape(Circle())

                        VStack(alignment: .leading) {
                            Text(friend.displayName)
                                .font(.headline)
                                .foregroundStyle(.white)
                            Text("@\(friend.username)")
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }

                        Spacer()

                        Image(systemName: "chevron.right")  // Navigate
                            .foregroundStyle(.secondary)
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)  // Remove default button highlighting that might look bad in list
                .listRowBackground(Color.white.opacity(0.1))
                .listRowSeparatorTint(.white.opacity(0.2))
            }
        }
        .listStyle(.plain)
    }

    private func requestsList(
        store: StoreOf<FriendsFeature>, requests: [AppSupportClients.FriendRequest]
    ) -> some View {
        List {
            ForEach(requests) { request in
                HStack {
                    Button(action: {
                        let friend = AppSupportClients.Friend(
                            id: request.senderId,
                            userId: request.senderId,
                            username: request.senderUsername,
                            displayName: request.senderDisplayName,
                            avatarUrl: request.senderAvatarUrl
                        )
                        store.send(.searchUserTapped(friend))
                    }) {
                        HStack {
                            AsyncImage(url: URL(string: request.senderAvatarUrl ?? "")) { image in
                                image.resizable()
                            } placeholder: {
                                Image(systemName: "person.circle.fill")
                                    .resizable()
                                    .foregroundStyle(.gray)
                            }
                            .frame(width: 40, height: 40)
                            .clipShape(Circle())

                            VStack(alignment: .leading) {
                                Text(
                                    request.senderDisplayName.isEmpty
                                        ? request.senderUsername : request.senderDisplayName
                                )
                                .font(.headline)
                                .foregroundStyle(.white)
                                Text("wants to be friends")
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                    .buttonStyle(.plain)

                    Spacer()

                    Button {
                        store.send(.acceptRequest(request.senderId))  // CAREFUL: Request senderId is the UserID to accept
                    } label: {
                        Image(systemName: "checkmark.circle.fill")
                            .foregroundStyle(.green)
                            .font(.title2)
                    }
                    .buttonStyle(.borderless)

                    Button {
                        store.send(.rejectRequest(request.senderId))
                    } label: {
                        Image(systemName: "xmark.circle.fill")
                            .foregroundStyle(.red)
                            .font(.title2)
                    }
                    .buttonStyle(.borderless)
                }
                .listRowBackground(Color.white.opacity(0.1))
                .listRowSeparatorTint(.white.opacity(0.2))
            }
        }
        .listStyle(.plain)
    }

    private func searchView(store: StoreOf<FriendsFeature>) -> some View {
        VStack {
            searchInput(store: store)
                .padding(.horizontal)

            searchResultsList(
                store: store, friends: store.friends, searchResults: store.searchResults)
        }
    }

    private func searchInput(store: StoreOf<FriendsFeature>) -> some View {
        @Bindable var store = store
        return HStack {
            Image(systemName: "magnifyingglass")
                .foregroundStyle(.secondary)
            TextField("Search users by name or username", text: $store.searchQuery)
                .textFieldStyle(.plain)
                .padding(8)
                .background(Color.white.opacity(0.1))
                .cornerRadius(8)
                .autocorrectionDisabled()
                .textInputAutocapitalization(.never)
                .submitLabel(.search)
                .onSubmit {
                    store.send(.performSearch)
                }
        }
    }

    private func searchResultsList(
        store: StoreOf<FriendsFeature>, friends: [AppSupportClients.Friend],
        searchResults: [AppSupportClients.Friend]
    ) -> some View {
        let friendIds = Set(friends.map { $0.userId })

        return List {
            ForEach(searchResults) { user in
                SearchResultRow(
                    user: user,
                    isFriend: friendIds.contains(user.userId),
                    isMe: user.userId == store.currentUserId,
                    onAdd: {
                        withAnimation {
                            _ = store.send(.sendRequest(user.id))
                        }
                    },
                    onTapProfile: {
                        store.send(.searchUserTapped(user))
                    }
                )
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .animation(.easeInOut, value: searchResults)
    }
}

private struct SearchResultRow: View {
    let user: AppSupportClients.Friend
    let isFriend: Bool
    let isMe: Bool
    let onAdd: () -> Void
    let onTapProfile: () -> Void

    var body: some View {
        HStack {
            Button(action: onTapProfile) {
                HStack {
                    AsyncImage(url: URL(string: user.avatarUrl ?? "")) { image in
                        image.resizable()
                    } placeholder: {
                        Image(systemName: "person.circle.fill")
                            .resizable()
                            .foregroundStyle(.gray)
                    }
                    .frame(width: 40, height: 40)
                    .clipShape(Circle())

                    VStack(alignment: .leading) {
                        Text(user.displayName)
                            .font(.headline)
                            .foregroundStyle(.white)
                        Text("@\(user.username)")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            }
            .buttonStyle(.plain)

            Spacer()

            if isMe {
                Text("You")
                    .font(.caption)
                    .padding(6)
                    .background(Color.blue.opacity(0.1))
                    .cornerRadius(8)
                    .foregroundStyle(.blue)
            } else if isFriend {
                Text("Friends")
                    .font(.caption)
                    .padding(6)
                    .background(Color.secondary.opacity(0.1))
                    .cornerRadius(8)
                    .foregroundStyle(.secondary)
            } else {
                Button("Add", action: onAdd)
                    .buttonStyle(.borderedProminent)
                    .tint(.purple)
            }
        }
        .listRowBackground(Color.white.opacity(0.1))
        .listRowSeparatorTint(Color.white.opacity(0.2))
    }
}

import AppSupportClients
import ComposableArchitecture
import MusicRoomDomain
import MusicRoomUI
import SwiftUI

public struct CreateEventView: View {
    @Bindable var store: StoreOf<CreateEventFeature>

    public init(store: StoreOf<CreateEventFeature>) {
        self.store = store
    }

    public var body: some View {
        NavigationStack {
            ZStack {
                LiquidBackground()
                    .ignoresSafeArea()

                VStack(spacing: 24) {
                    // Form Content
                    VStack(spacing: 16) {
                        TextField("Event Name", text: $store.name)
                            .padding()
                            .background(GlassView(cornerRadius: 12))
                            .foregroundStyle(.white)
                            .font(.liquidBody)

                        VStack(alignment: .leading, spacing: 8) {
                            Text("Visibility")
                                .font(.liquidCaption)
                                .foregroundStyle(.white.opacity(0.8))
                                .padding(.leading, 4)

                            Picker("Visibility", selection: $store.visibility) {
                                ForEach([EventVisibility.publicEvent, .privateEvent], id: \.self) {
                                    vis in
                                    Text(vis.label).tag(vis)
                                }
                            }
                            .pickerStyle(.segmented)
                            .colorScheme(.dark)  // Force dark mode for segmented picker visibility
                        }

                        VStack(alignment: .leading, spacing: 8) {
                            Text("Who can vote?")
                                .font(.liquidCaption)
                                .foregroundStyle(.white.opacity(0.8))
                                .padding(.leading, 4)

                            Picker("License", selection: $store.licenseMode) {
                                ForEach(EventLicenseMode.allCases, id: \.self) { mode in
                                    Text(mode.label).tag(mode)
                                }
                            }
                            .pickerStyle(.menu)
                            .tint(.white)
                            .padding()
                            .background(GlassView(cornerRadius: 12))
                        }
                    }
                    .padding()
                    .background(GlassView(cornerRadius: 20))
                    .padding(.horizontal)

                    // Friend Selection
                    if !store.friends.isEmpty {
                        VStack(alignment: .leading, spacing: 12) {
                            Text("Invite Friends")
                                .font(.liquidBody.bold())
                                .foregroundStyle(.white)
                                .padding(.leading, 4)

                            ScrollView {
                                LazyVStack(spacing: 8) {
                                    ForEach(store.friends) { friend in
                                        Button(action: {
                                            store.send(.toggleFriendSelection(friend.id))
                                        }) {
                                            HStack {
                                                AsyncImage(url: URL(string: friend.avatarUrl ?? ""))
                                                { image in
                                                    image.resizable().scaledToFill()
                                                } placeholder: {
                                                    Color.gray.opacity(0.3)
                                                }
                                                .frame(width: 40, height: 40)
                                                .clipShape(Circle())

                                                Text(friend.username)
                                                    .font(.liquidBody)
                                                    .foregroundStyle(.white)

                                                Spacer()

                                                if store.selectedFriendIDs.contains(friend.id) {
                                                    Image(systemName: "checkmark.circle.fill")
                                                        .foregroundStyle(Color.liquidAccent)
                                                } else {
                                                    Image(systemName: "circle")
                                                        .foregroundStyle(.gray)
                                                }
                                            }
                                            .padding()
                                            .background(GlassView(cornerRadius: 12))
                                        }
                                    }
                                }
                            }
                            .frame(maxHeight: 200)  // Limit height
                        }
                        .padding(.horizontal)
                    }

                    if let error = store.errorMessage {
                        Text(error)
                            .foregroundStyle(.red)
                            .font(.liquidCaption)
                    }

                    Button(action: { store.send(.createButtonTapped) }) {
                        if store.isLoading {
                            ProgressView()
                                .tint(.white)
                        } else {
                            Text("Create Event")
                                .font(.liquidBody.bold())
                                .foregroundStyle(.white)
                                .frame(maxWidth: .infinity)
                                .padding()
                                .background(Color.liquidAccent)
                                .clipShape(RoundedRectangle(cornerRadius: 16))
                        }
                    }
                    .disabled(store.isLoading)
                    .padding(.horizontal)

                    Spacer()
                }
                .padding(.top, 40)
            }
            .navigationTitle("New Event")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        store.send(.cancelButtonTapped)
                    }
                }
            }
            .task {
                store.send(.onAppear)
            }
        }
        .colorScheme(.dark)
    }
}

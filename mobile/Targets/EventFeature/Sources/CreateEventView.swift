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
        let minStartDate = Date.now.addingTimeInterval(60)
        let minEndDate = max(minStartDate, store.voteStart.addingTimeInterval(60))

        NavigationStack {
            ZStack {
                LiquidBackground()
                    .ignoresSafeArea()

                ScrollView {
                    LazyVStack(spacing: 24) {
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
                                        if !(store.visibility == .privateEvent && mode == .everyone) {
                                            Text(mode.label).tag(mode)
                                        }
                                    }
                                }
                                .pickerStyle(.menu)
                                .tint(.white)
                                .padding()
                                .background(GlassView(cornerRadius: 12))

                                if store.licenseMode == .geoTime {
                                    VStack(alignment: .leading, spacing: 12) {
                                        Text("Voting Restrictions")
                                            .font(.liquidBody.bold())
                                            .foregroundStyle(.white)

                                        // Time
                                        VStack(alignment: .leading, spacing: 4) {
                                            Text("Time Window")
                                                .font(.liquidCaption)
                                                .foregroundStyle(.gray)
                                            DatePicker(
                                                "Start",
                                                selection: $store.voteStart,
                                                in: minStartDate...
                                            )
                                            .labelsHidden()
                                            .colorScheme(.dark)
                                            DatePicker(
                                                "End",
                                                selection: $store.voteEnd,
                                                in: minEndDate...
                                            )
                                            .labelsHidden()
                                            .colorScheme(.dark)
                                        }
                                        .padding()
                                        .background(GlassView(cornerRadius: 12))

                                        // Location
                                        VStack(alignment: .leading, spacing: 8) {
                                            Text("Location")
                                                .font(.liquidCaption)
                                                .foregroundStyle(.gray)

                                            if let lat = store.geoLat, let lng = store.geoLng {
                                                HStack {
                                                    Image(systemName: "location.fill")
                                                        .foregroundStyle(Color.liquidAccent)
                                                    Text(
                                                        "\(String(format: "%.4f", lat)), \(String(format: "%.4f", lng))"
                                                    )
                                                    .font(.liquidBody)
                                                    .foregroundStyle(.white)
                                                }
                                            } else {
                                                Text("No location set")
                                                    .font(.liquidCaption)
                                                    .foregroundStyle(.red.opacity(0.8))
                                            }

                                            Button(action: { store.send(.getCurrentLocation) }) {
                                                if store.isGettingLocation {
                                                    ProgressView()
                                                        .tint(.white)
                                                } else {
                                                    Label(
                                                        "Set to Current Location",
                                                        systemImage: "location.circle"
                                                    )
                                                    .font(.liquidBody)
                                                    .foregroundStyle(.white)
                                                    .padding(8)
                                                    .background(Color.blue.opacity(0.6))
                                                    .clipShape(RoundedRectangle(cornerRadius: 8))
                                                }
                                            }
                                        }
                                        .padding()
                                        .background(GlassView(cornerRadius: 12))

                                        // Radius
                                        VStack(alignment: .leading, spacing: 8) {
                                            Text("Radius: \(store.geoRadiusM)m")
                                                .font(.liquidCaption)
                                                .foregroundStyle(.gray)

                                            Picker("Radius", selection: $store.geoRadiusM) {
                                                Text("100m").tag(100)
                                                Text("500m").tag(500)
                                                Text("1km").tag(1000)
                                            }
                                            .pickerStyle(.segmented)
                                            .colorScheme(.dark)
                                        }
                                    }
                                }
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
                    }
                    .padding(.top, 40)
                    .padding(.bottom, 24)
                }
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

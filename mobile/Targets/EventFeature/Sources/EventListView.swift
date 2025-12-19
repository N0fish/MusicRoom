import ComposableArchitecture
import MusicRoomDomain
import MusicRoomUI
import SwiftUI

public struct EventListView: View {
    @Bindable var store: StoreOf<EventListFeature>
    @Namespace private var namespace

    public init(store: StoreOf<EventListFeature>) {
        self.store = store
    }

    public var body: some View {
        NavigationStack(path: $store.scope(state: \.path, action: \.path)) {
            ZStack {
                LiquidBackground()
                    .ignoresSafeArea()

                VStack(spacing: 0) {

                    if store.isLoading {
                        ProgressView()
                            .tint(.white)
                            .scaleEffect(1.5)
                            .frame(maxHeight: .infinity)
                    } else if let error = store.errorMessage {
                        errorView(message: error)
                    } else if store.events.isEmpty {
                        emptyState
                    } else {
                        eventList
                    }
                }
            }
            .safeAreaInset(edge: .top) {
                if store.isOffline {
                    Text("You are offline. Showing cached data.")
                        .font(.caption)
                        .padding(8)
                        .frame(maxWidth: .infinity)
                        .background(Color.orange)
                        .foregroundColor(.white)
                }
            }
            .onAppear {
                store.send(.onAppear)
            }
            .navigationTitle("Events")
            .navigationBarTitleDisplayMode(.inline)
            .toolbarBackground(.hidden, for: .navigationBar)
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button(action: {
                        store.send(.createEventButtonTapped)
                    }) {
                        Image(systemName: "plus")
                            .font(.system(size: 20, weight: .bold))
                            .foregroundStyle(.white)
                            .frame(width: 40, height: 40)
                    }
                    .foregroundStyle(.white)
                }
            }
            .preferredColorScheme(.dark)
        } destination: { store in
            EventDetailView(store: store)
        }
        .sheet(item: $store.scope(state: \.createEvent, action: \.createEvent)) {
            createEventStore in
            CreateEventView(store: createEventStore)
        }
    }

    // Custom header removed in favor of standard toolbar

    private func errorView(message: String) -> some View {
        VStack(spacing: 16) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.liquidHeroIcon)
                .foregroundStyle(Color.liquidAccent)

            Text("Something went wrong")
                .font(.liquidH2)
                .foregroundStyle(.white)

            Text(message)
                .font(.liquidBody)
                .multilineTextAlignment(.center)
                .foregroundStyle(.white.opacity(0.8))

            LiquidPrimaryButton(
                title: "Retry",
                action: {
                    store.send(.retryButtonTapped)
                })
        }
        .padding()
        .frame(maxHeight: .infinity)
    }

    private var emptyState: some View {
        VStack(spacing: 16) {
            Image(systemName: "music.mic")
                .font(.liquidHeroIcon)
                .foregroundStyle(Color.liquidSecondary)

            Text("No Events Yet")
                .font(.liquidH2)
                .foregroundStyle(.white)

            Text("Create a party or wait for invites!")
                .font(.liquidBody)
                .foregroundStyle(.gray)
        }
        .frame(maxHeight: .infinity)
    }

    private var eventList: some View {
        List {
            // My Events Section
            let myEvents = store.events.filter {
                $0.ownerId == store.currentUserId || ($0.isJoined ?? false)
            }
            if !myEvents.isEmpty {
                Section {
                    ForEach(myEvents) { event in
                        let isOwner = event.ownerId == store.currentUserId
                        EventCard(event: event)
                            .listRowBackground(Color.clear)
                            .listRowSeparator(.hidden)
                            .listRowInsets(EdgeInsets(top: 8, leading: 16, bottom: 8, trailing: 16))
                            .contentShape(Rectangle())
                            .onTapGesture {
                                store.send(.eventTapped(event))
                            }
                            .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                                Button(role: .destructive) {
                                    store.send(.deleteEvent(event))
                                } label: {
                                    Label(
                                        isOwner ? "Delete" : "Leave",
                                        systemImage: isOwner ? "trash" : "door.left.hand.open"
                                    )
                                }
                                .tint(isOwner ? .red : .orange)
                            }
                    }
                } header: {
                    Text("My Events")
                        .font(.title2)
                        .foregroundStyle(.white)
                        .textCase(nil)
                        .padding(.vertical, 8)
                }
            }

            // Explore Section
            let exploreEvents = store.events.filter {
                $0.ownerId != store.currentUserId && !($0.isJoined ?? false)
            }
            if !exploreEvents.isEmpty {
                Section {
                    ForEach(exploreEvents) { event in
                        EventCard(event: event)
                            .listRowBackground(Color.clear)
                            .listRowSeparator(.hidden)
                            .listRowInsets(EdgeInsets(top: 8, leading: 16, bottom: 8, trailing: 16))
                            .contentShape(Rectangle())
                            .onTapGesture {
                                store.send(.eventTapped(event))
                            }
                    }
                } header: {
                    Text("Explore")
                        .font(.title2)
                        .foregroundStyle(.white)
                        .textCase(nil)
                        .padding(.vertical, 8)
                }
            }
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        .refreshable {
            store.send(.loadEvents)
        }
        .animation(.default, value: store.events)
    }
}

struct BouncyScaleButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.95 : 1.0)
            .animation(.spring(response: 0.3, dampingFraction: 0.6), value: configuration.isPressed)
    }
}

struct EventCard: View {
    let event: Event

    var body: some View {
        ZStack(alignment: .leading) {
            GlassView(cornerRadius: 20)

            VStack(alignment: .leading, spacing: 8) {
                HStack {
                    Text(event.name)
                        .font(.liquidH2)
                        .foregroundStyle(.white)
                        .lineLimit(1)

                    Spacer()

                    if event.visibility == .privateEvent {
                        Image(systemName: "lock.fill")
                            .foregroundStyle(Color.liquidSecondary)
                    }
                }

                HStack {
                    Label(event.licenseMode.label, systemImage: "person.2.fill")
                    Spacer()
                    //                    if let radius = event.geoRadiusM {
                    //                        Label("\(radius)m", systemImage: "location.fill")
                    //                    }
                }
                .font(.liquidCaption)
                .foregroundStyle(.gray)
            }
            .padding()
        }
    }
}

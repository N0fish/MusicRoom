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
        ScrollView {
            LazyVStack(spacing: 24) {
                // My Events Section
                let myEvents = store.events.filter {
                    $0.ownerId == store.currentUserId || ($0.isJoined ?? false)
                }
                if !myEvents.isEmpty {
                    VStack(alignment: .leading, spacing: 16) {
                        Text("My Events")
                            .font(.title2)
                            .foregroundStyle(.white)
                            .padding(.horizontal)

                        ForEach(Array(myEvents.enumerated()), id: \.element.id) { index, event in
                            SwipeableEventRow(
                                event: event,
                                currentUserId: store.currentUserId,
                                onTap: {
                                    store.send(.eventTapped(event))
                                },
                                onAction: {
                                    store.send(.deleteEvent(event))
                                }
                            )
                            .matchedGeometryEffect(id: event.id, in: namespace)
                        }
                    }
                }

                // Explore Section
                let exploreEvents = store.events.filter {
                    $0.ownerId != store.currentUserId && !($0.isJoined ?? false)
                }
                if !exploreEvents.isEmpty {
                    VStack(alignment: .leading, spacing: 16) {
                        Text("Explore")
                            .font(.title2)
                            .foregroundStyle(.white)
                            .padding(.horizontal)

                        ForEach(Array(exploreEvents.enumerated()), id: \.element.id) {
                            index, event in
                            // Non-swipeable row for Explore events (or swipe to join?)
                            // For now, just tap to enter (and potentially join via detail)
                            // We disable the swipe action by not wrapping in SwipeableEventRow or passing no-op
                            // But SwipeableEventRow provides the card UI.
                            // Let's use SwipeableEventRow but perhaps disable swipe if not joined?
                            // Currently SwipeableEventRow handles gesture.
                            // Simplest: Use SwipeableEventRow but onAction does nothing or shows alert?
                            // Better: Making Swipeable behavior conditional.

                            // Since we don't have time to refactor Row deeply, we use it.
                            // But maybe we should ONLY allow Leave on My Events.
                            SwipeableEventRow(
                                event: event,
                                currentUserId: store.currentUserId,
                                onTap: {
                                    store.send(.eventTapped(event))
                                },
                                onAction: {
                                    // No-op for explore events (can't leave what you haven't joined)
                                    // Or maybe "Hide"?
                                }
                            )
                            .matchedGeometryEffect(id: event.id, in: namespace)
                            // We need to modify SwipeableEventRow to accept "canSwipe".
                            // For now, we just pass the row.
                        }
                    }
                }
            }
            .padding(.vertical)
        }
        .animation(.default, value: store.events)
    }
}

struct SwipeableEventRow: View {
    let event: Event
    let currentUserId: String?
    let onTap: () -> Void
    let onAction: () -> Void

    @State private var offset: CGFloat = 0
    @State private var isSwiped = false
    private let actionThreshold: CGFloat = 60
    private let maxDrag: CGFloat = 100

    private var isOwner: Bool {
        event.ownerId == currentUserId
    }

    var body: some View {
        ZStack {
            // Background Action Layer
            GeometryReader { geometry in
                HStack {
                    Spacer()
                    ZStack {
                        RoundedRectangle(cornerRadius: 20)
                            .fill(isOwner ? Color.red : Color.orange)

                        VStack(spacing: 4) {
                            Image(systemName: isOwner ? "trash.fill" : "door.left.hand.open")
                                .font(.title2)
                                .fontWeight(.bold)
                            Text(isOwner ? "Delete" : "Leave")
                                .font(.caption)
                                .fontWeight(.medium)
                        }
                        .foregroundColor(.white)
                        .padding(.trailing, 20)
                    }
                    .frame(width: max(offset * -1, 0))
                    .clipShape(RoundedRectangle(cornerRadius: 20))
                    .opacity(offset < 0 ? 1 : 0)
                }
            }

            // Foreground Card
            EventCard(event: event)
                .contentShape(Rectangle())  // Ensure entire area is hittable
                .offset(x: offset)
                .onTapGesture {
                    if offset == 0 {
                        onTap()
                    } else {
                        // If swiped, valid tap resets the swipe
                        withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                            offset = 0
                        }
                    }
                }
                .gesture(
                    DragGesture()
                        .onChanged { value in
                            // Only allow swipe if owner or joined
                            guard (event.isJoined ?? false) || isOwner else { return }

                            if value.translation.width < 0 {
                                // Resistance curve
                                let translation = value.translation.width
                                offset =
                                    translation > -maxDrag
                                    ? translation
                                    : -maxDrag - (pow(abs(translation + maxDrag), 0.7))
                            } else {
                                // No right swipe - strict limit
                                offset = 0
                            }
                        }
                        .onEnded { value in
                            guard (event.isJoined ?? false) || isOwner else { return }

                            withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                                if value.translation.width < -actionThreshold {
                                    // Trigger action and reset
                                    offset = 0
                                    onAction()
                                } else {
                                    offset = 0
                                }
                            }
                        }
                )
        }
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

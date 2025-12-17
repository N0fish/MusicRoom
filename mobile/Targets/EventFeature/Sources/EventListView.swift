import ComposableArchitecture
import MusicRoomDomain
import MusicRoomUI
import SwiftUI

public struct EventListView: View {
    @Bindable var store: StoreOf<EventListFeature>

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
            LazyVStack(spacing: 16) {
                ForEach(Array(store.events.enumerated()), id: \.element.id) { index, event in
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
                    .transition(.move(edge: .bottom).combined(with: .opacity))
                    .animation(
                        .spring(duration: 0.5, bounce: 0.3).delay(Double(index) * 0.05),
                        value: store.events)
                }
            }
            .padding()
        }
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

                    StatusBadge(isActive: isEventActive(event))
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

    private func isEventActive(_ event: Event) -> Bool {
        // Simple logic: if voteStart/End are nil, it's active "Always"
        // If set, check current date
        let now = Date()
        if let start = event.voteStart, now < start { return false }
        if let end = event.voteEnd, now > end { return false }
        return true
    }
}

struct StatusBadge: View {
    let isActive: Bool

    var body: some View {
        Text(isActive ? "LIVE" : "ENDED")
            .font(.caption.bold())
            .padding(.horizontal, 8)
            .padding(.vertical, 4)
            .background(isActive ? Color.liquidAccent : Color.gray)
            .foregroundStyle(.white)
            .clipShape(Capsule())
    }
}

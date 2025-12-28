import AppSupportClients
@_spi(Presentation) import ComposableArchitecture
import Dependencies
import MusicRoomAPI
import MusicRoomDomain
import MusicRoomUI
import SearchFeature
import SwiftUI

public struct EventDetailView: View {
    @Bindable var store: StoreOf<EventDetailFeature>

    public init(store: StoreOf<EventDetailFeature>) {
        self.store = store
    }

    @Namespace private var animation

    private var isOwner: Bool {
        store.currentUserId == store.event.ownerId
    }

    public var body: some View {
        ZStack {
            LiquidBackground()
                .ignoresSafeArea()

            // Hidden YouTube Player
            EventYouTubePlayerView(store: store)
                .frame(width: 1, height: 1)
                .opacity(0.01)  // Nearly invisible but active
                .allowsHitTesting(false)

            VStack(spacing: 0) {
                // Header removed in favor of standard toolbar

                ScrollView {
                    VStack(alignment: .leading, spacing: 20) {

                        // Join Button for Explore events
                        JoinButtonView(store: store)

                        // Now Playing Section
                        NowPlayingSectionView(store: store, animation: animation)

                        // Leaderboard / Playlist
                        PlaylistView(store: store, animation: animation)

                        // Add Track Button
                        AddTrackButtonView(store: store)
                            .transition(.move(edge: .bottom).combined(with: .opacity))

                        Spacer(minLength: 100)
                    }
                    .padding(.top)
                }
            }

            if store.isLoading {
                ProgressView()
                    .tint(.white)
            }
        }
        .overlay(alignment: .top) {
            UserAlertOverlay(store: store)
        }
        .animation(.spring(response: 0.4, dampingFraction: 0.7), value: store.userAlert)
        .onAppear {
            store.send(.onAppear)
        }
        .navigationTitle(store.event.name)
        .navigationBarTitleDisplayMode(.inline)
        .toolbarBackground(.hidden, for: .navigationBar)
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                HStack(spacing: 16) {
                    if isOwner {
                        if store.isInvitingFriends {
                            ProgressView()
                                .tint(.white)
                        } else {
                            Button {
                                store.send(.inviteButtonTapped)
                            } label: {
                                Image(systemName: "person.crop.circle.badge.plus")
                                    .foregroundStyle(.white)
                            }
                        }
                    }
                    Button {
                        store.send(.participantsButtonTapped)
                    } label: {
                        Image(systemName: "person.2.fill")
                            .foregroundStyle(.white)
                    }
                }
            }
        }
        .sheet(item: $store.scope(state: \.musicSearch, action: \.musicSearch)) { searchStore in
            MusicSearchView(store: searchStore)
        }
        .sheet(isPresented: $store.isShowingParticipants) {
            ParticipantsListView(store: store)
                .presentationDetents([.medium, .large])
        }
        .sheet(isPresented: $store.isShowingInviteSheet) {
            InviteEventFriendSheet(friends: store.friends) { friend in
                store.send(.inviteFriendTapped(friend))
            }
        }
    }
}

struct TrackRow: View {
    let index: Int
    let track: Track
    let voteCount: Int
    let isMyVote: Bool
    let isOffline: Bool
    let onVote: () -> Void
    var showVote: Bool = true  // Parameter to hide vote button
    var timeRemaining: TimeInterval? = nil
    var totalDuration: TimeInterval? = nil

    @State private var isAnimating = false

    private func formatDuration(ms: Int) -> String {
        let duration = TimeInterval(ms) / 1000
        let formatter = DateComponentsFormatter()
        formatter.allowedUnits = [.minute, .second]
        formatter.unitsStyle = .positional
        formatter.zeroFormattingBehavior = .pad
        return formatter.string(from: duration) ?? "0:00"
    }

    var body: some View {
        GlassView(cornerRadius: 16)
            .frame(height: 80)
            .overlay(
                HStack(spacing: 12) {
                    // Index / Playing Icon
                    if index == 0 {
                        Image(systemName: "speaker.wave.3.fill")
                            .font(.system(size: 20))
                            .foregroundStyle(Color.liquidAccent)
                            .frame(width: 35)
                    } else {
                        Text("#\(index)")
                            .font(.system(size: 20, weight: .bold, design: .rounded))
                            .minimumScaleFactor(0.8)
                            .foregroundStyle(Color.white.opacity(0.5))
                            .frame(width: 35)
                    }

                    // Thumbnail
                    if let url = track.thumbnailUrl {
                        AsyncImage(url: url) { phase in
                            switch phase {
                            case .success(let image):
                                image.resizable().aspectRatio(contentMode: .fill)
                            default:
                                Color.gray.opacity(0.3)
                            }
                        }
                        .frame(width: 50, height: 50)
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                    } else {
                        Rectangle()
                            .fill(Color.gray.opacity(0.3))
                            .frame(width: 50, height: 50)
                            .clipShape(RoundedRectangle(cornerRadius: 8))
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        Text(track.title)
                            .font(.liquidBody.bold())
                            .foregroundStyle(Color.white)
                            .lineLimit(1)

                        HStack {
                            Text(track.artist)
                                .font(.liquidCaption)
                                .foregroundStyle(Color.white.opacity(0.7))
                                .lineLimit(1)

                            if let remaining = timeRemaining, let total = totalDuration, total > 0 {
                                VStack(alignment: .leading, spacing: 2) {
                                    ProgressView(value: total - remaining, total: total)
                                        .tint(.liquidAccent)
                                        .scaleEffect(y: 0.5)

                                    HStack {
                                        Text(formatDuration(ms: Int((total - remaining) * 1000)))
                                        Spacer()
                                        Text("-" + formatDuration(ms: Int(remaining * 1000)))
                                    }
                                    .font(.caption2)
                                    .monospacedDigit()
                                    .foregroundStyle(Color.white.opacity(0.6))
                                }
                                .padding(.top, 4)
                            } else if let durationMs = track.durationMs {
                                Text("• " + formatDuration(ms: durationMs))
                                    .font(.liquidCaption)
                                    .foregroundStyle(Color.white.opacity(0.5))
                            }
                        }

                        if timeRemaining == nil {
                            Text("\(voteCount) \(voteCount == 1 ? "vote" : "votes")")
                                .font(.caption2)
                                .foregroundStyle(Color.liquidAccent)
                        }
                    }

                    Spacer()

                    if showVote {
                        Button(action: {
                            withAnimation(
                                .spring(response: 0.3, dampingFraction: 0.4, blendDuration: 0)
                            ) {
                                isAnimating = true
                            }
                            onVote()
                            // Reset animation
                            DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
                                withAnimation {
                                    isAnimating = false
                                }
                            }
                        }) {
                            Image(systemName: isMyVote ? "arrow.up.circle.fill" : "arrow.up.circle")
                                .font(.system(size: 28))
                                .foregroundStyle(
                                    isOffline ? Color.gray : (isMyVote ? Color.green : Color.white)
                                )
                                .scaleEffect(isAnimating ? 1.3 : 1.0)
                        }
                        .disabled(isOffline || isMyVote)
                    } else {
                        Image(systemName: "waveform")  // Paying icon
                            .foregroundStyle(.white)
                            .font(.system(size: 24))
                    }
                }
                .padding(.horizontal, 16)
            )
    }
}

struct ParticipantsListView: View {
    @Bindable var store: StoreOf<EventDetailFeature>

    private var isOwner: Bool {
        store.currentUserId == store.event.ownerId
    }

    var body: some View {
        NavigationStack(path: $store.scope(state: \.path, action: \.path)) {
            List {
                if store.isLoadingParticipants {
                    HStack {
                        Spacer()
                        ProgressView()
                        Spacer()
                    }
                    .listRowBackground(Color.clear)
                } else {
                    // Organizer Section
                    if let owner = store.ownerProfile {
                        Section("Organizer") {
                            Button {
                                store.send(.participantTapped(owner))
                            } label: {
                                ParticipantRow(profile: owner)
                            }
                        }
                    }

                    // Participants Section
                    Section("Participants") {
                        if store.participants.isEmpty {
                            Text("No other participants joined yet.")
                                .foregroundStyle(.secondary)
                        } else {
                            ForEach(store.participants, id: \.userId) { participant in
                                Button {
                                    store.send(.participantTapped(participant))
                                } label: {
                                    ParticipantRow(profile: participant)
                                }
                                .contextMenu {
                                    if isOwner {
                                        Button(role: .destructive) {
                                            store.send(.requestTransferOwnership(participant))
                                        } label: {
                                            Label("Make Owner", systemImage: "crown.fill")
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
            .navigationTitle("Participants")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") {
                        store.send(.set(\.isShowingParticipants, false))
                    }
                }
            }
            .confirmationDialog(
                $store.scope(state: \.confirmationDialog, action: \.confirmationDialog))
        } destination: { store in
            FriendProfileView(store: store)
        }
    }
}

struct ParticipantRow: View {
    let profile: PublicUserProfile

    var body: some View {
        HStack {
            PremiumAvatarView(
                url: profile.avatarUrl,
                isPremium: profile.isPremium,
                size: 40
            )

            VStack(alignment: .leading) {
                Text(profile.displayName.isEmpty ? profile.username : profile.displayName)
                    .foregroundStyle(.primary)
                    .font(.headline)
                if !profile.displayName.isEmpty {
                    Text("@\(profile.username)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
        }
    }
}

struct InviteEventFriendSheet: View {
    let friends: [Friend]
    let onSelect: (Friend) -> Void

    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List(friends) { friend in
                Button {
                    onSelect(friend)
                    dismiss()
                } label: {
                    HStack(spacing: 12) {
                        PremiumAvatarView(
                            url: friend.avatarUrl,
                            isPremium: friend.isPremium,
                            size: 50
                        )

                        VStack(alignment: .leading, spacing: 4) {
                            Text(friend.displayName.isEmpty ? friend.username : friend.displayName)
                                .font(.body)
                                .fontWeight(.medium)
                                .foregroundColor(.primary)

                            Text("@\(friend.username)")
                                .font(.subheadline)
                                .foregroundColor(.secondary)
                        }

                        Spacer()

                        Image(systemName: "paperplane")
                            .foregroundColor(.accentColor)
                    }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
            }
            .listStyle(.plain)
            .navigationTitle("Invite Friend")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                }
            }
        }
    }
}

struct JoinButtonView: View {
    @Bindable var store: StoreOf<EventDetailFeature>

    var body: some View {
        if store.event.isJoined == false
            && (store.event.licenseMode != .invitedOnly || store.event.visibility == .publicEvent)
        {
            Button {
                withAnimation(.spring(response: 0.5, dampingFraction: 0.7)) {
                    _ = store.send(.joinEventTapped)
                }
            } label: {
                HStack {
                    Image(systemName: "plus.circle.fill")
                    Text("Join Event")
                }
                .font(.liquidBody.bold())
                .foregroundStyle(.white)
                .padding()
                .frame(maxWidth: .infinity)
                .background(Color.blue.opacity(0.8))
                .clipShape(RoundedRectangle(cornerRadius: 12))
            }
            .padding(.horizontal)
            .transition(
                .asymmetric(
                    insertion: .scale.combined(with: .opacity),
                    removal: .opacity.combined(with: .scale(scale: 0.8))))
        }
    }
}

struct NowPlayingSectionView: View {
    @Bindable var store: StoreOf<EventDetailFeature>
    var animation: Namespace.ID

    var body: some View {
        Group {
            if let currentTrack = store.tracks.first(where: { $0.status == "playing" }) {
                PlayingTrackView(
                    store: store, track: currentTrack, animation: animation
                )
            } else {
                QueuedTracksView(store: store)
            }
        }
        .animation(.spring(response: 0.5, dampingFraction: 0.8), value: store.tracks)
    }
}

struct PlayingTrackView: View {
    @Bindable var store: StoreOf<EventDetailFeature>
    let track: Track
    var animation: Namespace.ID

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Now Playing")
                .font(.liquidTitle)
                .foregroundStyle(Color.white)
                .padding(.horizontal)

            TrackRow(
                index: 0,
                track: track,
                voteCount: track.voteCount ?? 0,
                isMyVote: false,  // Can't vote on playing
                isOffline: store.isOffline,
                onVote: {},
                showVote: false,
                timeRemaining: store.timeRemaining,
                totalDuration: store.currentTrackDuration
            )
            .matchedGeometryEffect(id: track.id, in: animation)
            .transition(.scale(scale: 0.9).combined(with: .opacity))
            .padding(.horizontal)

            // Next Track Control (Play/Skip)
            if store.currentUserId == store.event.ownerId {
                Button {
                    store.send(.nextTrackButtonTapped)
                } label: {
                    HStack {
                        Image(systemName: "forward.end.fill")
                        Text("Next Track")
                    }
                    .font(.liquidBody.bold())
                    .foregroundStyle(.white)
                    .padding()
                    .background(Color.white.opacity(0.2))
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                }
                .padding(.horizontal)
            }
        }
    }
}

struct QueuedTracksView: View {
    @Bindable var store: StoreOf<EventDetailFeature>

    var body: some View {
        // Check if we have tracks ready to play
        let queuedTracks = store.tracks.filter {
            $0.status == "queued" || $0.status == nil
        }

        if !queuedTracks.isEmpty {
            if store.currentUserId == store.event.ownerId {
                Button {
                    store.send(.nextTrackButtonTapped)
                } label: {
                    HStack {
                        Image(systemName: "play.fill")
                        Text("Start Radio")
                    }
                    .font(.liquidBody.bold())
                    .foregroundStyle(.white)
                    .padding()
                    .frame(maxWidth: .infinity)
                    .background(Color.green.opacity(0.8))
                    .clipShape(RoundedRectangle(cornerRadius: 12))
                }
                .padding(.horizontal)
            } else {
                Text("Waiting for host to start...")
                    .font(.liquidCaption)
                    .foregroundStyle(.white.opacity(0.5))
                    .padding()
            }
        } else if !store.isLoading && !store.tracks.isEmpty {
            // Tracks exist but none are queued (all played) -> Event Finished
            VStack(spacing: 8) {
                Image(systemName: "flag.checkered")
                    .font(.system(size: 32))
                    .foregroundStyle(Color.white.opacity(0.8))
                    .padding(.bottom, 4)

                Text("Event Finished")
                    .font(.liquidH2)
                    .foregroundStyle(Color.white)

                Text("You can start anew!")
                    .font(.liquidCaption)
                    .foregroundStyle(Color.white.opacity(0.7))
            }
            .padding()
            .frame(maxWidth: .infinity)
            .background(Color.black.opacity(0.3))
            .clipShape(RoundedRectangle(cornerRadius: 16))
            .padding(.horizontal)
            .transition(.scale.combined(with: .opacity))
        }
    }
}

struct PlaylistView: View {
    @Bindable var store: StoreOf<EventDetailFeature>
    var animation: Namespace.ID

    var body: some View {
        Text("Up Next")
            .font(.liquidTitle)
            .foregroundStyle(Color.white)
            .padding(.horizontal)

        if store.tracks.filter({ $0.status == "queued" || $0.status == nil })
            .isEmpty && !store.isLoading
        {
            if store.isEventEnded {
                Text("No upcoming tracks")
                    .font(.liquidBody)
                    .foregroundStyle(Color.white.opacity(0.6))
                    .padding(.horizontal)
            } else if store.canVote {
                Text("Queue empty. Add tracks!")
                    .font(.liquidBody)
                    .foregroundStyle(Color.white.opacity(0.6))
                    .padding(.horizontal)
            } else {
                Text("Queue is empty")
                    .font(.liquidBody)
                    .foregroundStyle(Color.white.opacity(0.6))
                    .padding(.horizontal)
            }
        } else {
            LazyVStack(spacing: 12) {
                // Filter only queued (or nil status if legacy)
                let queuedTracks = store.tracks.filter {
                    $0.status == "queued" || $0.status == nil
                }

                // Sort tracks by vote count descending
                let sortedTracks = queuedTracks.sorted {
                    ($0.voteCount ?? 0) > ($1.voteCount ?? 0)
                }

                ForEach(sortedTracks) { track in
                    TrackRow(
                        index: (sortedTracks.firstIndex(of: track) ?? 0) + 1,
                        track: track,
                        voteCount: track.voteCount ?? 0,
                        isMyVote: track.isVoted ?? false,
                        isOffline: store.isOffline,
                        onVote: {
                            store.send(.voteButtonTapped(trackId: track.id))
                        },
                        showVote: store.canVote
                    )
                    .matchedGeometryEffect(id: track.id, in: animation)
                    .transition(.scale.combined(with: .opacity))
                    .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                        Button(role: .destructive) {
                            store.send(.removeTrackButtonTapped(trackId: track.id))
                        } label: {
                            Label("Remove", systemImage: "trash")
                        }
                    }
                }
            }
            .animation(
                .spring(response: 0.6, dampingFraction: 0.8), value: store.tracks
            )
            .padding(.horizontal)
        }
    }
}

struct UserAlertOverlay: View {
    @Bindable var store: StoreOf<EventDetailFeature>

    var body: some View {
        if let alert = store.userAlert {
            HStack(spacing: 12) {
                Image(
                    systemName: alert.type == .error
                        ? "exclamationmark.triangle.fill"
                        : (alert.type == .success
                            ? "checkmark.circle.fill" : "info.circle.fill")
                )
                .font(.system(size: 20))
                .foregroundStyle(.white)

                VStack(alignment: .leading, spacing: 2) {
                    Text(alert.title)
                        .font(.liquidBody.bold())
                        .foregroundStyle(.white)
                    Text(alert.message)
                        .font(.liquidCaption)
                        .foregroundStyle(.white.opacity(0.8))
                }
                Spacer()
            }
            .padding()
            .background(
                (alert.type == .error
                    ? Color.red
                    : alert.type == .success
                        ? Color.green : Color.blue)
                    .opacity(0.9)
                    .shadow(.drop(radius: 10, y: 5))
            )
            .clipShape(RoundedRectangle(cornerRadius: 16))
            .padding(.horizontal)
            .padding(.top, 8)  // Add some safe area padding
            .transition(
                .asymmetric(
                    insertion: .move(edge: .top).combined(with: .opacity),
                    removal: .move(edge: .top).combined(with: .opacity)
                )
            )
            .onTapGesture {
                store.send(.dismissInfo, animation: .spring())
            }
        }
    }
}

struct EventYouTubePlayerView: View {
    @Bindable var store: StoreOf<EventDetailFeature>

    var body: some View {
        YouTubePlayerView(
            videoId: Binding(
                get: { store.currentVideoId },
                set: { _ in }
            ),
            isPlaying: Binding(
                get: {
                    // Play only if joined
                    return (store.event.isJoined ?? false)
                },
                set: { _ in }
            ),
            startSeconds: store.metadata?.playingStartedAt.map { Date().timeIntervalSince($0) }
                ?? 0,
            onEnded: {
                // Auto-next logic:
                // 1. Owner always triggers
                // 2. If no tracks queued (end of event), anyone triggers to ensure "Finished" state syncs
                let hasNextTrack = store.tracks.contains { $0.status == "queued" }
                if store.currentUserId == store.event.ownerId || !hasNextTrack {
                    store.send(.nextTrackButtonTapped)
                }
            }
        )
    }
}

struct AddTrackButtonView: View {
    @Bindable var store: StoreOf<EventDetailFeature>

    var body: some View {
        if store.canVote && !store.isEventEnded {
            LiquidButton(
                useGlass: true,
                action: {
                    if !store.isOffline {
                        store.send(.addTrackButtonTapped)
                    }
                }
            ) {
                HStack {
                    Image(systemName: "plus.circle.fill")
                        .font(.liquidIcon)
                    Text("Add Track")
                        .font(.liquidBody.bold())
                    Spacer()
                }
                .foregroundStyle(store.isOffline ? Color.gray : Color.white)
                .frame(maxWidth: .infinity)
            }
            .disabled(store.isOffline)
            .padding(.horizontal)
            .padding(.top, 20)
        }
    }
}

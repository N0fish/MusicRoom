import AppSupportClients
import ComposableArchitecture
import MusicRoomAPI
import MusicRoomDomain
import MusicRoomUI
import SearchFeature
import SwiftUI

@Reducer
public struct PlaylistDetailFeature: Sendable {
    @ObservableState
    public struct State: Equatable, Sendable {
        public var playlist: Playlist
        public var tracks: [Track] = []
        public var isLoading = false
        public var friends: [Friend] = []
        public var playingTrackId: String?
        public var isPlaying: Bool = false
        @Presents public var musicSearch: MusicSearchFeature.State?
        @Presents public var destination: Destination.State?

        public init(playlist: Playlist) {
            self.playlist = playlist
        }
    }

    public enum Action: Equatable, Sendable {
        case onAppear
        case loadPlaylist
        case playlistLoaded(TaskResult<PlaylistResponse>)
        case addTrackButtonTapped
        case inviteButtonTapped
        case musicSearch(PresentationAction<MusicSearchFeature.Action>)
        case destination(PresentationAction<Destination.Action>)
        case deleteTrackTapped(Track)
        case startRealtimeConnection
        case realtimeMessageReceived(RealtimeMessage)
        case addTrackResponse(TaskResult<Track>)
        case friendsLoaded(TaskResult<[Friend]>)
        case inviteFriendTapped(Friend)
        case inviteFriendResponse(TaskResult<Bool>)
        case togglePlayback(Track)
        case pauseTrack
        case resumeTrack
        case playbackFinished
    }

    @Dependency(\.playlistClient) var playlistClient
    @Dependency(\.friendsClient) var friendsClient
    @Dependency(\.musicRoomAPI) var musicRoomAPI

    private enum CancelID { case realtime }

    public init() {}

    public var body: some ReducerOf<Self> {
        Reduce { state, action in
            switch action {
            case .onAppear:
                return .merge(
                    .send(.loadPlaylist),
                    .send(.startRealtimeConnection)
                )

            case .loadPlaylist:
                state.isLoading = true
                return .run { [id = state.playlist.id] send in
                    await send(.playlistLoaded(TaskResult { try await playlistClient.get(id) }))
                }

            case .playlistLoaded(.success(let response)):
                state.isLoading = false
                state.playlist = response.playlist
                state.tracks = response.tracks
                return .none

            case .playlistLoaded(.failure):
                state.isLoading = false
                return .none

            case .addTrackButtonTapped:
                guard !state.playlist.isEventPlaylist else { return .none }
                state.musicSearch = MusicSearchFeature.State()
                return .none

            case .musicSearch(.presented(.delegate(.trackTapped(let item)))):
                guard !state.playlist.isEventPlaylist else {
                    state.musicSearch = nil
                    return .none
                }
                state.musicSearch = nil
                state.isLoading = true
                let request = AddTrackRequest(
                    title: item.title,
                    artist: item.artist,
                    provider: "youtube",
                    providerTrackId: item.providerTrackId,
                    thumbnailUrl: item.thumbnailUrl?.absoluteString ?? "",
                    durationMs: item.durationMs
                )
                return .run { [playlistId = state.playlist.id] send in
                    await send(
                        .addTrackResponse(
                            TaskResult { try await playlistClient.addTrack(playlistId, request) }))
                }

            case .addTrackResponse(.success):
                // Real-time update should handle the reload, but we can also manually reload
                return .send(.loadPlaylist)

            case .addTrackResponse(.failure):
                state.isLoading = false
                return .none

            case .musicSearch:
                return .none

            case .inviteButtonTapped:
                state.isLoading = true
                return .run { send in
                    await send(.friendsLoaded(TaskResult { try await friendsClient.listFriends() }))
                }

            case .friendsLoaded(.success(let friends)):
                state.isLoading = false
                state.friends = friends
                state.destination = .inviteSheet(FriendsList(friends: friends))
                return .none

            case .friendsLoaded(.failure):
                state.isLoading = false
                // Handle error properly in a real app
                return .none

            case .inviteFriendTapped(let friend):
                state.destination = nil  // Dismiss sheet immediately
                return .run { [playlistId = state.playlist.id, userId = friend.userId] send in
                    print("Attempting to invite user \(userId) to playlist \(playlistId)")
                    await send(
                        .inviteFriendResponse(
                            TaskResult {
                                try await playlistClient.addInvite(playlistId, userId)
                                return true
                            }))
                }

            case .inviteFriendResponse(.success):
                print("Invite sent successfully")
                return .none

            case .inviteFriendResponse(.failure(let error)):
                print("Failed to send invite: \(error)")
                // In a real app, we might want to show an alert here, but since the sheet is gone,
                // we'd need a different mechanism (toast/banner).
                return .none

            case .destination:
                return .none

            case .deleteTrackTapped(let track):
                guard !state.playlist.isEventPlaylist else { return .none }
                // Optimistic removal
                state.tracks.removeAll { $0.id == track.id }
                return .run { [playlistId = state.playlist.id, trackId = track.id] send in
                    _ = try await playlistClient.deleteTrack(playlistId, trackId)
                }

            case .startRealtimeConnection:
                return .run { send in
                    for await message in musicRoomAPI.connectToRealtime() {
                        await send(.realtimeMessageReceived(message))
                    }
                }
                .cancellable(id: CancelID.realtime, cancelInFlight: true)

            case .realtimeMessageReceived(let msg):
                switch msg.type {
                case "track.added", "track.deleted", "playlist.reorder", "playlist.updated",
                    "track.updated":
                    // Check if it's for this playlist
                    // In a more complex app, we'd check if payload.playlistId == state.playlist.id
                    // But for now, any playlist update triggers reload if we are viewing one.
                    return .send(.loadPlaylist)
                default:
                    return .none
                }

            case .togglePlayback(let track):
                if state.playingTrackId == track.id {
                    state.isPlaying.toggle()
                } else {
                    state.playingTrackId = track.id
                    state.isPlaying = true
                }
                return .none

            case .pauseTrack:
                state.isPlaying = false
                return .none

            case .resumeTrack:
                if state.playingTrackId != nil {
                    state.isPlaying = true
                }
                return .none
            case .playbackFinished:
                guard let currentId = state.playingTrackId,
                    let currentIndex = state.tracks.firstIndex(where: { $0.id == currentId })
                else {
                    state.isPlaying = false
                    state.playingTrackId = nil
                    return .none
                }

                let nextIndex = currentIndex + 1
                if nextIndex < state.tracks.count {
                    state.playingTrackId = state.tracks[nextIndex].id
                    state.isPlaying = true
                } else {
                    state.isPlaying = false
                    state.playingTrackId = nil
                }
                return .none
            }
        }
        .ifLet(\.$musicSearch, action: \.musicSearch) {
            MusicSearchFeature()
        }
        .ifLet(\.$destination, action: \.destination) {
            Destination()
        }
    }
}

extension PlaylistDetailFeature {
    @Reducer
    public struct Destination: Sendable {
        @ObservableState
        public enum State: Equatable, Sendable, Identifiable {
            case inviteSheet(FriendsList)

            public var id: UUID {
                switch self {
                case .inviteSheet(let list): return list.id
                }
            }
        }

        public enum Action: Equatable, Sendable {
            case inviteSheet(Never)
        }

        public var body: some ReducerOf<Self> {
            EmptyReducer()
        }
    }

    @ObservableState
    public struct FriendsList: Equatable, Identifiable, Sendable {
        public let id = UUID()
        public let friends: [Friend]
    }
}

public struct PlaylistDetailView: View {
    @Bindable var store: StoreOf<PlaylistDetailFeature>

    public init(store: StoreOf<PlaylistDetailFeature>) {
        self.store = store
    }

    public var body: some View {
        let isEventPlaylistReadOnly = store.playlist.isEventPlaylist
        ZStack {
            LiquidBackground()
                .ignoresSafeArea()

            // Hidden YouTube Player for Playlists
            // Hidden YouTube Player for Playlists
            PlaylistYouTubePlayerView(store: store)
                .frame(width: 1, height: 1)
                .opacity(0.01)

            viewContent
        }
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                HStack {
                    if !store.playlist.isPublic {
                        Button {
                            store.send(.inviteButtonTapped)
                        } label: {
                            Image(systemName: "person.crop.circle.badge.plus")
                                .font(.title2)
                                .foregroundColor(.white)
                        }
                    }

                    if !isEventPlaylistReadOnly {
                        Button {
                            store.send(.addTrackButtonTapped)
                        } label: {
                            Image(systemName: "plus.circle.fill")
                                .font(.title2)
                                .foregroundColor(.white)
                        }
                    }
                }
            }
        }
        .onAppear {
            store.send(.onAppear)
        }
        .sheet(item: $store.scope(state: \.musicSearch, action: \.musicSearch)) { searchStore in
            MusicSearchView(store: searchStore)
        }
        .sheet(
            item: $store.scope(state: \.destination?.inviteSheet, action: \.destination.inviteSheet)
        ) { friendsStore in
            InviteFriendSheet(friends: friendsStore.friends) { friend in
                store.send(.inviteFriendTapped(friend))
            }
        }
    }

    @ViewBuilder
    private var viewContent: some View {
        VStack {
            if store.playlist.isEventPlaylist {
                HStack(spacing: 8) {
                    Image(systemName: "calendar")
                    Text("Event Playlist - Read-only")
                        .fontWeight(.semibold)
                }
                .font(.caption)
                .foregroundColor(.white.opacity(0.85))
                .padding(.horizontal, 12)
                .padding(.vertical, 6)
                .background(Color.white.opacity(0.15))
                .clipShape(Capsule())
                .padding(.top, 8)
            }

            if store.isLoading && store.tracks.isEmpty {
                VStack {
                    Spacer()
                    ProgressView()
                        .tint(.white)
                        .scaleEffect(1.5)
                    Spacer()
                }
            } else if store.tracks.isEmpty {
                VStack(spacing: 20) {
                    Spacer()
                    Image(systemName: "music.note.list")
                        .font(.system(size: 80))
                        .foregroundColor(.white.opacity(0.2))
                    Text("No tracks yet")
                        .font(.headline)
                        .foregroundColor(.white.opacity(0.5))
                    if store.playlist.isEventPlaylist {
                        Text("Event playlists are read-only.")
                            .font(.subheadline)
                            .foregroundColor(.white.opacity(0.5))
                    } else {
                        Button("Add Tracks") {
                            store.send(.addTrackButtonTapped)
                        }
                        .buttonStyle(.bordered)
                        .tint(.white)
                    }
                    Spacer()
                }
            } else {
                ScrollView {
                    LazyVStack(spacing: 12) {
                        ForEach(store.tracks) { track in
                            TrackRow(
                                track: track,
                                isPlaying: store.playingTrackId == track.id && store.isPlaying,
                                isCurrent: store.playingTrackId == track.id,
                                onPlayPause: {
                                    store.send(.togglePlayback(track))
                                },
                                onDelete: store.playlist.isEventPlaylist
                                    ? nil
                                    : {
                                        store.send(.deleteTrackTapped(track))
                                    }
                            )
                        }
                    }
                    .padding()
                }
            }
        }
    }
}

struct InviteFriendSheet: View {
    let friends: [Friend]
    let onSelect: (Friend) -> Void

    @Environment(\.dismiss) var dismiss

    var body: some View {
        NavigationStack {
            List(friends) { friend in
                Button {
                    onSelect(friend)
                } label: {
                    HStack(spacing: 12) {
                        PremiumAvatarView(
                            url: friend.avatarUrl,
                            isPremium: friend.isPremium,
                            size: 50
                        )

                        VStack(alignment: .leading, spacing: 4) {
                            Text(friend.displayName)
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
                    .contentShape(Rectangle())  // Ensure entire row is tappable
                }
                .buttonStyle(.plain)  // Standard list row behavior
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

extension Friend {
    var initials: String {
        let names = displayName.split(separator: " ")
        if let first = names.first?.first, let last = names.last?.first {
            return "\(first)\(last)"
        }
        return String(displayName.prefix(2))
    }
}

struct TrackRow: View {
    let track: Track
    var isPlaying: Bool = false
    var isCurrent: Bool = false
    var onPlayPause: () -> Void = {}
    var onDelete: (() -> Void)?

    var body: some View {
        GlassView {
            HStack(spacing: 12) {
                if let url = track.thumbnailUrl {
                    AsyncImage(url: url) { image in
                        image.resizable()
                            .aspectRatio(contentMode: .fill)
                    } placeholder: {
                        Color.white.opacity(0.1)
                    }
                    .frame(width: 50, height: 50)
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                } else {
                    RoundedRectangle(cornerRadius: 8)
                        .fill(Color.white.opacity(0.1))
                        .frame(width: 50, height: 50)
                        .overlay(
                            Image(systemName: "music.note").foregroundColor(.white.opacity(0.3)))
                }

                VStack(alignment: .leading, spacing: 4) {
                    Text(track.title)
                        .font(.headline)
                        .foregroundColor(isCurrent ? .accentColor : .white)
                        .lineLimit(1)
                    Text(track.artist)
                        .font(.subheadline)
                        .foregroundColor(.white.opacity(0.7))
                        .lineLimit(1)
                }

                Spacer()

                Button(action: onPlayPause) {
                    Image(systemName: isPlaying ? "pause.circle.fill" : "play.circle.fill")
                        .font(.system(size: 32))
                        .foregroundColor(.white)
                }
                .padding(.trailing, 8)

                if let onDelete {
                    Button(action: onDelete) {
                        Image(systemName: "trash")
                            .foregroundColor(.red.opacity(0.7))
                    }
                }
            }
            .padding(12)
        }
    }
}

struct PlaylistYouTubePlayerView: View {
    let store: StoreOf<PlaylistDetailFeature>

    var body: some View {
        @Bindable var store = store
        if let videoId = store.playingTrackId,
            let track = store.tracks.first(where: { $0.id == videoId })
        {
            YouTubePlayerView(
                videoId: Binding(
                    get: { Optional(track.providerTrackId) },
                    set: { _ in }
                ),
                isPlaying: Binding(
                    get: { store.isPlaying },
                    set: { isPlaying in
                        if isPlaying {
                            store.send(.resumeTrack)
                        } else {
                            store.send(.pauseTrack)
                        }
                    }
                ),
                onEnded: {
                    store.send(.playbackFinished)
                }
            )
        }
    }
}

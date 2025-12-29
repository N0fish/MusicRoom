import AppSupportClients
import ComposableArchitecture
import MusicRoomAPI
import MusicRoomDomain
import MusicRoomUI
import SwiftUI

@Reducer
public struct PlaylistListFeature: Sendable {
    @ObservableState
    public struct State: Equatable, Sendable {
        public var playlists: [Playlist] = []
        public var isLoading = false
        public var errorMessage: String?
        public var path = StackState<PlaylistDetailFeature.State>()
        @Presents public var createPlaylist: CreatePlaylistFeature.State?
        public var currentUserId: String?

        public init() {}
    }

    public enum Action: Equatable, Sendable {
        case onAppear
        case loadPlaylists
        case playlistsLoaded(TaskResult<[Playlist]>)
        case playlistTapped(Playlist)
        case createPlaylistButtonTapped
        case createPlaylist(PresentationAction<CreatePlaylistFeature.Action>)
        case path(StackAction<PlaylistDetailFeature.State, PlaylistDetailFeature.Action>)
        case startRealtimeConnection
        case realtimeMessageReceived(RealtimeMessage)
        case fetchCurrentUser
        case currentUserLoaded(TaskResult<String>)
        case deletePlaylist(Playlist)
        case playlistDeleted(TaskResult<String>)
    }

    private enum CancelID { case realtime }

    @Dependency(\.playlistClient) var playlistClient
    @Dependency(\.musicRoomAPI) var musicRoomAPI

    public init() {}

    public var body: some ReducerOf<Self> {
        Reduce { state, action in
            switch action {
            case .onAppear:
                return .merge(
                    .send(.loadPlaylists),
                    .send(.startRealtimeConnection),
                    .send(.fetchCurrentUser)
                )

            case .loadPlaylists:
                state.isLoading = true
                state.errorMessage = nil
                return .run { send in
                    await send(.playlistsLoaded(TaskResult { try await playlistClient.list() }))
                }

            case .playlistsLoaded(.success(let playlists)):
                state.isLoading = false
                state.playlists = playlists
                return .none

            case .playlistsLoaded(.failure(let error)):
                state.isLoading = false
                state.errorMessage = error.localizedDescription
                return .none

            case .playlistTapped(let playlist):
                state.path.append(PlaylistDetailFeature.State(playlist: playlist))
                return .none

            case .createPlaylistButtonTapped:
                state.createPlaylist = CreatePlaylistFeature.State()
                return .none

            case .createPlaylist(.presented(.delegate(.playlistCreated(let playlist)))):
                state.playlists.insert(playlist, at: 0)
                return .none

            case .createPlaylist:
                return .none

            case .path:
                return .none

            case .startRealtimeConnection:
                return .run { send in
                    for await message in musicRoomAPI.connectToRealtime() {
                        await send(.realtimeMessageReceived(message))
                    }
                }
                .cancellable(id: CancelID.realtime, cancelInFlight: true)

            case .realtimeMessageReceived(let msg):
                switch msg.type {
                case "playlist.invited":
                    // Reload playlists when invited
                    return .send(.loadPlaylists)
                case "playlist.deleted":
                    if let dict = msg.payload.value as? [String: Any],
                        let playlistId = dict["playlistId"] as? String
                    {
                        state.playlists.removeAll { $0.id == playlistId }
                    }
                    return .none
                default:
                    return .none
                }

            case .fetchCurrentUser:
                return .run { send in
                    await send(
                        .currentUserLoaded(
                            TaskResult {
                                let response = try await musicRoomAPI.authMe()
                                return response.userId
                            }))
                }

            case .currentUserLoaded(.success(let userId)):
                state.currentUserId = userId
                return .none

            case .currentUserLoaded(.failure):
                return .none

            case .deletePlaylist(let playlist):
                return .run { send in
                    await send(
                        .playlistDeleted(
                            TaskResult {
                                try await playlistClient.delete(playlist.id)
                                return playlist.id
                            }))
                }

            case .playlistDeleted(.success(let playlistId)):
                state.playlists.removeAll { $0.id == playlistId }
                return .none

            case .playlistDeleted(.failure(let error)):
                state.errorMessage = "Failed to delete playlist: \(error.localizedDescription)"
                return .none
            }
        }
        .forEach(\.path, action: \.path) {
            PlaylistDetailFeature()
        }
        .ifLet(\.$createPlaylist, action: \.createPlaylist) {
            CreatePlaylistFeature()
        }
    }
}

public struct PlaylistListView: View {
    @Bindable var store: StoreOf<PlaylistListFeature>

    public init(store: StoreOf<PlaylistListFeature>) {
        self.store = store
    }

    public var body: some View {
        NavigationStack(path: $store.scope(state: \.path, action: \.path)) {
            ZStack {
                LiquidBackground()
                    .ignoresSafeArea()

                VStack {
                    if store.isLoading && store.playlists.isEmpty {
                        ProgressView()
                            .tint(.white)
                    } else if let error = store.errorMessage {
                        VStack {
                            Text("Error: \(error)")
                                .foregroundColor(.red)
                            Button("Retry") {
                                store.send(.loadPlaylists)
                            }
                            .buttonStyle(.bordered)
                        }
                    } else {
                        List {
                            ForEach(store.playlists) { playlist in
                                playlistRow(playlist)
                            }
                        }
                        .listStyle(.plain)
                        .scrollContentBackground(.hidden)
                    }
                }
            }
            .navigationTitle("Playlists")
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button {
                        store.send(.createPlaylistButtonTapped)
                    } label: {
                        Image(systemName: "plus.circle.fill")
                            .font(.title2)
                    }
                }
            }
            .onAppear {
                store.send(.onAppear)
            }
            .sheet(item: $store.scope(state: \.createPlaylist, action: \.createPlaylist)) {
                createStore in
                CreatePlaylistView(store: createStore)
            }
        } destination: { store in
            PlaylistDetailView(store: store)
        }
    }

    @ViewBuilder
    private func playlistRow(_ playlist: Playlist) -> some View {
        let canDelete =
            playlist.ownerId == store.currentUserId && !playlist.isEventPlaylist

        let row = PlaylistRow(playlist: playlist)
            .listRowBackground(Color.clear)
            .listRowSeparator(.hidden)
            .listRowInsets(EdgeInsets(top: 8, leading: 16, bottom: 8, trailing: 16))
            .contentShape(Rectangle())
            .onTapGesture {
                store.send(.playlistTapped(playlist))
            }

        if canDelete {
            row.swipeActions(edge: .trailing, allowsFullSwipe: true) {
                Button(role: .destructive) {
                    store.send(.deletePlaylist(playlist))
                } label: {
                    Label("Delete", systemImage: "trash")
                }
                .tint(.red)
            }
        } else {
            row
        }
    }
}

struct PlaylistRow: View {
    let playlist: Playlist

    var body: some View {
        GlassView {
            HStack {
                VStack(alignment: .leading, spacing: 4) {
                    Text(playlist.name)
                        .font(.headline)
                        .foregroundColor(.white)

                    if playlist.isEventPlaylist {
                        Text("Event Playlist - Read-only")
                            .font(.caption)
                            .foregroundColor(.white.opacity(0.7))
                    } else if !playlist.description.isEmpty {
                        Text(playlist.description)
                            .font(.subheadline)
                            .foregroundColor(.white.opacity(0.7))
                            .lineLimit(1)
                    }

                    HStack {
                        Image(systemName: playlist.isPublic ? "globe" : "lock.fill")
                        Text(playlist.editMode == "everyone" ? "Collaborative" : "Private Edit")
                    }
                    .font(.caption)
                    .foregroundColor(.white.opacity(0.5))
                }

                Spacer()

                Image(systemName: "chevron.right")
                    .foregroundColor(.white.opacity(0.3))
            }
            .padding()
        }
    }
}

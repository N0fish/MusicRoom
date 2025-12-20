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
                    .send(.startRealtimeConnection)
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
                default:
                    return .none
                }
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
                        ScrollView {
                            LazyVStack(spacing: 16) {
                                ForEach(store.playlists) { playlist in
                                    Button {
                                        store.send(.playlistTapped(playlist))
                                    } label: {
                                        PlaylistRow(playlist: playlist)
                                    }
                                }
                            }
                            .padding()
                        }
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

                    if !playlist.description.isEmpty {
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

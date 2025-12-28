import AppSupportClients
import ComposableArchitecture
import MusicRoomDomain
import SwiftUI

@Reducer
public struct CreatePlaylistFeature: Sendable {
    public struct State: Equatable, Sendable {
        public var name = ""
        public var description = ""
        public var isPublic = true
        public var editMode = "everyone"
        public var isSaving = false

        public init() {}
    }

    public enum Action: Equatable, Sendable {
        case nameChanged(String)
        case descriptionChanged(String)
        case isPublicChanged(Bool)
        case editModeChanged(String)
        case saveButtonTapped
        case createResponse(TaskResult<Playlist>)
        case delegate(Delegate)

        public enum Delegate: Equatable, Sendable {
            case playlistCreated(Playlist)
        }
    }

    @Dependency(\.playlistClient) var playlistClient
    @Dependency(\.dismiss) var dismiss

    public init() {}

    public var body: some ReducerOf<Self> {
        Reduce { state, action in
            switch action {
            case .nameChanged(let name):
                state.name = name
                return .none
            case .descriptionChanged(let desc):
                state.description = desc
                return .none
            case .isPublicChanged(let isPublic):
                state.isPublic = isPublic
                if !isPublic {
                    state.editMode = "invited"
                }
                return .none
            case .editModeChanged(let mode):
                state.editMode = mode
                return .none
            case .saveButtonTapped:
                state.isSaving = true
                let request = CreatePlaylistRequest(
                    name: state.name,
                    description: state.description,
                    isPublic: state.isPublic,
                    editMode: state.editMode
                )
                return .run { send in
                    await send(
                        .createResponse(TaskResult { try await playlistClient.create(request) }))
                }
            case .createResponse(.success(let playlist)):
                state.isSaving = false
                return .run { send in
                    await send(.delegate(.playlistCreated(playlist)))
                    await dismiss()
                }
            case .createResponse(.failure):
                state.isSaving = false
                return .none
            case .delegate:
                return .none
            }
        }
    }
}

public struct CreatePlaylistView: View {
    let store: StoreOf<CreatePlaylistFeature>

    public init(store: StoreOf<CreatePlaylistFeature>) {
        self.store = store
    }

    public var body: some View {
        WithViewStore(store, observe: { $0 }) { viewStore in
            NavigationStack {
                Form {
                    Section("Information") {
                        TextField(
                            "Name", text: viewStore.binding(get: \.name, send: { .nameChanged($0) })
                        )
                        TextField(
                            "Description",
                            text: viewStore.binding(
                                get: \.description, send: { .descriptionChanged($0) }))
                    }

                    Section("Settings") {
                        Toggle(
                            "Public",
                            isOn: viewStore.binding(get: \.isPublic, send: { .isPublicChanged($0) })
                        )
                        if viewStore.isPublic {
                            Picker(
                                "Who can add tracks?",
                                selection: viewStore.binding(
                                    get: \.editMode, send: { .editModeChanged($0) })
                            ) {
                                Text("Everyone").tag("everyone")
                                Text("Invited Only").tag("invited")
                            }
                        }
                    }
                }
                .navigationTitle("New Playlist")
                .toolbar {
                    ToolbarItem(placement: .confirmationAction) {
                        Button("Create") {
                            viewStore.send(.saveButtonTapped)
                        }
                        .disabled(viewStore.name.isEmpty || viewStore.isSaving)
                    }
                }
            }
        }
    }
}

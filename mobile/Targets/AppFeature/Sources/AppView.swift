import AuthenticationFeature
import ComposableArchitecture
import EventFeature
import MusicRoomDomain
import PlaylistFeature
import SettingsFeature
import SwiftUI

public struct AppView: View {
    private let store: StoreOf<AppFeature>

    private struct ViewState: Equatable {
        let destination: AppFeature.State.Destination
        let isSettingsPresented: Bool
    }

    public init(store: StoreOf<AppFeature>) {
        self.store = store
    }

    public var body: some View {
        WithViewStore(
            store,
            observe: { ViewState(destination: $0.destination, isSettingsPresented: $0.isSettingsPresented) }
        ) { viewStore in
            ShakeDetectingView(onShake: { viewStore.send(.shakeDetected, animation: .default) }) {
                Group {
                    switch viewStore.state.destination {
                    case .login:
                        AuthenticationView(
                            store: store.scope(
                                state: \.authentication,
                                action: \.authentication
                            )
                        )
                    case .app:
                        appContent
                    case .splash:
                        SplashView()
                    }
                }
            }
            .task {
                await viewStore.send(.task).finish()
            }
            .sheet(
                isPresented: viewStore.binding(
                    get: { $0.isSettingsPresented },
                    send: AppFeature.Action.setSettingsPresented
                )
            ) {
                NavigationStack {
                    SettingsView(
                        store: store.scope(
                            state: \.settings,
                            action: \.settings
                        )
                    )
                }
                .presentationDetents([.medium, .large])
                .presentationDragIndicator(.visible)
            }
        }
        .ignoresSafeArea()
    }

    private var appContent: some View {
        TabView {
            // Tab 1: Events
            EventListView(
                store: store.scope(
                    state: \.eventList,
                    action: \.eventList
                )
            )
            .tabItem {
                Label("Events", systemImage: "music.note.list")
            }

            // Tab 2: Playlists
            PlaylistListView(
                store: store.scope(
                    state: \.playlistList,
                    action: \.playlistList
                )
            )
            .tabItem {
                Label("Playlists", systemImage: "music.note")
            }

            // Tab 3: Profile
            NavigationStack {
                ProfileView(
                    store: store.scope(
                        state: \.profile,
                        action: \.profile
                    )
                )
            }
            .tabItem {
                Label("Profile", systemImage: "person.crop.circle")
            }

            // Tab 4: Friends
            FriendsView(
                store: store.scope(state: \.friends, action: \.friends)
            )
            .tabItem {
                Label("Friends", systemImage: "person.2.fill")
            }
        }
        // Assuming .liquidAccent is defined or available globally, otherwise it would need to be defined.
        .tint(.liquidAccent)  // Consistent styling
    }
}

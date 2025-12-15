import AuthenticationFeature
import ComposableArchitecture
import EventFeature
import MusicRoomDomain
import SettingsFeature
import SwiftUI

public struct AppView: View {
    private let store: StoreOf<AppFeature>

    public init(store: StoreOf<AppFeature>) {
        self.store = store
    }

    public var body: some View {
        WithViewStore(store, observe: { $0.destination }) { viewStore in
            Group {
                switch viewStore.state {
                case .login:
                    AuthenticationView(
                        store: store.scope(
                            state: \.authentication,
                            action: \.authentication
                        )
                    )
                case .app:
                    NavigationStack {
                        appContent
                    }
                }
            }
            .task {
                await viewStore.send(.task).finish()
            }
        }
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

            // Tab 2: Profile
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

            // Tab 3: Settings
            NavigationStack {
                SettingsView(
                    store: store.scope(
                        state: \.settings,
                        action: \.settings
                    )
                )
            }
            .tabItem {
                Label("Settings", systemImage: "gear")
            }
        }
        // Assuming .liquidAccent is defined or available globally, otherwise it would need to be defined.
        .tint(.liquidAccent)  // Consistent styling
    }
}

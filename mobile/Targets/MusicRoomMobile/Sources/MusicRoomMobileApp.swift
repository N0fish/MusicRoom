import AppFeature
import ComposableArchitecture
import SwiftUI

@main
struct MusicRoomMobileApp: App {
    private let store = Store(initialState: AppFeature.State()) {
        AppFeature()
    }

    var body: some Scene {
        WindowGroup {
            AppView(store: store)
                .onOpenURL { url in
                    store.send(.handleDeepLink(url))
                }
        }
    }
}

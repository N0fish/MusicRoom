import SwiftUI
import ComposableArchitecture
import AppFeature

@main
struct MusicRoomMobileApp: App {
    private let store = Store(initialState: AppFeature.State()) {
        AppFeature()
    }

    var body: some Scene {
        WindowGroup {
            AppView(store: store)
        }
    }
}

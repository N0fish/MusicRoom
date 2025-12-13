import AuthenticationFeature
import ComposableArchitecture
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
                        WithViewStore(store, observe: { $0 }) { appViewStore in
                            List {
                                Section("Environment") {
                                    NavigationLink {
                                        SettingsView(
                                            store: store.scope(
                                                state: \.settings,
                                                action: \.settings
                                            )
                                        )
                                    } label: {
                                        VStack(alignment: .leading, spacing: 4) {
                                            Label(
                                                "Backend Settings",
                                                systemImage: "antenna.radiowaves.left.and.right")
                                            Text(appViewStore.settings.backendURLSummary)
                                                .font(.footnote)
                                                .foregroundStyle(.secondary)
                                                .lineLimit(1)
                                        }
                                    }
                                }

                                Section("Sample Data Preview") {
                                    if appViewStore.isSampleDataLoading {
                                        ProgressView("Loading mock events…")
                                    } else if let error = appViewStore.sampleDataError {
                                        Text("Error: \(error)")
                                            .foregroundStyle(.red)
                                    } else {
                                        ForEach(appViewStore.sampleEvents) { event in
                                            VStack(alignment: .leading) {
                                                Text(event.name)
                                                    .font(.headline)
                                                Text(
                                                    "\(event.licenseTier.label) · \(event.visibility.label)"
                                                )
                                                .font(.footnote)
                                                .foregroundStyle(.secondary)
                                            }
                                        }
                                    }

                                    VStack(alignment: .leading, spacing: 4) {
                                        Label(
                                            appViewStore.policySummary,
                                            systemImage: "checkmark.shield"
                                        )
                                        .font(.subheadline)
                                        Label(
                                            appViewStore.latestStreamMessage,
                                            systemImage: "dot.radiowaves.left.and.right"
                                        )
                                        .font(.subheadline)
                                        .foregroundStyle(.secondary)
                                    }
                                }

                                Section {
                                    Button("Logout", role: .destructive) {
                                        appViewStore.send(.logoutButtonTapped)
                                    }
                                }
                            }
                            .navigationTitle("Music Room")
                        }
                    }
                }
            }
            .task {
                await viewStore.send(.task).finish()
            }
        }
    }
}

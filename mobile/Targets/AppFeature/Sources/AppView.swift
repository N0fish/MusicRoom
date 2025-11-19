import SwiftUI
import ComposableArchitecture
import SettingsFeature
import MusicRoomDomain

public struct AppView: View {
    private let store: StoreOf<AppFeature>

    public init(store: StoreOf<AppFeature>) {
        self.store = store
    }

    public var body: some View {
        NavigationStack {
            WithViewStore(store, observe: { $0 }) { viewStore in
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
                                Label("Backend Settings", systemImage: "antenna.radiowaves.left.and.right")
                                Text(viewStore.settings.backendURLSummary)
                                    .font(.footnote)
                                    .foregroundStyle(.secondary)
                                    .lineLimit(1)
                            }
                        }
                    }

                    Section("Sample Data Preview") {
                        if viewStore.isSampleDataLoading {
                            ProgressView("Loading mock events…")
                        } else if let error = viewStore.sampleDataError {
                            Text("Error: \(error)")
                                .foregroundStyle(.red)
                        } else {
                            ForEach(viewStore.sampleEvents) { event in
                                VStack(alignment: .leading) {
                                    Text(event.name)
                                        .font(.headline)
                                    Text("\(event.licenseTier.label) · \(event.visibility.label)")
                                        .font(.footnote)
                                        .foregroundStyle(.secondary)
                                }
                            }
                        }

                        VStack(alignment: .leading, spacing: 4) {
                            Label(viewStore.policySummary, systemImage: "checkmark.shield")
                                .font(.subheadline)
                            Label(viewStore.latestStreamMessage, systemImage: "dot.radiowaves.left.and.right")
                                .font(.subheadline)
                                .foregroundStyle(.secondary)
                        }
                    }
                }
                .navigationTitle("Music Room")
                .task {
                    await viewStore.send(.task).finish()
                }
            }
        }
    }
}

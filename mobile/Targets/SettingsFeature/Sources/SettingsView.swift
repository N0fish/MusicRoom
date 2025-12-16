import AppSettingsClient
import ComposableArchitecture
import SwiftUI

public struct SettingsView: View {
    private let store: StoreOf<SettingsFeature>
    @FocusState private var isURLFieldFocused: Bool

    public init(store: StoreOf<SettingsFeature>) {
        self.store = store
    }

    public var body: some View {
        WithViewStore(store, observe: { $0 }) { viewStore in
            Form {
                Section {
                    Picker(
                        "Preset",
                        selection: viewStore.binding(
                            get: \SettingsFeature.State.selectedPreset,
                            send: SettingsFeature.Action.presetChanged
                        )
                    ) {
                        ForEach(
                            BackendEnvironmentPreset.allCases, id: \BackendEnvironmentPreset.self
                        ) { preset in
                            Text(preset.title).tag(preset)
                        }
                    }
                    .pickerStyle(.segmented)

                    Text(viewStore.environmentNote)
                        .font(.footnote)
                        .foregroundStyle(.secondary)
                } header: {
                    Text("Environment Preset")
                }

                Section {
                    TextField(
                        "https://api.musicroom.app",
                        text: viewStore.binding(
                            get: \SettingsFeature.State.backendURLText,
                            send: SettingsFeature.Action.backendURLTextChanged
                        )
                    )
                    .keyboardType(.URL)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .focused($isURLFieldFocused)
                    .disabled(!viewStore.canEditBackendURL)

                    if viewStore.isLoading {
                        ProgressView()
                            .frame(maxWidth: .infinity, alignment: .leading)
                    } else {
                        Text("Saved: \(viewStore.backendURLSummary)")
                            .font(.footnote)
                            .foregroundStyle(.secondary)
                    }
                } header: {
                    Text("Backend API")
                } footer: {
                    if !viewStore.canEditBackendURL {
                        Text("Switch to Hosted to edit the URL manually.")
                    }
                }

                Section {
                    if viewStore.isDiagnosticsInFlight {
                        ProgressView("Running connection test…")
                            .frame(maxWidth: .infinity, alignment: .leading)
                    } else {
                        VStack(spacing: 8) {
                            HStack {
                                Label("API", systemImage: "network")
                                Spacer()
                                Text(viewStore.apiStatusText)
                                    .font(.subheadline)
                                    .foregroundStyle(
                                        viewStore.apiStatusColor == "green"
                                            ? .green
                                            : (viewStore.apiStatusColor == "red"
                                                ? .red : .secondary))
                            }
                            HStack {
                                Label("Realtime", systemImage: "bolt.horizontal")
                                Spacer()
                                Text(viewStore.wsStatusText)
                                    .font(.subheadline)
                                    .foregroundStyle(
                                        viewStore.wsStatusColor == "green"
                                            ? .green
                                            : (viewStore.wsStatusColor == "red" ? .red : .secondary)
                                    )
                            }
                        }
                    }

                    Button {
                        viewStore.send(.runConnectionTest, animation: .default)
                    } label: {
                        Label("Run Connection Test", systemImage: "dot.radiowaves.left.and.right")
                    }
                    .disabled(viewStore.isDiagnosticsInFlight || viewStore.isLoading)
                } header: {
                    Text("Diagnostics")
                } footer: {
                    Text(
                        "Checks connectivity to the /health API endpoint and the /ws WebSocket service."
                    )
                }

                Section {
                    HStack {
                        Label("Version", systemImage: "info.circle")
                        Spacer()
                        Text(viewStore.versionSummary)
                            .font(.subheadline)
                            .foregroundStyle(.secondary)
                    }
                } header: {
                    Text("App Info")
                }

                Section {
                    Button {
                        viewStore.send(.saveButtonTapped, animation: .default)
                    } label: {
                        Label("Save", systemImage: "externaldrive.connected.to.line.below")
                    }
                    .disabled(viewStore.isPersisting || viewStore.isLoading)

                    Button(role: .destructive) {
                        viewStore.send(.resetButtonTapped, animation: .default)
                    } label: {
                        Label("Reset to Default", systemImage: "arrow.counterclockwise.circle")
                    }
                    .disabled(viewStore.isPersisting)
                } header: {
                    Text("Actions")
                } footer: {
                    Text(
                        "Point the remote-control client to staging, local, or production without recompiling."
                    )
                }
            }
            .navigationTitle("Backend Settings")
            .task {
                await viewStore.send(.task).finish()
            }
            .alert(
                store: store.scope(
                    state: \.$alert,
                    action: \.alert
                )
            )
        }
    }
}

#Preview {
    NavigationStack {
        SettingsView(
            store: Store(initialState: SettingsFeature.State()) {
                SettingsFeature()
            }
        )
    }
}

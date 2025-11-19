import SwiftUI
import ComposableArchitecture
import AppSettingsClient

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
                        ForEach(BackendEnvironmentPreset.allCases, id: \BackendEnvironmentPreset.self) { preset in
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
                        Text("Switch to Custom to edit the URL manually.")
                    }
                }

                Section {
                    if viewStore.isDiagnosticsInFlight {
                        ProgressView("Running mock ping…")
                            .frame(maxWidth: .infinity, alignment: .leading)
                    } else {
                        HStack {
                            Label("Status", systemImage: "waveform.badge.exclamationmark")
                            Spacer()
                            Text(viewStore.diagnosticsDescription)
                                .multilineTextAlignment(.trailing)
                                .font(.subheadline)
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
                    Text("Mocks a /health ping so QA can verify URLs before the backend is online.")
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
                    Text("Point the remote-control client to staging, local, or production without recompiling.")
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

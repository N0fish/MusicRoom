import Foundation
import ComposableArchitecture
import AppSettingsClient
import AppSupportClients

@Reducer
public struct SettingsFeature {
    public struct State: Equatable {
        public var backendURLText: String
        public var savedBackendURL: URL?
        public var selectedPreset: BackendEnvironmentPreset
        public var lastCustomURLText: String
        public var lastPingedURL: URL?
        public var diagnosticsSummary: DiagnosticsSummary?
        public var metadata: AppMetadata?
        public var isLoading: Bool
        public var isPersisting: Bool
        public var isDiagnosticsInFlight: Bool
        @PresentationState public var alert: AlertState<Alert>?

        public init(
            backendURLText: String = "",
            savedBackendURL: URL? = nil,
            selectedPreset: BackendEnvironmentPreset = .local,
            lastCustomURLText: String = BackendEnvironmentPreset.local.defaultURL.absoluteString,
            lastPingedURL: URL? = nil,
            diagnosticsSummary: DiagnosticsSummary? = nil,
            metadata: AppMetadata? = nil,
            isLoading: Bool = false,
            isPersisting: Bool = false,
            isDiagnosticsInFlight: Bool = false
        ) {
            self.backendURLText = backendURLText
            self.savedBackendURL = savedBackendURL
            self.selectedPreset = selectedPreset
            self.lastCustomURLText = lastCustomURLText
            self.lastPingedURL = lastPingedURL
            self.diagnosticsSummary = diagnosticsSummary
            self.metadata = metadata
            self.isLoading = isLoading
            self.isPersisting = isPersisting
            self.isDiagnosticsInFlight = isDiagnosticsInFlight
        }

        public var backendURLSummary: String {
            savedBackendURL?.absoluteString ?? "Not configured"
        }

        public var canEditBackendURL: Bool { selectedPreset == .custom }

        public var environmentNote: String { selectedPreset.note }

        public var versionSummary: String {
            metadata?.summary ?? "Collecting app info…"
        }

        public var diagnosticsDescription: String {
            guard let summary = diagnosticsSummary else { return "No checks run yet." }
            switch summary.status {
            case .reachable:
                return "Reachable in \(String(format: "%.0f", summary.latencyMs)) ms"
            case let .unreachable(reason):
                return "Unavailable: \(reason)"
            }
        }
    }

    public enum Action: Equatable {
        case backendURLTextChanged(String)
        case task
        case loadResponse(AppSettings)
        case metadataLoaded(AppMetadata)
        case presetChanged(BackendEnvironmentPreset)
        case saveButtonTapped
        case resetButtonTapped
        case settingsSaved(AppSettings)
        case runConnectionTest
        case connectionResponseSuccess(DiagnosticsSummary)
        case connectionResponseFailed(String)
        case alert(PresentationAction<Alert>)
    }

    public enum Alert: Equatable {
        case dismiss
    }

    @Dependency(\.appSettings) var appSettings
    @Dependency(\.diagnostics) var diagnostics
    @Dependency(\.appMetadata) var appMetadata

    public init() {}

    public var body: some ReducerOf<Self> {
        Reduce { state, action in
            switch action {
            case let .backendURLTextChanged(text):
                state.backendURLText = text
                if state.canEditBackendURL {
                    state.lastCustomURLText = text
                }
                return .none

            case .task:
                state.isLoading = true
                return .run { [appSettings = self.appSettings, appMetadata = self.appMetadata] send in
                    await send(.loadResponse(appSettings.load()))
                    await send(.metadataLoaded(await appMetadata.load()))
                }

            case let .loadResponse(settings):
                state.isLoading = false
                state.selectedPreset = settings.selectedPreset
                state.savedBackendURL = settings.backendURL
                state.backendURLText = settings.backendURL.absoluteString
                state.lastCustomURLText = settings.lastCustomURL?.absoluteString ?? settings.backendURL.absoluteString
                return .none

            case let .metadataLoaded(metadata):
                state.metadata = metadata
                return .none

            case let .presetChanged(preset):
                state.selectedPreset = preset
                switch preset {
                case .custom:
                    state.backendURLText = state.lastCustomURLText
                default:
                    state.backendURLText = preset.defaultURL.absoluteString
                }
                return .none

            case .saveButtonTapped:
                guard let targetURL = state.resolveURLForCurrentPreset(), targetURL.scheme != nil else {
                    state.alert = SettingsFeature.invalidURLAlert()
                    return .none
                }
                state.isPersisting = true
                let preset = state.selectedPreset
                let customURL = preset == .custom ? targetURL : URL(string: state.lastCustomURLText)
                return .run { [appSettings = self.appSettings] send in
                    var settings = AppSettings(
                        backendURL: targetURL,
                        selectedPreset: preset,
                        lastCustomURL: customURL
                    )
                    if preset == .custom {
                        settings.lastCustomURL = targetURL
                    }
                    appSettings.save(settings)
                    await send(.settingsSaved(settings))
                }

            case .resetButtonTapped:
                state.isPersisting = true
                return .run { [appSettings = self.appSettings] send in
                    await send(.settingsSaved(appSettings.reset()))
                }

            case let .settingsSaved(settings):
                state.isPersisting = false
                state.selectedPreset = settings.selectedPreset
                state.savedBackendURL = settings.backendURL
                state.backendURLText = settings.backendURL.absoluteString
                if settings.selectedPreset == .custom {
                    state.lastCustomURLText = settings.backendURL.absoluteString
                } else {
                    state.lastCustomURLText = settings.lastCustomURL?.absoluteString ?? state.lastCustomURLText
                }
                return .none

            case .runConnectionTest:
                guard let targetURL = state.resolveURLForCurrentPreset(), targetURL.scheme != nil else {
                    state.alert = SettingsFeature.invalidURLAlert()
                    return .none
                }
                state.isDiagnosticsInFlight = true
                state.lastPingedURL = targetURL
                return .run { [diagnostics = self.diagnostics, targetURL] send in
                    do {
                        let summary = try await diagnostics.ping(targetURL)
                        await send(.connectionResponseSuccess(summary))
                    } catch {
                        await send(.connectionResponseFailed(error.localizedDescription))
                    }
                }

            case let .connectionResponseSuccess(summary):
                state.isDiagnosticsInFlight = false
                state.diagnosticsSummary = summary
                return .none

            case let .connectionResponseFailed(message):
                state.isDiagnosticsInFlight = false
                guard let url = state.lastPingedURL else { return .none }
                state.diagnosticsSummary = DiagnosticsSummary(
                    testedURL: url,
                    status: .unreachable(reason: message),
                    latencyMs: 0,
                    measuredAt: Date()
                )
                return .none

            case .alert(.dismiss), .alert(.presented(.dismiss)):
                state.alert = nil
                return .none
            }
        }
    }
}

private extension SettingsFeature.State {
    func resolveURLForCurrentPreset() -> URL? {
        let trimmed = backendURLText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return nil }
        if selectedPreset == .custom {
            return URL(string: trimmed)
        } else {
            return selectedPreset.defaultURL
        }
    }
}

private extension SettingsFeature {
    static func invalidURLAlert() -> AlertState<Alert> {
        AlertState {
            TextState("Invalid URL")
        } actions: {
            ButtonState(action: .send(.dismiss)) {
                TextState("OK")
            }
        } message: {
            TextState("Provide a full URL including scheme, e.g. https://api.musicroom.app")
        }
    }
}

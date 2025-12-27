import AppSettingsClient
import AppSupportClients
import ComposableArchitecture
import Foundation

@Reducer
public struct SettingsFeature {
    public struct State: Equatable {
        public var backendURLText: String
        public var savedBackendURL: URL?
        public var selectedPreset: BackendEnvironmentPreset
        public var lastLocalURLText: String
        public var lastHostedURLText: String
        public var lastPingedURL: URL?
        public var diagnosticsSummary: DiagnosticsSummary?
        public var metadata: AppMetadata?
        public var isLoading: Bool
        public var isPersisting: Bool
        public var isDiagnosticsInFlight: Bool
        @PresentationState public var alert: AlertState<Alert>?

        public init(
            backendURLText: String = BackendEnvironmentPreset.local.defaultURL.absoluteString,
            savedBackendURL: URL? = nil,
            selectedPreset: BackendEnvironmentPreset = .local,
            lastLocalURLText: String = BackendEnvironmentPreset.local.defaultURL.absoluteString,
            lastHostedURLText: String = BackendEnvironmentPreset.hosted.defaultURL.absoluteString,
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
            self.lastLocalURLText = lastLocalURLText
            self.lastHostedURLText = lastHostedURLText
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

        public var canEditBackendURL: Bool { true }

        public var environmentNote: String { selectedPreset.note }

        public var versionSummary: String {
            metadata?.summary ?? "Collecting app info…"
        }

        public var diagnosticsDescription: String {
            guard let summary = diagnosticsSummary else { return "No checks run yet." }
            let apiStatus =
                summary.status == .reachable
                ? "API: OK (\(Int(summary.latencyMs))ms)" : "API: Failed"
            let wsStatus =
                summary.wsStatus == .reachable
                ? "WS: OK (\(Int(summary.wsLatencyMs))ms)" : "WS: Failed"
            return "\(apiStatus)\n\(wsStatus)"
        }

        public var apiStatusText: String {
            guard let summary = diagnosticsSummary else { return "Not run" }
            switch summary.status {
            case .reachable: return "Reachable (\(Int(summary.latencyMs))ms)"
            case .unreachable(let reason): return "Error: \(reason)"
            }
        }

        public var apiStatusColor: String {  // Returning semantic color name for View to interpret or just logic
            guard let summary = diagnosticsSummary else { return "secondary" }
            return summary.status == .reachable ? "green" : "red"
        }

        public var wsStatusText: String {
            guard let summary = diagnosticsSummary else { return "Not run" }
            switch summary.wsStatus {
            case .reachable: return "Reachable (\(Int(summary.wsLatencyMs))ms)"
            case .unreachable(let reason): return "Error: \(reason)"
            }
        }

        public var wsStatusColor: String {
            guard let summary = diagnosticsSummary else { return "secondary" }
            return summary.wsStatus == .reachable ? "green" : "red"
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
    @Dependency(\.date) var date

    public init() {}

    public var body: some ReducerOf<Self> {
        Reduce { state, action in
            switch action {
            case .backendURLTextChanged(let text):
                state.backendURLText = text
                switch state.selectedPreset {
                case .local:
                    state.lastLocalURLText = text
                case .hosted:
                    state.lastHostedURLText = text
                }
                return .none

            case .task:
                state.isLoading = true
                return .run {
                    [appSettings = self.appSettings, appMetadata = self.appMetadata] send in
                    await send(.loadResponse(appSettings.load()))
                    await send(.metadataLoaded(await appMetadata.load()))
                }

            case .loadResponse(let settings):
                state.isLoading = false
                state.selectedPreset = settings.selectedPreset
                state.savedBackendURL = settings.backendURL
                state.backendURLText = settings.backendURL.absoluteString
                state.lastLocalURLText = settings.localURL.absoluteString
                state.lastHostedURLText = settings.hostedURL.absoluteString
                return .none

            case .metadataLoaded(let metadata):
                state.metadata = metadata
                return .none

            case .presetChanged(let preset):
                state.selectedPreset = preset
                switch preset {
                case .local:
                    state.backendURLText = state.lastLocalURLText
                case .hosted:
                    state.backendURLText = state.lastHostedURLText
                }
                return .none

            case .saveButtonTapped:
                guard let targetURL = state.resolveURLForCurrentPreset(), targetURL.scheme != nil
                else {
                    state.alert = SettingsFeature.invalidURLAlert()
                    return .none
                }
                state.isPersisting = true
                let preset = state.selectedPreset
                return .run { [appSettings = self.appSettings] send in
                    var settings = appSettings.load()
                    settings.selectedPreset = preset
                    settings.setURL(targetURL, for: preset)
                    appSettings.save(settings)
                    await send(.settingsSaved(settings))
                }

            case .resetButtonTapped:
                state.isPersisting = true
                return .run { [appSettings = self.appSettings] send in
                    await send(.settingsSaved(appSettings.reset()))
                }

            case .settingsSaved(let settings):
                state.isPersisting = false
                state.selectedPreset = settings.selectedPreset
                state.savedBackendURL = settings.backendURL
                state.backendURLText = settings.backendURL.absoluteString
                state.lastLocalURLText = settings.localURL.absoluteString
                state.lastHostedURLText = settings.hostedURL.absoluteString
                return .none

            case .runConnectionTest:
                guard let targetURL = state.resolveURLForCurrentPreset(), targetURL.scheme != nil
                else {
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

            case .connectionResponseSuccess(let summary):
                state.isDiagnosticsInFlight = false
                state.diagnosticsSummary = summary
                return .none

            case .connectionResponseFailed(let message):
                state.isDiagnosticsInFlight = false
                guard let url = state.lastPingedURL else { return .none }
                state.diagnosticsSummary = DiagnosticsSummary(
                    testedURL: url,
                    status: .unreachable(reason: message),
                    latencyMs: 0,
                    wsStatus: .unreachable(reason: message),
                    wsLatencyMs: 0,
                    measuredAt: date()
                )
                return .none

            case .alert(.dismiss), .alert(.presented(.dismiss)):
                state.alert = nil
                return .none
            }
        }
    }
}

extension SettingsFeature.State {
    fileprivate func resolveURLForCurrentPreset() -> URL? {
        let trimmed = backendURLText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return nil }
        return URL(string: trimmed)
    }
}

extension SettingsFeature {
    fileprivate static func invalidURLAlert() -> AlertState<Alert> {
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

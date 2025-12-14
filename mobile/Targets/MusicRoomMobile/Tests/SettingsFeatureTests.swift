import ComposableArchitecture
import XCTest

@testable import AppSettingsClient
@testable import AppSupportClients
@testable import SettingsFeature

@MainActor
final class SettingsFeatureTests: XCTestCase {
    func testLoadsPersistedURLOnTask() async {
        let storedURL = URL(string: "https://staging.musicroom.app")!
        let metadata = AppMetadata(
            version: "1.0", build: "10", deviceModel: "iPhone", systemVersion: "18.1")

        let store = TestStore(initialState: SettingsFeature.State()) {
            SettingsFeature()
        } withDependencies: {
            $0.appSettings.load = {
                AppSettings(
                    backendURL: storedURL,
                    selectedPreset: .staging,
                    lastCustomURL: URL(string: "https://custom.musicroom.dev")
                )
            }
            $0.appMetadata.load = { metadata }
        }

        await store.send(.task) {
            $0.isLoading = true
        }

        await store.receive(
            .loadResponse(
                AppSettings(
                    backendURL: storedURL, selectedPreset: .staging,
                    lastCustomURL: URL(string: "https://custom.musicroom.dev")))
        ) {
            $0.isLoading = false
            $0.selectedPreset = .staging
            $0.backendURLText = storedURL.absoluteString
            $0.savedBackendURL = storedURL
            $0.lastCustomURLText = "https://custom.musicroom.dev"
        }

        await store.receive(.metadataLoaded(metadata)) {
            $0.metadata = metadata
        }
    }

    func testInvalidURLShowsAlert() async {
        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: "invalid", selectedPreset: .custom, lastCustomURLText: "invalid")
        ) {
            SettingsFeature()
        }

        await store.send(.saveButtonTapped) {
            $0.alert = AlertState {
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

    func testSavePersistsCustomURL() async {
        let newURL = URL(string: "https://prod.musicroom.app")!
        let savedURL = LockIsolated<URL?>(nil)

        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: newURL.absoluteString, selectedPreset: .custom,
                lastCustomURLText: newURL.absoluteString)
        ) {
            SettingsFeature()
        } withDependencies: {
            $0.appSettings.save = { settings in
                savedURL.setValue(settings.backendURL)
            }
        }

        await store.send(.saveButtonTapped) {
            $0.isPersisting = true
        }

        await store.receive(
            .settingsSaved(
                AppSettings(backendURL: newURL, selectedPreset: .custom, lastCustomURL: newURL))
        ) {
            $0.isPersisting = false
            $0.selectedPreset = .custom
            $0.savedBackendURL = newURL
            $0.backendURLText = newURL.absoluteString
            $0.lastCustomURLText = newURL.absoluteString
        }

        XCTAssertEqual(savedURL.value, newURL)
    }

    func testRunConnectionTestSuccess() async {
        let targetURL = URL(string: "https://api.musicroom.app")!
        let summary = DiagnosticsSummary(
            testedURL: targetURL,
            status: .reachable,
            latencyMs: 42,
            measuredAt: Date()
        )

        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: targetURL.absoluteString, selectedPreset: .custom,
                lastCustomURLText: targetURL.absoluteString)
        ) {
            SettingsFeature()
        } withDependencies: {
            $0.diagnostics.ping = { _ in summary }
        }

        await store.send(.runConnectionTest) {
            $0.isDiagnosticsInFlight = true
            $0.lastPingedURL = targetURL
        }

        await store.receive(.connectionResponseSuccess(summary)) {
            $0.isDiagnosticsInFlight = false
            $0.diagnosticsSummary = summary
        }
    }

    func testPresetChangeUpdatesText() async {
        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: "https://custom", selectedPreset: .custom,
                lastCustomURLText: "https://custom")
        ) {
            SettingsFeature()
        }

        await store.send(.presetChanged(.staging)) {
            $0.selectedPreset = .staging
            $0.backendURLText = BackendEnvironmentPreset.staging.defaultURL.absoluteString
        }

        await store.send(.presetChanged(.custom)) {
            $0.selectedPreset = .custom
            $0.backendURLText = "https://custom"
        }
    }
    func testBackendURLTextChanged() async {
        let store = TestStore(
            initialState: SettingsFeature.State(backendURLText: "", selectedPreset: .custom)
        ) {
            SettingsFeature()
        }

        await store.send(.backendURLTextChanged("https://new-url.com")) {
            $0.backendURLText = "https://new-url.com"
            $0.lastCustomURLText = "https://new-url.com"
        }
    }

    func testBackendURLTextChanged_WhenPresetNotCustom_DoesNotUpdateLastCustomURL() async {
        let store = TestStore(
            initialState: SettingsFeature.State(backendURLText: "", selectedPreset: .staging)
        ) {
            SettingsFeature()
        }

        await store.send(.backendURLTextChanged("https://new-url.com")) {
            $0.backendURLText = "https://new-url.com"
            // lastCustomURLText should NOT change
        }
    }

    func testResetButtonTapped() async {
        let store = TestStore(
            initialState: SettingsFeature.State(backendURLText: "custom", selectedPreset: .custom)
        ) {
            SettingsFeature()
        } withDependencies: {
            $0.appSettings.reset = { AppSettings.default }
        }

        await store.send(.resetButtonTapped) {
            $0.isPersisting = true
        }

        await store.receive(.settingsSaved(AppSettings.default)) {
            $0.isPersisting = false
            $0.selectedPreset = .local
            $0.savedBackendURL = AppSettings.default.backendURL
            $0.backendURLText = AppSettings.default.backendURL.absoluteString
            $0.lastCustomURLText = AppSettings.default.backendURL.absoluteString
        }
    }

    private struct TestError: LocalizedError {
        let errorDescription: String?

        init(description: String) {
            self.errorDescription = description
        }
    }

    func testConnectionResponseFailed() async {
        let targetURL = URL(string: "https://api.musicroom.app")!
        let now = Date(timeIntervalSince1970: 1_735_689_600)  // 2025-01-01 00:00:00 UTC

        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: targetURL.absoluteString,
                selectedPreset: .custom,
                isDiagnosticsInFlight: true
            )
        ) {
            SettingsFeature()
        } withDependencies: {
            $0.date = .constant(now)
            $0.diagnostics.ping = { _ in throw TestError(description: "Network error") }
        }

        await store.send(SettingsFeature.Action.connectionResponseFailed("Network error")) {
            $0.isDiagnosticsInFlight = false
            // Should not update summary if lastPingedURL is nil
        }

        // Now test with lastPingedURL set (simulating a request was made)
        await store.send(SettingsFeature.Action.runConnectionTest) {
            $0.isDiagnosticsInFlight = true
            $0.lastPingedURL = targetURL
        }

        await store.receive(SettingsFeature.Action.connectionResponseFailed("Network error")) {
            $0.isDiagnosticsInFlight = false
            $0.diagnosticsSummary = DiagnosticsSummary(
                testedURL: targetURL,
                status: .unreachable(reason: "Network error"),
                latencyMs: 0,
                measuredAt: now
            )
        }
    }

    func testSaveStandardPreset_PreservesLastCustomURL() async {
        let customURL = URL(string: "https://my-custom.com")!
        let standardURL = BackendEnvironmentPreset.staging.defaultURL
        let savedSettings = LockIsolated<AppSettings?>(nil)

        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: standardURL.absoluteString,
                selectedPreset: .staging,
                lastCustomURLText: customURL.absoluteString
            )
        ) {
            SettingsFeature()
        } withDependencies: {
            $0.appSettings.save = { settings in savedSettings.setValue(settings) }
        }

        await store.send(.saveButtonTapped) {
            $0.isPersisting = true
        }

        await store.receive(
            .settingsSaved(
                AppSettings(
                    backendURL: standardURL, selectedPreset: .staging, lastCustomURL: customURL))
        ) {
            $0.isPersisting = false
            $0.selectedPreset = .staging
            $0.savedBackendURL = standardURL
            $0.backendURLText = standardURL.absoluteString
            // lastCustomURLText remains unchanged
        }
    }
}

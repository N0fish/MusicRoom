import ComposableArchitecture
import XCTest

@testable import AppSettingsClient
@testable import AppSupportClients
@testable import SettingsFeature

@MainActor
final class SettingsFeatureTests: XCTestCase {
    func testLoadsPersistedURLOnTask() async {
        let storedURL = URL(string: "https://staging.musicroom.app")!
        let localURL = BackendEnvironmentPreset.local.defaultURL
        let metadata = AppMetadata(
            version: "1.0", build: "10", deviceModel: "iPhone", systemVersion: "18.1")

        let store = TestStore(initialState: SettingsFeature.State()) {
            SettingsFeature()
        } withDependencies: {
            $0.appSettings.load = {
                AppSettings(
                    selectedPreset: .hosted,
                    localURL: localURL,
                    hostedURL: storedURL
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
                    selectedPreset: .hosted,
                    localURL: localURL,
                    hostedURL: storedURL))
        ) {
            $0.isLoading = false
            $0.selectedPreset = .hosted
            $0.backendURLText = storedURL.absoluteString
            $0.savedBackendURL = storedURL
            $0.lastLocalURLText = localURL.absoluteString
            $0.lastHostedURLText = storedURL.absoluteString
        }

        await store.receive(.metadataLoaded(metadata)) {
            $0.metadata = metadata
        }
    }

    func testInvalidURLShowsAlert() async {
        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: "invalid",
                selectedPreset: .hosted,
                lastLocalURLText: BackendEnvironmentPreset.local.defaultURL.absoluteString,
                lastHostedURLText: "invalid")
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

    func testSavePersistsHostedURL() async {
        let newURL = URL(string: "https://prod.musicroom.app")!
        let localURL = BackendEnvironmentPreset.local.defaultURL
        let savedSettings = LockIsolated<AppSettings?>(nil)

        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: newURL.absoluteString,
                selectedPreset: .hosted,
                lastLocalURLText: localURL.absoluteString,
                lastHostedURLText: newURL.absoluteString)
        ) {
            SettingsFeature()
        } withDependencies: {
            $0.appSettings.load = {
                AppSettings(
                    selectedPreset: .hosted,
                    localURL: localURL,
                    hostedURL: BackendEnvironmentPreset.hosted.defaultURL
                )
            }
            $0.appSettings.save = { settings in savedSettings.setValue(settings) }
        }

        await store.send(.saveButtonTapped) {
            $0.isPersisting = true
        }

        await store.receive(
            .settingsSaved(
                AppSettings(selectedPreset: .hosted, localURL: localURL, hostedURL: newURL))
        ) {
            $0.isPersisting = false
            $0.selectedPreset = .hosted
            $0.savedBackendURL = newURL
            $0.backendURLText = newURL.absoluteString
            $0.lastLocalURLText = localURL.absoluteString
            $0.lastHostedURLText = newURL.absoluteString
        }

        XCTAssertEqual(savedSettings.value?.hostedURL, newURL)
        XCTAssertEqual(savedSettings.value?.localURL, localURL)
        XCTAssertEqual(savedSettings.value?.selectedPreset, .hosted)
    }

    func testSettingsSavedLogsOutWhenBackendChanges() async {
        let originalURL = URL(string: "https://old.musicroom.app")!
        let newURL = URL(string: "https://new.musicroom.app")!
        let logoutCalled = LockIsolated(false)

        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: originalURL.absoluteString,
                savedBackendURL: originalURL,
                selectedPreset: .hosted,
                lastLocalURLText: BackendEnvironmentPreset.local.defaultURL.absoluteString,
                lastHostedURLText: originalURL.absoluteString)
        ) {
            SettingsFeature()
        } withDependencies: {
            $0.authentication.logout = { logoutCalled.setValue(true) }
        }

        await store.send(
            .settingsSaved(
                AppSettings(
                    selectedPreset: .hosted,
                    localURL: BackendEnvironmentPreset.local.defaultURL,
                    hostedURL: newURL))
        ) {
            $0.isPersisting = false
            $0.selectedPreset = .hosted
            $0.savedBackendURL = newURL
            $0.backendURLText = newURL.absoluteString
            $0.lastLocalURLText = BackendEnvironmentPreset.local.defaultURL.absoluteString
            $0.lastHostedURLText = newURL.absoluteString
        }

        await store.finish()

        XCTAssertTrue(logoutCalled.value)
    }

    func testSettingsSavedDoesNotLogoutWhenBackendUnchanged() async {
        let originalURL = URL(string: "https://same.musicroom.app")!
        let logoutCalled = LockIsolated(false)

        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: originalURL.absoluteString,
                savedBackendURL: originalURL,
                selectedPreset: .hosted,
                lastLocalURLText: BackendEnvironmentPreset.local.defaultURL.absoluteString,
                lastHostedURLText: originalURL.absoluteString)
        ) {
            SettingsFeature()
        } withDependencies: {
            $0.authentication.logout = { logoutCalled.setValue(true) }
        }

        await store.send(
            .settingsSaved(
                AppSettings(
                    selectedPreset: .hosted,
                    localURL: BackendEnvironmentPreset.local.defaultURL,
                    hostedURL: originalURL))
        )

        await store.finish()

        XCTAssertFalse(logoutCalled.value)
    }

    func testRunConnectionTestSuccess() async {
        let targetURL = URL(string: "https://api.musicroom.app")!
        let summary = DiagnosticsSummary(
            testedURL: targetURL,
            status: .reachable,
            latencyMs: 42,
            wsStatus: .reachable,
            wsLatencyMs: 35,
            measuredAt: Date()
        )

        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: targetURL.absoluteString,
                selectedPreset: .hosted,
                lastLocalURLText: BackendEnvironmentPreset.local.defaultURL.absoluteString,
                lastHostedURLText: targetURL.absoluteString)
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
                backendURLText: "https://hosted",
                selectedPreset: .hosted,
                lastLocalURLText: BackendEnvironmentPreset.local.defaultURL.absoluteString,
                lastHostedURLText: "https://hosted")
        ) {
            SettingsFeature()
        }

        await store.send(.presetChanged(.local)) {
            $0.selectedPreset = .local
            $0.backendURLText = BackendEnvironmentPreset.local.defaultURL.absoluteString
        }

        await store.send(.presetChanged(.hosted)) {
            $0.selectedPreset = .hosted
            $0.backendURLText = "https://hosted"
        }
    }
    func testBackendURLTextChanged() async {
        let store = TestStore(
            initialState: SettingsFeature.State(backendURLText: "", selectedPreset: .hosted)
        ) {
            SettingsFeature()
        }

        await store.send(.backendURLTextChanged("https://new-url.com")) {
            $0.backendURLText = "https://new-url.com"
            $0.lastHostedURLText = "https://new-url.com"
        }
    }

    func testBackendURLTextChanged_WhenPresetLocal_UpdatesLocalOnly() async {
        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: "",
                selectedPreset: .local,
                lastLocalURLText: BackendEnvironmentPreset.local.defaultURL.absoluteString,
                lastHostedURLText: "https://hosted"
            )
        ) {
            SettingsFeature()
        }

        await store.send(.backendURLTextChanged("https://new-url.com")) {
            $0.backendURLText = "https://new-url.com"
            $0.lastLocalURLText = "https://new-url.com"
            // lastHostedURLText should NOT change
        }
    }

    func testResetButtonTapped() async {
        let store = TestStore(
            initialState: SettingsFeature.State(backendURLText: "hosted", selectedPreset: .hosted)
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
            $0.lastLocalURLText = AppSettings.default.localURL.absoluteString
            $0.lastHostedURLText = AppSettings.default.hostedURL.absoluteString
        }
    }

    func testSaveLocalPreset_PersistsLocalURL() async {
        let localURL = URL(string: "http://192.168.0.42:8080")!
        let savedSettings = LockIsolated<AppSettings?>(nil)

        let store = TestStore(
            initialState: SettingsFeature.State(
                backendURLText: localURL.absoluteString,
                selectedPreset: .local,
                lastLocalURLText: localURL.absoluteString,
                lastHostedURLText: BackendEnvironmentPreset.hosted.defaultURL.absoluteString
            )
        ) {
            SettingsFeature()
        } withDependencies: {
            $0.appSettings.load = {
                AppSettings(
                    selectedPreset: .local,
                    localURL: BackendEnvironmentPreset.local.defaultURL,
                    hostedURL: BackendEnvironmentPreset.hosted.defaultURL
                )
            }
            $0.appSettings.save = { settings in savedSettings.setValue(settings) }
        }

        await store.send(.saveButtonTapped) {
            $0.isPersisting = true
        }

        await store.receive(
            .settingsSaved(
                AppSettings(
                    selectedPreset: .local,
                    localURL: localURL,
                    hostedURL: BackendEnvironmentPreset.hosted.defaultURL))
        ) {
            $0.isPersisting = false
            $0.selectedPreset = .local
            $0.savedBackendURL = localURL
            $0.backendURLText = localURL.absoluteString
            $0.lastLocalURLText = localURL.absoluteString
            $0.lastHostedURLText = BackendEnvironmentPreset.hosted.defaultURL.absoluteString
        }

        XCTAssertEqual(savedSettings.value?.localURL, localURL)
        XCTAssertEqual(
            savedSettings.value?.hostedURL,
            BackendEnvironmentPreset.hosted.defaultURL
        )
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
                selectedPreset: .hosted,
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
                wsStatus: .unreachable(reason: "Network error"),
                wsLatencyMs: 0,
                measuredAt: now
            )
        }
    }
}

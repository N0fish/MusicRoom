import XCTest

@testable import AppSettingsClient

final class AppSettingsClientOverrideTests: XCTestCase {
    func testBackendURLStringStripsTrailingSlashes() {
        let hostedURL = URL(string: "https://api.musicroom.app///")!
        let settings = AppSettings(
            selectedPreset: .hosted,
            localURL: BackendEnvironmentPreset.local.defaultURL,
            hostedURL: hostedURL
        )

        XCTAssertEqual(settings.backendURLString, "https://api.musicroom.app")
    }

    func testSetURLUpdatesOnlyTargetPreset() {
        var settings = AppSettings(
            selectedPreset: .local,
            localURL: BackendEnvironmentPreset.local.defaultURL,
            hostedURL: BackendEnvironmentPreset.hosted.defaultURL
        )

        let newHosted = URL(string: "https://staging.musicroom.app")!
        settings.setURL(newHosted, for: .hosted)

        XCTAssertEqual(settings.localURL, BackendEnvironmentPreset.local.defaultURL)
        XCTAssertEqual(settings.hostedURL, newHosted)
    }
}

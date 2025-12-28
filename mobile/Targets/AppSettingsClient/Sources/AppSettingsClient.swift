import Dependencies
import Foundation

public struct AppSettings: Codable, Equatable, Sendable {
    public var selectedPreset: BackendEnvironmentPreset
    public var localURL: URL
    public var hostedURL: URL

    public init(
        selectedPreset: BackendEnvironmentPreset = .local,
        localURL: URL = BackendEnvironmentPreset.local.defaultURL,
        hostedURL: URL = BackendEnvironmentPreset.hosted.defaultURL
    ) {
        self.selectedPreset = selectedPreset
        self.localURL = localURL
        self.hostedURL = hostedURL
    }

    public var backendURL: URL {
        get { selectedPreset == .local ? localURL : hostedURL }
        set {
            switch selectedPreset {
            case .local:
                localURL = newValue
            case .hosted:
                hostedURL = newValue
            }
        }
    }

    public func url(for preset: BackendEnvironmentPreset) -> URL {
        preset == .local ? localURL : hostedURL
    }

    public mutating func setURL(_ url: URL, for preset: BackendEnvironmentPreset) {
        switch preset {
        case .local:
            localURL = url
        case .hosted:
            hostedURL = url
        }
    }

    public var canEditBackendURL: Bool { true }

    public var backendURLSummary: String { backendURL.absoluteString }

    public var backendURLString: String {
        var string = backendURL.absoluteString
        while string.hasSuffix("/") {
            string.removeLast()
        }
        return string
    }
}

public enum BackendEnvironmentPreset: String, CaseIterable, Codable, Sendable {
    case local
    case hosted

    public var title: String {
        switch self {
        case .local: return "Local"
        case .hosted: return "Hosted"
        }
    }

    public var note: String {
        switch self {
        case .local:
            return "Local dev server (localhost or LAN IP)."
        case .hosted:
            return "Hosted Musicroom server (production or staging)."
        }
    }

    public var defaultURL: URL {
        switch self {
        case .local:
            return URL(string: "http://localhost:8080")!
        case .hosted:
            return URL(string: "https://api.musicroom.app")!
        }
    }
}

public struct AppSettingsClient: Sendable {
    public var load: @Sendable () -> AppSettings
    public var save: @Sendable (AppSettings) -> Void
    public var reset: @Sendable () -> AppSettings

    public init(
        load: @escaping @Sendable () -> AppSettings,
        save: @escaping @Sendable (AppSettings) -> Void,
        reset: @escaping @Sendable () -> AppSettings
    ) {
        self.load = load
        self.save = save
        self.reset = reset
    }
}

extension AppSettingsClient {
    public static let testValue = AppSettingsClient(
        load: { .default },
        save: { _ in },
        reset: { .default }
    )
    public static let previewValue = testValue
}

extension AppSettings {
    public static let `default` = AppSettings(
        selectedPreset: .local,
        localURL: BackendEnvironmentPreset.local.defaultURL,
        hostedURL: BackendEnvironmentPreset.hosted.defaultURL
    )
}

extension DependencyValues {
    public var appSettings: AppSettingsClient {
        get { self[AppSettingsClientKey.self] }
        set { self[AppSettingsClientKey.self] = newValue }
    }
}

private enum AppSettingsStorageKey {
    static let backendURL = "musicroom.settings.backend-url"
    static let preset = "musicroom.settings.backend-preset"
    static let customBackendURL = "musicroom.settings.custom-backend-url"
    static let localBackendURL = "musicroom.settings.backend-url-local"
    static let hostedBackendURL = "musicroom.settings.backend-url-hosted"
}

private enum AppSettingsClientKey: DependencyKey, TestDependencyKey {
    static let liveValue = AppSettingsClient(
        load: {
            let defaults = UserDefaults.standard
            let presetRaw = defaults.string(forKey: AppSettingsStorageKey.preset)
            let preset = BackendEnvironmentPreset(rawValue: presetRaw ?? "") ?? .local

            let legacyBackendURL = defaults.string(forKey: AppSettingsStorageKey.backendURL)
                .flatMap(URL.init(string:))
            let legacyCustomURL = defaults.string(forKey: AppSettingsStorageKey.customBackendURL)
                .flatMap(URL.init(string:))

            var localURL = defaults.string(forKey: AppSettingsStorageKey.localBackendURL)
                .flatMap(URL.init(string:))
            var hostedURL = defaults.string(forKey: AppSettingsStorageKey.hostedBackendURL)
                .flatMap(URL.init(string:))

            if localURL == nil {
                if preset == .local, let legacyBackendURL {
                    localURL = legacyBackendURL
                } else {
                    localURL = BackendEnvironmentPreset.local.defaultURL
                }
            }

            if hostedURL == nil {
                if let legacyCustomURL {
                    hostedURL = legacyCustomURL
                } else if preset == .hosted, let legacyBackendURL {
                    hostedURL = legacyBackendURL
                } else {
                    hostedURL = BackendEnvironmentPreset.hosted.defaultURL
                }
            }

            return AppSettings(
                selectedPreset: preset,
                localURL: localURL ?? BackendEnvironmentPreset.local.defaultURL,
                hostedURL: hostedURL ?? BackendEnvironmentPreset.hosted.defaultURL
            )
        },
        save: { settings in
            let defaults = UserDefaults.standard
            defaults.set(settings.backendURL.absoluteString, forKey: AppSettingsStorageKey.backendURL)
            defaults.set(
                settings.selectedPreset.rawValue,
                forKey: AppSettingsStorageKey.preset
            )
            defaults.set(
                settings.localURL.absoluteString,
                forKey: AppSettingsStorageKey.localBackendURL
            )
            defaults.set(
                settings.hostedURL.absoluteString,
                forKey: AppSettingsStorageKey.hostedBackendURL
            )
            defaults.set(
                settings.hostedURL.absoluteString,
                forKey: AppSettingsStorageKey.customBackendURL
            )
        },
        reset: {
            let defaults = UserDefaults.standard
            defaults.removeObject(forKey: AppSettingsStorageKey.backendURL)
            defaults.removeObject(forKey: AppSettingsStorageKey.preset)
            defaults.removeObject(forKey: AppSettingsStorageKey.customBackendURL)
            defaults.removeObject(forKey: AppSettingsStorageKey.localBackendURL)
            defaults.removeObject(forKey: AppSettingsStorageKey.hostedBackendURL)
            return .default
        }
    )

    static let previewValue = AppSettingsClient(
        load: { .default },
        save: { _ in },
        reset: { .default }
    )

    static let testValue = AppSettingsClient(
        load: { .default },
        save: { _ in },
        reset: { .default }
    )
}

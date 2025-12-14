import Dependencies
import Foundation

public struct AppSettings: Codable, Equatable, Sendable {
    public var backendURL: URL
    public var selectedPreset: BackendEnvironmentPreset
    public var lastCustomURL: URL?

    public init(
        backendURL: URL,
        selectedPreset: BackendEnvironmentPreset = .custom,
        lastCustomURL: URL? = nil
    ) {
        self.backendURL = backendURL
        self.selectedPreset = selectedPreset
        self.lastCustomURL = lastCustomURL
    }

    public var canEditBackendURL: Bool { selectedPreset == .custom }

    public var backendURLSummary: String { backendURL.absoluteString }
}

public enum BackendEnvironmentPreset: String, CaseIterable, Codable, Sendable {
    case local
    case staging
    case production
    case custom

    public var title: String {
        switch self {
        case .local: return "Local"
        case .staging: return "Staging"
        case .production: return "Production"
        case .custom: return "Custom"
        }
    }

    public var note: String {
        switch self {
        case .local:
            return "Uses localhost with default dev port."
        case .staging:
            return "Points to the shared staging cluster."
        case .production:
            return "Routes to api.musicroom.app."
        case .custom:
            return "Provide any reachable URL manually."
        }
    }

    public var defaultURL: URL {
        switch self {
        case .local:
            return URL(string: "http://localhost:8080")!
        case .staging:
            return URL(string: "https://staging.api.musicroom.app")!
        case .production:
            return URL(string: "https://api.musicroom.app")!
        case .custom:
            return URL(string: "http://localhost:8080")!
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
        backendURL: BackendEnvironmentPreset.local.defaultURL,
        selectedPreset: .local,
        lastCustomURL: nil
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
}

private enum AppSettingsClientKey: DependencyKey, TestDependencyKey {
    static let liveValue = AppSettingsClient(
        load: {
            let defaults = UserDefaults.standard
            let presetRaw = defaults.string(forKey: AppSettingsStorageKey.preset)
            let preset = BackendEnvironmentPreset(rawValue: presetRaw ?? "") ?? .local

            let storedURLString = defaults.string(forKey: AppSettingsStorageKey.backendURL)
            let customURLString = defaults.string(forKey: AppSettingsStorageKey.customBackendURL)

            let fallbackURL = preset.defaultURL
            let backendURL = storedURLString.flatMap(URL.init(string:)) ?? fallbackURL
            let customURL = customURLString.flatMap(URL.init(string:))

            if preset == .custom {
                return AppSettings(
                    backendURL: customURL ?? backendURL,
                    selectedPreset: .custom,
                    lastCustomURL: customURL ?? backendURL
                )
            } else {
                return AppSettings(
                    backendURL: backendURL,
                    selectedPreset: preset,
                    lastCustomURL: customURL
                )
            }
        },
        save: { settings in
            let defaults = UserDefaults.standard
            defaults.set(
                settings.backendURL.absoluteString,
                forKey: AppSettingsStorageKey.backendURL
            )
            defaults.set(
                settings.selectedPreset.rawValue,
                forKey: AppSettingsStorageKey.preset
            )
            let customURLString: String?
            if settings.selectedPreset == .custom {
                customURLString = settings.backendURL.absoluteString
            } else {
                customURLString = settings.lastCustomURL?.absoluteString
            }
            defaults.set(customURLString, forKey: AppSettingsStorageKey.customBackendURL)
        },
        reset: {
            let defaults = UserDefaults.standard
            defaults.removeObject(forKey: AppSettingsStorageKey.backendURL)
            defaults.removeObject(forKey: AppSettingsStorageKey.preset)
            defaults.removeObject(forKey: AppSettingsStorageKey.customBackendURL)
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

import Dependencies
import Foundation

public struct AppSettings: Codable, Equatable, Sendable {
    public var backendURL: URL
    public var selectedPreset: BackendEnvironmentPreset
    public var lastCustomURL: URL?

    public init(
        backendURL: URL,
        selectedPreset: BackendEnvironmentPreset = .hosted,
        lastCustomURL: URL? = nil
    ) {
        self.backendURL = backendURL
        self.selectedPreset = selectedPreset
        self.lastCustomURL = lastCustomURL
    }

    public var canEditBackendURL: Bool { selectedPreset == .hosted }

    public var backendURLSummary: String { backendURL.absoluteString }
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
            return "Uses localhost with default dev port."
        case .hosted:
            return "Connect to an existing Musicroom server."
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

            if preset == .hosted {
                return AppSettings(
                    backendURL: customURL ?? backendURL,
                    selectedPreset: .hosted,
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
            if settings.selectedPreset == .hosted {
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

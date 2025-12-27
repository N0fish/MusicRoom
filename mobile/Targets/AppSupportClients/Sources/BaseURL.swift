import Foundation

enum BaseURL {
    static func resolve() -> String {
        let defaults = UserDefaults.standard
        let presetRaw = defaults.string(forKey: "musicroom.settings.backend-preset")
        let preset = presetRaw == "hosted" ? "hosted" : "local"

        if preset == "hosted" {
            if let customOne = defaults.string(forKey: "musicroom.settings.custom-backend-url"),
                !customOne.isEmpty
            {
                return customOne
            }
            if let backendOne = defaults.string(forKey: "musicroom.settings.backend-url"),
                !backendOne.isEmpty
            {
                return backendOne
            }
            return "https://api.musicroom.app"
        } else {
            if let localOne = defaults.string(forKey: "musicroom.settings.backend-url"),
                !localOne.isEmpty
            {
                return localOne
            }
            return "http://localhost:8080"
        }
    }
}

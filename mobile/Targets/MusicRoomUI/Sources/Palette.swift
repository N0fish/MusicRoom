import SwiftUI

extension Color {
    // MARK: - Liquid Palette (Neon/Vibrant)
    public static let liquidPrimary = Color(hex: "00F0FF")  // Cyan
    public static let liquidSecondary = Color(hex: "7000FF")  // Electric Purple
    public static let liquidAccent = Color(hex: "FF00AA")  // Hot Pink

    // MARK: - Backgrounds
    public static let liquidBlack = Color(hex: "050505")  // Deep Black
    public static let liquidDarkGray = Color(hex: "121212")  // Surface

    // MARK: - Glass
    public static let glassBorder = Color.white.opacity(0.2)
    public static let glassContent = Color.white.opacity(0.1)
}

// MARK: - Hex Helper
extension Color {
    public init(hex: String) {
        let hex = hex.trimmingCharacters(in: CharacterSet.alphanumerics.inverted)
        var int: UInt64 = 0
        Scanner(string: hex).scanHexInt64(&int)
        let a: UInt64
        let r: UInt64
        let g: UInt64
        let b: UInt64
        switch hex.count {
        case 3: (a, r, g, b) = (255, (int >> 8) * 17, (int >> 4 & 0xF) * 17, (int & 0xF) * 17)
        case 6: (a, r, g, b) = (255, int >> 16, int >> 8 & 0xFF, int & 0xFF)
        case 8: (a, r, g, b) = (int >> 24, int >> 16 & 0xFF, int >> 8 & 0xFF, int & 0xFF)
        default: (a, r, g, b) = (1, 1, 1, 0)
        }
        self.init(
            .sRGB,
            red: Double(r) / 255,
            green: Double(g) / 255,
            blue: Double(b) / 255,
            opacity: Double(a) / 255
        )
    }
}

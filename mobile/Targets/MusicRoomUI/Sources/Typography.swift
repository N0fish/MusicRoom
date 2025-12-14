import SwiftUI

extension Font {
    // MARK: - Liquid Typography

    /// Large title for headers (34pt, Bold, Rounded)
    public static let liquidTitle: Font = .system(size: 34, weight: .bold, design: .rounded)

    /// H2 for section headers (22pt, Semibold, Rounded)
    public static let liquidH2: Font = .system(size: 22, weight: .semibold, design: .rounded)

    /// Body text (17pt, Regular, Rounded)
    public static let liquidBody: Font = .system(size: 17, weight: .regular, design: .rounded)

    /// Button text (19pt, Semibold, Rounded)
    public static let liquidButton: Font = .system(size: 19, weight: .semibold, design: .rounded)

    /// Caption (13pt, Medium, Rounded)
    public static let liquidCaption: Font = .system(size: 13, weight: .medium, design: .rounded)
}

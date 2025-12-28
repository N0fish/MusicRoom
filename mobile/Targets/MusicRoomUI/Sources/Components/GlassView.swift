import SwiftUI

public struct GlassView<Content: View>: View {
    let content: Content
    let cornerRadius: CGFloat

    public init(cornerRadius: CGFloat = 20, @ViewBuilder content: () -> Content) {
        self.cornerRadius = cornerRadius
        self.content = content()
    }

    public var body: some View {
        content
            .background(Material.ultraThin)  // The "Glass"
            .cornerRadius(cornerRadius)
            .padding(1)  // Border width
            .background(
                RoundedRectangle(cornerRadius: cornerRadius + 1)
                    .fill(Color.glassBorder)
                    .opacity(0.3)
            )  // The Border
            .shadow(color: Color.black.opacity(0.2), radius: 10, x: 0, y: 5)
    }
}

extension GlassView where Content == Color {
    public init(cornerRadius: CGFloat = 20) {
        self.init(cornerRadius: cornerRadius) {
            Color.clear
        }
    }
}

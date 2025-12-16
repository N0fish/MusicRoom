import SwiftUI

public struct LiquidButton<Label: View>: View {
    let action: () -> Void
    let label: Label

    // Customization
    let useGlass: Bool

    @State private var isPressed = false

    public init(
        useGlass: Bool = true,
        action: @escaping () -> Void,
        @ViewBuilder label: () -> Label
    ) {
        self.useGlass = useGlass
        self.action = action
        self.label = label()
    }

    public var body: some View {
        Button(action: {
            // Trigger impact feedback (Removed)

            action()
        }) {
            label
                .padding()
                .background(
                    Group {
                        if useGlass {
                            GlassView<Color>(cornerRadius: 30)
                        } else {
                            Color.clear
                        }
                    }
                )
                .contentShape(Rectangle())  // Ensure tap area is good
        }
        .buttonStyle(ScaleButtonStyle())
    }
}

// Simple scale effect style
struct ScaleButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.95 : 1.0)
            .animation(.spring(response: 0.3, dampingFraction: 0.6), value: configuration.isPressed)
            .opacity(configuration.isPressed ? 0.8 : 1.0)
    }
}

// NEW APPROACH: "LiquidPrimaryButton" view that is concrete.
public struct LiquidPrimaryButton: View {
    let title: String
    let icon: String?
    let action: () -> Void

    public init(title: String, icon: String? = nil, action: @escaping () -> Void) {
        self.title = title
        self.icon = icon
        self.action = action
    }

    public var body: some View {
        LiquidButton(action: action) {
            HStack(spacing: 8) {
                if let icon = icon {
                    Image(systemName: icon)
                }
                Text(title)
            }
            .font(Font.liquidButton)
            .foregroundStyle(.white)
        }
    }
}

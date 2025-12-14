import SwiftUI

public struct LiquidBackground: View {
    @State private var start = UnitPoint(x: 0, y: -2)
    @State private var end = UnitPoint(x: 4, y: 0)

    public init() {}

    public var body: some View {
        ZStack {
            Color.liquidBlack.ignoresSafeArea()

            // "Liquid" Blobs (simulated with Gradients for now, MeshGradient in iOS 18+)
            // Since this is generic SwiftUI, we stick to Linear/Radial for max compatibility unless instructed.
            // User mentioned iOS 26+ -> Safe to assume MeshGradient exists, but sticking to standard for now to ensure compilation.

            LinearGradient(
                colors: [.liquidPrimary.opacity(0.4), .liquidSecondary.opacity(0.3), .liquidBlack],
                startPoint: start,
                endPoint: end
            )
            .ignoresSafeArea()
            .blur(radius: 60)
            .onAppear {
                withAnimation(.easeInOut(duration: 5).repeatForever(autoreverses: true)) {
                    start = UnitPoint(x: 4, y: 0)
                    end = UnitPoint(x: 0, y: 2)
                }
            }

            // Floating Orb
            Circle()
                .fill(Color.liquidAccent)
                .frame(width: 200, height: 200)
                .blur(radius: 80)
                .offset(x: -100, y: -200)
                .opacity(0.5)
        }
    }
}

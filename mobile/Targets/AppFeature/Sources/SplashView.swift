import SwiftUI

public struct SplashView: View {
    @State private var isAnimating = false

    public init() {}

    public var body: some View {
        ZStack {
            // Background Gradient
            LinearGradient(
                gradient: Gradient(colors: [
                    Color(red: 0.1, green: 0.0, blue: 0.2),  // Deep Purple/Black
                    Color(red: 0.3, green: 0.0, blue: 0.4),  // Rich Purple
                    Color(red: 0.6, green: 0.1, blue: 0.5),  // Vibrant Magenta/Pink
                ]),
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            .ignoresSafeArea()

            // Pulsing Music Icon
            ZStack {
                Circle()
                    .stroke(Color.white.opacity(0.2), lineWidth: 40)
                    .scaleEffect(isAnimating ? 1.5 : 0.8)
                    .opacity(isAnimating ? 0.0 : 0.5)

                Circle()
                    .stroke(Color.white.opacity(0.3), lineWidth: 20)
                    .scaleEffect(isAnimating ? 1.3 : 0.9)
                    .opacity(isAnimating ? 0.0 : 0.6)

                Image(systemName: "music.note")
                    .resizable()
                    .aspectRatio(contentMode: .fit)
                    .frame(width: 80, height: 80)
                    .foregroundColor(.white)
                    .shadow(color: .white.opacity(0.5), radius: 10, x: 0, y: 0)
            }
            .onAppear {
                withAnimation(
                    Animation.easeInOut(duration: 1.5)
                        .repeatForever(autoreverses: false)
                ) {
                    isAnimating = true
                }
            }
        }
    }
}

#Preview {
    SplashView()
}

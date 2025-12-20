import SwiftUI

public struct PremiumAvatarView: View {
    let url: String?
    let isPremium: Bool
    let size: CGFloat

    @State private var rotation: Double = 0

    public init(url: String?, isPremium: Bool, size: CGFloat = 120) {
        self.url = url
        self.isPremium = isPremium
        self.size = size
    }

    public var body: some View {
        ZStack {
            if isPremium {
                // Shimmering Halo
                Circle()
                    .stroke(
                        AngularGradient(
                            gradient: Gradient(colors: [
                                Color.liquidPrimary,
                                Color.liquidSecondary,
                                Color.liquidAccent,
                                Color.liquidPrimary,
                            ]),
                            center: .center
                        ),
                        lineWidth: 4
                    )
                    .frame(width: size + 6, height: size + 6)
                    .rotationEffect(.degrees(rotation))
                    .blur(radius: 1)
                    .onAppear {
                        withAnimation(.linear(duration: 4).repeatForever(autoreverses: false)) {
                            rotation = 360
                        }
                    }

                // Outer Glow for extra magic
                Circle()
                    .stroke(
                        AngularGradient(
                            gradient: Gradient(colors: [
                                Color.liquidPrimary.opacity(0.6),
                                Color.liquidSecondary.opacity(0.6),
                                Color.liquidAccent.opacity(0.6),
                                Color.liquidPrimary.opacity(0.6),
                            ]),
                            center: .center
                        ),
                        lineWidth: 2
                    )
                    .frame(width: size + 10, height: size + 10)
                    .rotationEffect(.degrees(-rotation * 0.5))  // Slow counter-rotation
                    .blur(radius: 3)
            } else {
                Circle()
                    .stroke(.white.opacity(0.2), lineWidth: 2)
                    .frame(width: size, height: size)
            }

            AsyncImage(url: URL(string: url ?? "")) { image in
                image
                    .resizable()
                    .aspectRatio(contentMode: .fill)
            } placeholder: {
                Image(systemName: "person.circle.fill")
                    .resizable()
                    .foregroundStyle(.white.opacity(0.3))
            }
            .frame(width: size, height: size)
            .clipShape(Circle())
        }
    }
}

#Preview {
    ZStack {
        Color.black.ignoresSafeArea()
        VStack(spacing: 40) {
            PremiumAvatarView(url: nil, isPremium: true, size: 100)
            PremiumAvatarView(url: nil, isPremium: false, size: 100)
        }
    }
}

import SwiftUI
import UIKit

struct ShakeDetectingView<Content: View>: UIViewControllerRepresentable {
    var onShake: () -> Void
    var content: Content

    init(onShake: @escaping () -> Void, @ViewBuilder content: () -> Content) {
        self.onShake = onShake
        self.content = content()
    }

    func makeUIViewController(context: Context) -> ShakeDetectingHostingController<Content> {
        ShakeDetectingHostingController(rootView: content, onShake: onShake)
    }

    func updateUIViewController(
        _ uiViewController: ShakeDetectingHostingController<Content>,
        context: Context
    ) {
        uiViewController.onShake = onShake
        uiViewController.rootView = content
    }
}

final class ShakeDetectingHostingController<Content: View>: UIHostingController<Content> {
    var onShake: (() -> Void)?

    init(rootView: Content, onShake: @escaping () -> Void) {
        self.onShake = onShake
        super.init(rootView: rootView)
    }

    @MainActor @objc dynamic required init?(coder aDecoder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    override var canBecomeFirstResponder: Bool {
        true
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        becomeFirstResponder()
    }

    override func motionEnded(_ motion: UIEvent.EventSubtype, with event: UIEvent?) {
        super.motionEnded(motion, with: event)
        guard motion == .motionShake else { return }
        onShake?()
    }
}

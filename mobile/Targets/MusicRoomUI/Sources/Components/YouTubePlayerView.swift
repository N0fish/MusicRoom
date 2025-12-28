import SwiftUI
import YouTubeiOSPlayerHelper

public struct YouTubePlayerView: UIViewRepresentable {
    @Binding var videoId: String?
    @Binding var isPlaying: Bool
    var onEnded: (() -> Void)?
    var startSeconds: Double

    public init(
        videoId: Binding<String?>, isPlaying: Binding<Bool> = .constant(true),
        startSeconds: Double = 0,
        onEnded: (() -> Void)? = nil
    ) {
        self._videoId = videoId
        self._isPlaying = isPlaying
        self.startSeconds = startSeconds
        self.onEnded = onEnded
    }

    public func makeUIView(context: Context) -> YTPlayerView {
        let playerView = YTPlayerView()
        playerView.delegate = context.coordinator
        return playerView
    }

    public func updateUIView(_ uiView: YTPlayerView, context: Context) {
        if let videoId = videoId {
            if videoId != context.coordinator.currentVideoId {
                print("YouTubePlayerView: Loading video \(videoId) at \(startSeconds)s")
                context.coordinator.currentVideoId = videoId
                uiView.load(
                    withVideoId: videoId,
                    playerVars: [
                        "playsinline": 1,
                        "controls": 0,
                        "showinfo": 0,
                        "modestbranding": 1,
                        "start": Int(startSeconds),
                        "vq": "small",
                    ])
            }
        } else if context.coordinator.currentVideoId != nil {
            print("YouTubePlayerView: Stopping playback (no ID)")
            uiView.stopVideo()
            context.coordinator.currentVideoId = nil
        }

        // Handle play/pause state if needed via binding
        if context.coordinator.currentVideoId != nil {
            if isPlaying {
                uiView.playVideo()
            } else {
                uiView.pauseVideo()
            }
        }
    }

    public func makeCoordinator() -> Coordinator {
        Coordinator(parent: self)
    }

    public class Coordinator: NSObject, YTPlayerViewDelegate, @unchecked Sendable {
        var parent: YouTubePlayerView
        var currentVideoId: String?

        init(parent: YouTubePlayerView) {
            self.parent = parent
        }

        public func playerViewDidBecomeReady(_ playerView: YTPlayerView) {
            print("YouTubePlayerView: Ready")
            Task { @MainActor in
                playerView.playVideo()
            }
        }

        public func playerView(_ playerView: YTPlayerView, didChangeTo state: YTPlayerState) {
            print("YouTubePlayerView: State changed to \(state.rawValue)")
            Task { @MainActor in
                switch state {
                case .playing:
                    self.parent.isPlaying = true
                case .paused:
                    self.parent.isPlaying = false
                case .ended:
                    self.parent.isPlaying = false
                    self.parent.onEnded?()
                default:
                    break
                }
            }
        }

        public func playerView(_ playerView: YTPlayerView, receivedError error: YTPlayerError) {
            print("YouTubePlayerView: Error \(error)")
        }
    }
}

import Foundation
import Dependencies
import MusicRoomDomain

public struct PlaylistStreamClient: Sendable {
    public var startPreview: @Sendable (_ event: Event) -> AsyncStream<PlaylistUpdate>

    public init(startPreview: @escaping @Sendable (_ event: Event) -> AsyncStream<PlaylistUpdate>) {
        self.startPreview = startPreview
    }
}

extension PlaylistStreamClient: DependencyKey {
    public static let liveValue = PlaylistStreamClient { event in
        AsyncStream { continuation in
            let updates = event.playlist.enumerated().map { index, track in
                PlaylistUpdate(
                    eventID: event.id,
                    updatedTrack: track,
                    message: "Track \(track.title) now at position #\(index + 1)"
                )
            }

            Task {
                for update in updates {
                    try? await Task.sleep(nanoseconds: 200_000_000)
                    continuation.yield(update)
                }
                continuation.finish()
            }
        }
    }

    public static let previewValue = PlaylistStreamClient { event in
        AsyncStream { continuation in
            let track = event.playlist.first ?? Track(title: "Preview", artist: "", votes: 0)
            continuation.yield(
                PlaylistUpdate(eventID: event.id, updatedTrack: track, message: "Preview event fired")
            )
            continuation.finish()
        }
    }

    public static let testValue = PlaylistStreamClient { _ in
        AsyncStream { continuation in
            continuation.finish()
        }
    }
}

extension DependencyValues {
    public var playlistStream: PlaylistStreamClient {
        get { self[PlaylistStreamClient.self] }
        set { self[PlaylistStreamClient.self] = newValue }
    }
}

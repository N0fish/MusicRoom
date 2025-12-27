import Dependencies
import Foundation
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
            // Synthesize a playlist for the mock since Event doesn't have one
            let synthesizedPlaylist = [
                Track(
                    title: "Song 1", artist: "Artist 1", provider: "mock", providerTrackId: "1",
                    votes: 5),
                Track(
                    title: "Song 2", artist: "Artist 2", provider: "mock", providerTrackId: "2",
                    votes: 3),
                Track(
                    title: "Song 3", artist: "Artist 3", provider: "mock", providerTrackId: "3",
                    votes: 1),
            ]

            let updates = synthesizedPlaylist.enumerated().map { index, track in
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
            let track = Track(
                title: "Preview", artist: "Unknown", provider: "mock", providerTrackId: "0",
                votes: 0)
            continuation.yield(
                PlaylistUpdate(
                    eventID: event.id, updatedTrack: track, message: "Preview event fired")
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

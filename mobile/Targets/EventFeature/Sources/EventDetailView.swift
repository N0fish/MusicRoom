@_spi(Presentation) import ComposableArchitecture
import MusicRoomAPI
import MusicRoomDomain
import MusicRoomUI
import SwiftUI

public struct EventDetailView: View {
    @Bindable var store: StoreOf<EventDetailFeature>

    public init(store: StoreOf<EventDetailFeature>) {
        self.store = store
    }

    public var body: some View {
        ZStack {
            LiquidBackground()
                .ignoresSafeArea()

            VStack(spacing: 0) {
                // Header
                VStack(spacing: 8) {
                    Text(store.event.name)
                        .font(.liquidTitle)
                        .foregroundStyle(.white)
                        .multilineTextAlignment(.center)

                    Text(store.event.visibility.label + " • " + store.event.licenseMode.label)
                        .font(.liquidCaption)
                        .foregroundStyle(.white.opacity(0.8))
                }
                .padding()
                .frame(maxWidth: .infinity)
                .background(.ultraThinMaterial)

                ScrollView {
                    VStack(alignment: .leading, spacing: 20) {

                        // Alert Overlay
                        if let alert = store.userAlert {
                            VStack(alignment: .leading, spacing: 4) {
                                HStack {
                                    Image(
                                        systemName: alert.type == .error
                                            ? "exclamationmark.triangle.fill"
                                            : (alert.type == .success
                                                ? "checkmark.circle.fill" : "info.circle.fill")
                                    )
                                    .foregroundStyle(.white)
                                    Text(alert.title)
                                        .font(.liquidBody.bold())
                                        .foregroundStyle(.white)
                                }
                                Text(alert.message)
                                    .font(.liquidCaption)
                                    .foregroundStyle(.white.opacity(0.8))
                            }
                            .padding()
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .background(
                                alert.type == .error
                                    ? Color.red.opacity(0.8)
                                    : alert.type == .success
                                        ? Color.green.opacity(0.8) : Color.blue.opacity(0.8)
                            )
                            .clipShape(RoundedRectangle(cornerRadius: 12))
                            .padding(.horizontal)
                        }

                        // Leaderboard / Playlist
                        Text("Playlist")
                            .font(.liquidTitle)
                            .foregroundStyle(Color.white)
                            .padding(.horizontal)

                        if store.tracks.isEmpty && !store.isLoading {
                            Text("No tracks yet. Add one!")
                                .font(.liquidBody)
                                .foregroundStyle(Color.white.opacity(0.6))
                                .padding(.horizontal)
                        } else {
                            LazyVStack(spacing: 12) {
                                // Sort tracks by vote count descending
                                let sortedTracks = store.tracks.map { track -> (Track, Int, Bool) in
                                    let tallyItem = store.tally.first(where: {
                                        $0.track == track.id
                                            || $0.track == track.providerTrackId
                                    })
                                    let count = tallyItem?.count ?? 0
                                    let isMyVote = tallyItem?.isMyVote ?? false
                                    return (track, count, isMyVote)
                                }.sorted { $0.1 > $1.1 }

                                ForEach(Array(sortedTracks.enumerated()), id: \.element.0.id) {
                                    index, pair in
                                    let (track, count, isMyVote) = pair
                                    TrackRow(
                                        index: index + 1,
                                        track: track,
                                        voteCount: count,
                                        isMyVote: isMyVote,
                                        isOffline: store.isOffline,
                                        onVote: {
                                            store.send(.voteButtonTapped(trackId: track.id))
                                        }
                                    )
                                    .transition(.scale.combined(with: .opacity))
                                    .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                                        Button(role: .destructive) {
                                            store.send(.removeTrackButtonTapped(trackId: track.id))
                                        } label: {
                                            Label("Remove", systemImage: "trash")
                                        }
                                    }
                                }
                            }
                            .padding(.horizontal)
                        }

                        // Add Track Button
                        LiquidButton(
                            useGlass: true,
                            action: {
                                if !store.isOffline {
                                    store.send(.addTrackButtonTapped)
                                }
                            }
                        ) {
                            HStack {
                                Image(systemName: "plus.circle.fill")
                                    .font(.liquidIcon)
                                Text("Add Track")
                                    .font(.liquidBody.bold())
                                Spacer()
                            }
                            .foregroundStyle(store.isOffline ? Color.gray : Color.white)
                            .frame(maxWidth: .infinity)
                        }
                        .disabled(store.isOffline)
                        .padding(.horizontal)
                        .padding(.top, 20)

                        Spacer(minLength: 100)
                    }
                    .padding(.top)
                }
            }

            if store.isLoading {
                ProgressView()
                    .tint(.white)
            }
        }
        .onAppear {
            store.send(.onAppear)
        }
        .navigationBarTitleDisplayMode(.inline)
        .sheet(item: $store.scope(state: \.musicSearch, action: \.musicSearch)) { searchStore in
            MusicSearchView(store: searchStore)
        }
    }
}

struct TrackRow: View {
    let index: Int
    let track: Track
    let voteCount: Int
    let isMyVote: Bool
    let isOffline: Bool
    let onVote: () -> Void

    @State private var isAnimating = false

    var body: some View {
        GlassView(cornerRadius: 16)
            .frame(height: 80)
            .overlay(
                HStack(spacing: 12) {
                    Text("#\(index)")
                        .font(.liquidTitle)
                        .foregroundStyle(Color.white.opacity(0.5))
                        .frame(width: 35)

                    // Thumbnail
                    if let url = track.thumbnailUrl {
                        AsyncImage(url: url) { phase in
                            switch phase {
                            case .success(let image):
                                image.resizable().aspectRatio(contentMode: .fill)
                            default:
                                Color.gray.opacity(0.3)
                            }
                        }
                        .frame(width: 50, height: 50)
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                    } else {
                        Rectangle()
                            .fill(Color.gray.opacity(0.3))
                            .frame(width: 50, height: 50)
                            .clipShape(RoundedRectangle(cornerRadius: 8))
                    }

                    VStack(alignment: .leading, spacing: 4) {
                        Text(track.title)
                            .font(.liquidBody.bold())
                            .foregroundStyle(Color.white)
                            .lineLimit(1)

                        Text(track.artist)
                            .font(.liquidCaption)
                            .foregroundStyle(Color.white.opacity(0.7))
                            .lineLimit(1)

                        Text("\(voteCount) \(voteCount == 1 ? "vote" : "votes")")
                            .font(.caption2)
                            .foregroundStyle(Color.liquidAccent)
                    }

                    Spacer()

                    Button(action: {
                        withAnimation(
                            .spring(response: 0.3, dampingFraction: 0.4, blendDuration: 0)
                        ) {
                            isAnimating = true
                        }
                        onVote()
                        // Reset animation
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
                            withAnimation {
                                isAnimating = false
                            }
                        }
                    }) {
                        Image(systemName: isMyVote ? "arrow.up.circle.fill" : "arrow.up.circle")
                            .font(.system(size: 28))
                            .foregroundStyle(
                                isOffline ? Color.gray : (isMyVote ? Color.green : Color.white)
                            )
                            .scaleEffect(isAnimating ? 1.3 : 1.0)
                    }
                    .disabled(isOffline || isMyVote)
                }
                .padding(.horizontal, 16)
            )
    }
}

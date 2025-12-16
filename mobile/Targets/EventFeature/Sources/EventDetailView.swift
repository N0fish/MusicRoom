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

    @Namespace private var animation

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

                        // Now Playing Section
                        if let currentTrack = store.tracks.first(where: { $0.status == "playing" })
                        {
                            VStack(alignment: .leading, spacing: 8) {
                                Text("Now Playing")
                                    .font(.liquidTitle)
                                    .foregroundStyle(Color.white)
                                    .padding(.horizontal)

                                TrackRow(
                                    index: 0,
                                    track: currentTrack,
                                    voteCount: currentTrack.voteCount ?? 0,
                                    isMyVote: false,  // Can't vote on playing
                                    isOffline: store.isOffline,
                                    onVote: {},
                                    showVote: false,
                                    timeRemaining: store.timeRemaining,
                                    totalDuration: store.currentTrackDuration
                                )
                                .matchedGeometryEffect(id: currentTrack.id, in: animation)
                                .transition(.scale(scale: 0.9).combined(with: .opacity))
                                .padding(.horizontal)

                                // Next Track Control (Play/Skip)
                                Button {
                                    store.send(.nextTrackButtonTapped)
                                } label: {
                                    HStack {
                                        Image(systemName: "forward.end.fill")
                                        Text("Next Track")
                                    }
                                    .font(.liquidBody.bold())
                                    .foregroundStyle(.white)
                                    .padding()
                                    .background(Color.white.opacity(0.2))
                                    .clipShape(RoundedRectangle(cornerRadius: 12))
                                }
                                .padding(.horizontal)
                            }
                        } else {
                            // Check if we have tracks ready to play
                            let queuedTracks = store.tracks.filter {
                                $0.status == "queued" || $0.status == nil
                            }

                            if !queuedTracks.isEmpty {
                                Button {
                                    store.send(.nextTrackButtonTapped)
                                } label: {
                                    HStack {
                                        Image(systemName: "play.fill")
                                        Text("Start Radio")
                                    }
                                    .font(.liquidBody.bold())
                                    .foregroundStyle(.white)
                                    .padding()
                                    .frame(maxWidth: .infinity)
                                    .background(Color.green.opacity(0.8))
                                    .clipShape(RoundedRectangle(cornerRadius: 12))
                                }
                                .padding(.horizontal)
                            } else if !store.isLoading && !store.tracks.isEmpty {
                                // Tracks exist but none are queued (all played) -> Event Finished
                                VStack(spacing: 8) {
                                    Image(systemName: "flag.checkered")
                                        .font(.system(size: 32))
                                        .foregroundStyle(Color.white.opacity(0.8))
                                        .padding(.bottom, 4)

                                    Text("Event Finished")
                                        .font(.liquidH2)
                                        .foregroundStyle(Color.white)

                                    Text("You can start anew!")
                                        .font(.liquidCaption)
                                        .foregroundStyle(Color.white.opacity(0.7))
                                }
                                .padding()
                                .frame(maxWidth: .infinity)
                                .background(Color.black.opacity(0.3))
                                .clipShape(RoundedRectangle(cornerRadius: 16))
                                .padding(.horizontal)
                            }
                        }

                        // Leaderboard / Playlist
                        Text("Up Next")
                            .font(.liquidTitle)
                            .foregroundStyle(Color.white)
                            .padding(.horizontal)

                        if store.tracks.filter({ $0.status == "queued" || $0.status == nil })
                            .isEmpty && !store.isLoading
                        {
                            Text("Queue empty. Add tracks!")
                                .font(.liquidBody)
                                .foregroundStyle(Color.white.opacity(0.6))
                                .padding(.horizontal)
                        } else {
                            LazyVStack(spacing: 12) {
                                // Filter only queued (or nil status if legacy)
                                let queuedTracks = store.tracks.filter {
                                    $0.status == "queued" || $0.status == nil
                                }

                                // Sort tracks by vote count descending
                                let sortedTracks = queuedTracks.sorted {
                                    ($0.voteCount ?? 0) > ($1.voteCount ?? 0)
                                }

                                ForEach(sortedTracks) { track in
                                    TrackRow(
                                        index: (sortedTracks.firstIndex(of: track) ?? 0) + 1,
                                        track: track,
                                        voteCount: track.voteCount ?? 0,
                                        isMyVote: track.isVoted ?? false,
                                        isOffline: store.isOffline,
                                        onVote: {
                                            store.send(.voteButtonTapped(trackId: track.id))
                                        }
                                    )
                                    .matchedGeometryEffect(id: track.id, in: animation)
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
                            .animation(
                                .spring(response: 0.6, dampingFraction: 0.8), value: store.tracks
                            )
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
        .overlay(alignment: .top) {
            if let alert = store.userAlert {
                HStack(spacing: 12) {
                    Image(
                        systemName: alert.type == .error
                            ? "exclamationmark.triangle.fill"
                            : (alert.type == .success
                                ? "checkmark.circle.fill" : "info.circle.fill")
                    )
                    .font(.system(size: 20))
                    .foregroundStyle(.white)

                    VStack(alignment: .leading, spacing: 2) {
                        Text(alert.title)
                            .font(.liquidBody.bold())
                            .foregroundStyle(.white)
                        Text(alert.message)
                            .font(.liquidCaption)
                            .foregroundStyle(.white.opacity(0.8))
                    }
                    Spacer()
                }
                .padding()
                .background(
                    (alert.type == .error
                        ? Color.red
                        : alert.type == .success
                            ? Color.green : Color.blue)
                        .opacity(0.9)
                        .shadow(.drop(radius: 10, y: 5))
                )
                .clipShape(RoundedRectangle(cornerRadius: 16))
                .padding(.horizontal)
                .padding(.top, 8)  // Add some safe area padding
                .transition(
                    .asymmetric(
                        insertion: .move(edge: .top).combined(with: .opacity),
                        removal: .move(edge: .top).combined(with: .opacity)
                    )
                )
                .onTapGesture {
                    store.send(.dismissInfo, animation: .spring())
                }
            }
        }
        .animation(.spring(response: 0.4, dampingFraction: 0.7), value: store.userAlert)
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
    var showVote: Bool = true  // Parameter to hide vote button
    var timeRemaining: TimeInterval? = nil
    var totalDuration: TimeInterval? = nil

    @State private var isAnimating = false

    private func formatDuration(ms: Int) -> String {
        let duration = TimeInterval(ms) / 1000
        let formatter = DateComponentsFormatter()
        formatter.allowedUnits = [.minute, .second]
        formatter.unitsStyle = .positional
        formatter.zeroFormattingBehavior = .pad
        return formatter.string(from: duration) ?? "0:00"
    }

    var body: some View {
        GlassView(cornerRadius: 16)
            .frame(height: 80)
            .overlay(
                HStack(spacing: 12) {
                    // Index / Playing Icon
                    if index == 0 {
                        Image(systemName: "speaker.wave.3.fill")
                            .font(.system(size: 20))
                            .foregroundStyle(Color.liquidAccent)
                            .frame(width: 35)
                    } else {
                        Text("#\(index)")
                            .font(.system(size: 20, weight: .bold, design: .rounded))
                            .minimumScaleFactor(0.8)
                            .foregroundStyle(Color.white.opacity(0.5))
                            .frame(width: 35)
                    }

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

                        HStack {
                            Text(track.artist)
                                .font(.liquidCaption)
                                .foregroundStyle(Color.white.opacity(0.7))
                                .lineLimit(1)

                            if let remaining = timeRemaining, let total = totalDuration, total > 0 {
                                VStack(alignment: .leading, spacing: 2) {
                                    ProgressView(value: total - remaining, total: total)
                                        .tint(.liquidAccent)
                                        .scaleEffect(y: 0.5)

                                    HStack {
                                        Text(formatDuration(ms: Int((total - remaining) * 1000)))
                                        Spacer()
                                        Text("-" + formatDuration(ms: Int(remaining * 1000)))
                                    }
                                    .font(.caption2)
                                    .monospacedDigit()
                                    .foregroundStyle(Color.white.opacity(0.6))
                                }
                                .padding(.top, 4)
                            } else if let durationMs = track.durationMs {
                                Text("• " + formatDuration(ms: durationMs))
                                    .font(.liquidCaption)
                                    .foregroundStyle(Color.white.opacity(0.5))
                            }
                        }

                        if timeRemaining == nil {
                            Text("\(voteCount) \(voteCount == 1 ? "vote" : "votes")")
                                .font(.caption2)
                                .foregroundStyle(Color.liquidAccent)
                        }
                    }

                    Spacer()

                    if showVote {
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
                    } else {
                        Image(systemName: "waveform")  // Paying icon
                            .foregroundStyle(.white)
                            .font(.system(size: 24))
                    }
                }
                .padding(.horizontal, 16)
            )
    }
}

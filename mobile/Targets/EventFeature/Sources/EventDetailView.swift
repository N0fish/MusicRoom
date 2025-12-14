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

                        // Error/Success Messages
                        if let error = store.errorMessage {
                            Text(error)
                                .foregroundStyle(.white)
                                .padding()
                                .background(Color.liquidAccent.opacity(0.8))
                                .clipShape(RoundedRectangle(cornerRadius: 12))
                                .padding(.horizontal)
                        }

                        if let success = store.successMessage {
                            Text(success)
                                .foregroundStyle(.white)
                                .padding()
                                .background(Color.green.opacity(0.8))
                                .clipShape(RoundedRectangle(cornerRadius: 12))
                                .padding(.horizontal)
                        }

                        // Leaderboard
                        Text("Leaderboard")
                            .font(.liquidTitle)
                            .foregroundStyle(Color.white)
                            .padding(.horizontal)

                        if store.tally.isEmpty && !store.isLoading {
                            Text("No votes yet. Be the first!")
                                .font(.liquidBody)
                                .foregroundStyle(Color.white.opacity(0.6))
                                .padding(.horizontal)
                        } else {
                            LazyVStack(spacing: 12) {
                                ForEach(Array(store.tally.enumerated()), id: \.element.track) {
                                    index, item in
                                    TallyRow(
                                        index: index + 1,
                                        item: item,
                                        isMyVote: false,  // Need backend support to know
                                        onVote: {
                                            store.send(.voteButtonTapped(trackId: item.track))
                                        }
                                    )
                                }
                            }
                            .padding(.horizontal)
                        }

                        // Add Track Button
                        Button {
                            store.send(.addTrackButtonTapped)
                        } label: {
                            HStack {
                                Image(systemName: "plus.circle.fill")
                                    .font(.system(size: 24))
                                Text("Add Track")
                                    .font(.liquidBody.bold())
                                Spacer()
                            }
                            .foregroundStyle(Color.white)
                            .padding()
                            .background(
                                GlassView(cornerRadius: 16)
                            )
                        }
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

struct TallyRow: View {
    let index: Int
    let item: MusicRoomAPIClient.TallyItem
    let isMyVote: Bool
    let onVote: () -> Void

    var body: some View {
        GlassView(cornerRadius: 16)
            .frame(height: 70)
            .overlay(
                HStack(spacing: 16) {
                    Text("#\(index)")
                        .font(.liquidTitle)
                        .foregroundStyle(Color.white.opacity(0.5))
                        .frame(width: 40)

                    VStack(alignment: .leading, spacing: 4) {
                        Text(item.track)  // Placeholder until we resolve Track ID -> Title
                            .font(.liquidBody.bold())
                            .foregroundStyle(Color.white)

                        Text("\(item.count) votes")
                            .font(.liquidCaption)
                            .foregroundStyle(Color.white.opacity(0.7))
                    }

                    Spacer()

                    Button(action: onVote) {
                        Image(systemName: isMyVote ? "arrow.up.circle.fill" : "arrow.up.circle")
                            .font(.system(size: 28))
                            .foregroundStyle(isMyVote ? Color.green : Color.white)
                    }
                }
                .padding(.horizontal, 16)
            )
    }
}

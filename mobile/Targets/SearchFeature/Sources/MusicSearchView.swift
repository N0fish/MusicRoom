import ComposableArchitecture
import MusicRoomDomain
import MusicRoomUI
import SwiftUI

public struct MusicSearchView: View {
    @Bindable var store: StoreOf<MusicSearchFeature>

    public init(store: StoreOf<MusicSearchFeature>) {
        self.store = store
    }

    public var body: some View {
        ZStack {
            LiquidBackground()
                .ignoresSafeArea()

            VStack(spacing: 0) {
                // Header
                HStack {
                    Text("Add Track")
                        .font(.title)  // Fallback
                        .foregroundStyle(Color.white)

                    Spacer()
                }
                .padding()

                // Search Bar
                HStack {
                    Image(systemName: "magnifyingglass")
                        .foregroundStyle(Color.white.opacity(0.6))

                    TextField("Search songs...", text: $store.query)
                        .textFieldStyle(.plain)
                        .foregroundStyle(Color.white)
                        .font(.body)  // Fallback font
                        .onSubmit {
                            store.send(.search)
                        }

                    if !store.query.isEmpty {
                        Button {
                            store.send(.binding(.set(\.query, "")))
                        } label: {
                            Image(systemName: "xmark.circle.fill")
                                .foregroundStyle(Color.white.opacity(0.6))
                        }
                    }
                }
                .padding()
                .background(
                    GlassView(cornerRadius: 12) {
                        Color.clear
                    }
                )
                .padding(.horizontal)
                .padding(.bottom)

                // Content
                if store.isLoading {
                    Spacer()
                    ProgressView()
                        .tint(.white)
                        .scaleEffect(1.5)
                    Spacer()
                } else if let error = store.errorMessage {
                    Spacer()
                    VStack {
                        Image(systemName: "exclamationmark.triangle")
                            .font(.system(size: 40))
                            .foregroundStyle(Color.red)  // Fallback for liquidPink
                            .padding(.bottom, 8)
                        Text(error)
                            .font(.body)
                            .foregroundStyle(Color.white)
                            .multilineTextAlignment(.center)
                    }
                    .padding()
                    Spacer()
                } else if store.results.isEmpty {
                    Spacer()
                    if !store.query.isEmpty {
                        Text("No results found")
                            .font(.body)
                            .foregroundStyle(Color.white.opacity(0.7))
                    } else {
                        VStack(spacing: 8) {
                            Image(systemName: "music.note.list")
                                .font(.system(size: 40))
                                .foregroundStyle(Color.blue.opacity(0.5))  // Fallback for liquidBlue
                            Text("Search for your favorite tracks")
                                .font(.body)
                                .foregroundStyle(Color.white.opacity(0.7))
                        }
                    }
                    Spacer()
                } else {
                    List {
                        ForEach(store.results) { item in
                            Button {
                                store.send(.trackTapped(item))
                            } label: {
                                SearchResultRow(item: item)
                            }
                            .listRowBackground(Color.clear)
                            .listRowSeparator(.hidden)
                        }
                    }
                    .listStyle(.plain)
                }
            }
        }
        .navigationBarTitleDisplayMode(.inline)
    }
}

struct SearchResultRow: View {
    let item: MusicSearchItem

    var body: some View {
        GlassView(cornerRadius: 12) {
            HStack(spacing: 12) {
                // Thumbnail placeholder or AsyncImage
                ZStack {
                    if let url = item.thumbnailUrl {
                        AsyncImage(url: url) { image in
                            image.resizable().aspectRatio(contentMode: .fill)
                        } placeholder: {
                            Color.black.opacity(0.3)
                        }
                    } else {
                        Color.black.opacity(0.3)  // Fallback for liquidBlack
                        Image(systemName: "music.note")
                            .foregroundStyle(Color.white.opacity(0.5))
                    }
                }
                .frame(width: 50, height: 50)
                .clipShape(RoundedRectangle(cornerRadius: 8))

                VStack(alignment: .leading, spacing: 4) {
                    Text(item.title)
                        .font(.headline)  // Fallback
                        .foregroundStyle(Color.white)
                        .lineLimit(1)

                    Text(item.artist)
                        .font(.caption)  // Fallback
                        .foregroundStyle(Color.white.opacity(0.7))
                        .lineLimit(1)
                }

                Spacer()

                Image(systemName: "plus.circle.fill")
                    .font(.system(size: 24))
                    .foregroundStyle(Color.blue)  // Fallback
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 12)  // Added vertical padding since frame height was removed from GlassView
        }
    }
}

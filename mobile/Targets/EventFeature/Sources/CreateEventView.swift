import ComposableArchitecture
import MusicRoomDomain
import MusicRoomUI
import SwiftUI

public struct CreateEventView: View {
    @Bindable var store: StoreOf<CreateEventFeature>

    public init(store: StoreOf<CreateEventFeature>) {
        self.store = store
    }

    public var body: some View {
        NavigationStack {
            ZStack {
                LiquidBackground()
                    .ignoresSafeArea()

                VStack(spacing: 24) {
                    // Form Content
                    VStack(spacing: 16) {
                        TextField("Event Name", text: $store.name)
                            .padding()
                            .background(GlassView(cornerRadius: 12))
                            .foregroundStyle(.white)
                            .font(.liquidBody)

                        VStack(alignment: .leading, spacing: 8) {
                            Text("Visibility")
                                .font(.liquidCaption)
                                .foregroundStyle(.white.opacity(0.8))
                                .padding(.leading, 4)

                            Picker("Visibility", selection: $store.visibility) {
                                ForEach([EventVisibility.publicEvent, .privateEvent], id: \.self) {
                                    vis in
                                    Text(vis.label).tag(vis)
                                }
                            }
                            .pickerStyle(.segmented)
                            .colorScheme(.dark)  // Force dark mode for segmented picker visibility
                        }

                        VStack(alignment: .leading, spacing: 8) {
                            Text("Who can vote?")
                                .font(.liquidCaption)
                                .foregroundStyle(.white.opacity(0.8))
                                .padding(.leading, 4)

                            Picker("License", selection: $store.licenseMode) {
                                ForEach(EventLicenseMode.allCases, id: \.self) { mode in
                                    Text(mode.label).tag(mode)
                                }
                            }
                            .pickerStyle(.menu)
                            .tint(.white)
                            .padding()
                            .background(GlassView(cornerRadius: 12))
                        }
                    }
                    .padding()
                    .background(GlassView(cornerRadius: 20))
                    .padding(.horizontal)

                    if let error = store.errorMessage {
                        Text(error)
                            .foregroundStyle(.red)
                            .font(.liquidCaption)
                    }

                    Button(action: { store.send(.createButtonTapped) }) {
                        if store.isLoading {
                            ProgressView()
                                .tint(.white)
                        } else {
                            Text("Create Event")
                                .font(.liquidBody.bold())
                                .foregroundStyle(.white)
                                .frame(maxWidth: .infinity)
                                .padding()
                                .background(Color.liquidAccent)
                                .clipShape(RoundedRectangle(cornerRadius: 16))
                        }
                    }
                    .disabled(store.isLoading)
                    .padding(.horizontal)

                    Spacer()
                }
                .padding(.top, 40)
            }
            .navigationTitle("New Event")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        // Dismiss handled by parent or environment
                    }
                }
            }
        }
    }
}

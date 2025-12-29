import AppSettingsClient
import AppSupportClients  // For Friend model & LocationClient
import ComposableArchitecture
import CoreLocation
import MusicRoomAPI
import MusicRoomDomain
import SwiftUI  // For Binding

@Reducer
public struct CreateEventFeature: Sendable {
    @ObservableState
    public struct State: Equatable, Sendable {
        public var name: String = ""
        public var visibility: EventVisibility = .publicEvent
        public var licenseMode: EventLicenseMode = .everyone
        public var isLoading: Bool = false
        public var errorMessage: String?

        // Invite Friends
        public var friends: [Friend] = []
        public var selectedFriendIDs: Set<String> = []

        // Geo + Time
        public var voteStart: Date = Date().addingTimeInterval(60)
        public var voteEnd: Date = Date().addingTimeInterval(3600 * 24)  // Default 24h
        public var geoLat: Double?
        public var geoLng: Double?
        public var geoRadiusM: Int = 100
        public var isGettingLocation: Bool = false

        public init() {}
    }

    public enum Action: BindableAction, Sendable, Equatable {
        case binding(BindingAction<State>)
        case onAppear
        case friendsLoaded(Result<[Friend], ErrorBinder>)  // Error needs to be Equatable
        case toggleFriendSelection(String)
        case createButtonTapped
        case cancelButtonTapped
        case createResponse(Result<Event, ErrorBinder>)

        // Geo
        case getCurrentLocation
        case locationLoaded(Result<EquatableCoordinate, ErrorBinder>)
    }

    public struct EquatableCoordinate: Equatable, Sendable {
        public let latitude: Double
        public let longitude: Double
        public init(_ coordinate: CLLocationCoordinate2D) {
            self.latitude = coordinate.latitude
            self.longitude = coordinate.longitude
        }
    }

    // Simple Error Wrapper for Equatable
    public struct ErrorBinder: Error, Equatable, Sendable {
        let message: String
        public init(_ error: Error) {
            self.message = error.localizedDescription
        }
    }

    @Dependency(\.musicRoomAPI) var musicRoomAPI
    @Dependency(\.friendsClient) var friendsClient
    @Dependency(\.locationClient) var locationClient
    @Dependency(\.appSettings) var appSettings
    @Dependency(\.date) var date
    @Dependency(\.dismiss) var dismiss

    public init() {}

    public var body: some ReducerOf<Self> {
        let friendsClient = self.friendsClient
        let musicRoomAPI = self.musicRoomAPI
        let locationClient = self.locationClient

        BindingReducer()
        Reduce { state, action in
            switch action {
            case .binding(\.visibility):
                if state.visibility == .privateEvent && state.licenseMode == .everyone {
                    state.licenseMode = .invitedOnly
                }
                return .none

            case .binding(\.voteStart):
                let minStart = date.now.addingTimeInterval(60)
                if state.voteStart < minStart {
                    state.voteStart = minStart
                }
                let minEnd = state.voteStart.addingTimeInterval(60)
                if state.voteEnd < minEnd {
                    state.voteEnd = minEnd
                }
                return .none

            case .binding(\.voteEnd):
                let minEnd = state.voteStart.addingTimeInterval(60)
                if state.voteEnd < minEnd {
                    state.voteEnd = minEnd
                }
                return .none

            case .binding:
                return .none

            case .onAppear:
                return .run { send in
                    do {
                        let friends = try await friendsClient.listFriends()
                        await send(.friendsLoaded(.success(friends)))
                    } catch {
                        await send(.friendsLoaded(.failure(ErrorBinder(error))))
                    }
                }

            case .friendsLoaded(.success(let friends)):
                state.friends = friends
                return .none

            case .friendsLoaded(.failure(let error)):
                // Just log or show error? Maybe failing to load friends shouldn't block creation.
                print("Failed to load friends: \(error.message)")
                return .none

            case .toggleFriendSelection(let id):
                if state.selectedFriendIDs.contains(id) {
                    state.selectedFriendIDs.remove(id)
                } else {
                    state.selectedFriendIDs.insert(id)
                }
                return .none

            case .getCurrentLocation:
                state.isGettingLocation = true
                return .run { send in
                    do {
                        await locationClient.requestWhenInUseAuthorization()
                        let location = try await locationClient.getCurrentLocation()
                        await send(.locationLoaded(.success(EquatableCoordinate(location))))
                    } catch {
                        await send(.locationLoaded(.failure(ErrorBinder(error))))
                    }
                }

            case .locationLoaded(.success(let coordinate)):
                state.isGettingLocation = false
                state.geoLat = coordinate.latitude
                state.geoLng = coordinate.longitude
                return .none

            case .locationLoaded(.failure(let error)):
                state.isGettingLocation = false
                state.errorMessage = "Failed to get location: \(error.message)"
                return .none

            case .createButtonTapped:
                guard !state.name.isEmpty else {
                    state.errorMessage = "Event name cannot be empty."
                    return .none
                }
                state.isLoading = true
                state.errorMessage = nil

                return .run {
                    [
                        name = state.name, visibility = state.visibility,
                        licenseMode = state.licenseMode,
                        selectedFriendIDs = state.selectedFriendIDs,
                        voteStart = state.voteStart, voteEnd = state.voteEnd,
                        geoLat = state.geoLat, geoLng = state.geoLng, geoRadiusM = state.geoRadiusM
                    ] send in

                    var finalRequest = CreateEventRequest(
                        name: name,
                        visibility: visibility,
                        licenseMode: licenseMode
                    )

                    if licenseMode == .geoTime {
                        finalRequest = CreateEventRequest(
                            name: name,
                            visibility: visibility,
                            licenseMode: licenseMode,
                            geoLat: geoLat,
                            geoLng: geoLng,
                            geoRadiusM: geoRadiusM,
                            voteStart: voteStart,
                            voteEnd: voteEnd
                        )
                    }
                    let request = finalRequest

                    do {
                        let event = try await musicRoomAPI.createEvent(request)

                        // Send Invites if any
                        // We do this in parallel or serial? Serial is safer.
                        // Errors here? Should we fail the whole flow?
                        // Ideally we warn user "Event created but failed to invite X".
                        // For MVP, try best effort.

                        if !selectedFriendIDs.isEmpty {
                            await withTaskGroup(of: Void.self) { group in
                                for friendID in selectedFriendIDs {
                                    group.addTask {
                                        do {
                                            try await musicRoomAPI.inviteUser(event.id, friendID)
                                        } catch {
                                            print("Failed to invite \(friendID): \(error)")
                                        }
                                    }
                                }
                            }
                        }

                        await send(.createResponse(.success(event)))

                    } catch {
                        await send(.createResponse(.failure(ErrorBinder(error))))
                    }
                }

            case .createResponse(.success):
                state.isLoading = false
                return .run { _ in await dismiss() }

            case .createResponse(.failure(let error)):
                state.isLoading = false
                state.errorMessage = error.message
                return .none

            case .cancelButtonTapped:
                return .run { _ in await dismiss() }
            }
        }
    }

    private func normalizeUrl(_ url: String?) -> String? {
        guard let url, !url.isEmpty else { return url }
        if url.lowercased().hasPrefix("http") { return url }
        let settings = appSettings.load()
        let baseUrlString = settings.backendURL.absoluteString.trimmingCharacters(
            in: .init(charactersIn: "/"))
        let cleanPath = url.trimmingCharacters(in: .init(charactersIn: "/"))
        return "\(baseUrlString)/\(cleanPath)"
    }
}

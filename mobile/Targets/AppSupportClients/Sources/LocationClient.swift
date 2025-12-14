import CoreLocation
import Dependencies
import Foundation

public struct LocationClient: Sendable {
    public var requestWhenInUseAuthorization: @Sendable () async -> Void
    public var authorizationStatus: @Sendable () async -> CLAuthorizationStatus
    public var getCurrentLocation: @Sendable () async throws -> CLLocationCoordinate2D
}

extension DependencyValues {
    public var locationClient: LocationClient {
        get { self[LocationClient.self] }
        set { self[LocationClient.self] = newValue }
    }
}

extension LocationClient: DependencyKey {
    public static var liveValue: LocationClient {
        let manager = LocationManager()
        return LocationClient(
            requestWhenInUseAuthorization: {
                await manager.requestAuthorization()
            },
            authorizationStatus: {
                await manager.authorizationStatus
            },
            getCurrentLocation: {
                try await manager.requestLocation()
            }
        )
    }

    public static var testValue: LocationClient {
        LocationClient(
            requestWhenInUseAuthorization: {},
            authorizationStatus: { .authorizedWhenInUse },
            getCurrentLocation: { CLLocationCoordinate2D(latitude: 48.8966, longitude: 2.3185) }
        )
    }
}

private actor LocationManager: NSObject, CLLocationManagerDelegate {
    private let manager = CLLocationManager()
    private var locationContinuation: CheckedContinuation<CLLocationCoordinate2D, Error>?

    var authorizationStatus: CLAuthorizationStatus {
        manager.authorizationStatus
    }

    override init() {
        super.init()
        manager.delegate = self
        manager.desiredAccuracy = kCLLocationAccuracyNearestTenMeters
    }

    func requestAuthorization() {
        manager.requestWhenInUseAuthorization()
    }

    func requestLocation() async throws -> CLLocationCoordinate2D {
        if let existing = locationContinuation {
            existing.resume(throwing: LocationError.cancelled)
            locationContinuation = nil
        }

        return try await withCheckedThrowingContinuation { continuation in
            self.locationContinuation = continuation
            manager.requestLocation()
        }
    }

    nonisolated func locationManager(
        _ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]
    ) {
        Task {
            await self.handleLocationUpdate(locations.first)
        }
    }

    nonisolated func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        Task {
            await self.handleLocationError(error)
        }
    }

    nonisolated func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        // We could expose a stream of auth status if needed
    }

    private func handleLocationUpdate(_ location: CLLocation?) {
        if let coordinate = location?.coordinate {
            locationContinuation?.resume(returning: coordinate)
            locationContinuation = nil
        }
    }

    private func handleLocationError(_ error: Error) {
        locationContinuation?.resume(throwing: error)
        locationContinuation = nil
    }
}

public enum LocationError: Error {
    case cancelled
    case unknown
}

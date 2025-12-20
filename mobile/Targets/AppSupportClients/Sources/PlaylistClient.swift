import Dependencies
import Foundation
import MusicRoomDomain
import Security

public struct PlaylistClient: Sendable {
    public var list: @Sendable () async throws -> [Playlist]
    public var create: @Sendable (CreatePlaylistRequest) async throws -> Playlist
    public var get: @Sendable (String) async throws -> PlaylistResponse
    public var update: @Sendable (String, UpdatePlaylistRequest) async throws -> Playlist
    public var addTrack: @Sendable (String, AddTrackRequest) async throws -> Track
    public var deleteTrack: @Sendable (String, String) async throws -> Void
    public var moveTrack: @Sendable (String, String, Int) async throws -> Void
    public var addInvite: @Sendable (String, String) async throws -> Void
}

extension PlaylistClient: DependencyKey {
    public static let liveValue = PlaylistClient.live()

    public static let testValue = PlaylistClient(
        list: { [] },
        create: { _ in .mock() },
        get: { _ in .mock() },
        update: { _, _ in .mock() },
        addTrack: { _, _ in .mock() },
        deleteTrack: { _, _ in },
        moveTrack: { _, _, _ in },
        addInvite: { _, _ in }
    )
}

extension DependencyValues {
    public var playlistClient: PlaylistClient {
        get { self[PlaylistClient.self] }
        set { self[PlaylistClient.self] = newValue }
    }
}

extension PlaylistClient {
    static func live(urlSession: URLSession = .shared) -> Self {
        let keychain = KeychainHelper()

        @Sendable func performRequest<T: Decodable & Sendable>(_ request: URLRequest) async throws
            -> T
        {
            var request = request
            if let token = keychain.read("accessToken") {
                request.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
            }

            let (data, response) = try await urlSession.data(for: request)

            guard let httpResponse = response as? HTTPURLResponse else {
                throw URLError(.badServerResponse)
            }

            guard (200...299).contains(httpResponse.statusCode) else {
                throw URLError(.init(rawValue: httpResponse.statusCode))
            }

            let decoder = JSONDecoder()
            decoder.dateDecodingStrategy = .iso8601
            return try decoder.decode(T.self, from: data)
        }

        @Sendable func performRequestNoContent(_ request: URLRequest) async throws {
            var request = request
            if let token = keychain.read("accessToken") {
                request.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
            }

            let (_, response) = try await urlSession.data(for: request)

            guard let httpResponse = response as? HTTPURLResponse else {
                throw URLError(.badServerResponse)
            }

            guard (200...299).contains(httpResponse.statusCode) else {
                throw URLError(.init(rawValue: httpResponse.statusCode))
            }
        }

        return Self(
            list: {
                let baseUrl = BaseURL.resolve()
                guard let url = URL(string: "\(baseUrl)/playlists") else { throw URLError(.badURL) }
                let response: [Playlist]? = try await performRequest(URLRequest(url: url))
                return response ?? []
            },
            create: { payload in
                let baseUrl = BaseURL.resolve()
                guard let url = URL(string: "\(baseUrl)/playlists") else { throw URLError(.badURL) }
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")
                request.httpBody = try JSONEncoder().encode(payload)
                return try await performRequest(request)
            },
            get: { id in
                let baseUrl = BaseURL.resolve()
                guard let url = URL(string: "\(baseUrl)/playlists/\(id)") else {
                    throw URLError(.badURL)
                }
                return try await performRequest(URLRequest(url: url))
            },
            update: { id, payload in
                let baseUrl = BaseURL.resolve()
                guard let url = URL(string: "\(baseUrl)/playlists/\(id)") else {
                    throw URLError(.badURL)
                }
                var request = URLRequest(url: url)
                request.httpMethod = "PATCH"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")
                request.httpBody = try JSONEncoder().encode(payload)
                return try await performRequest(request)
            },
            addTrack: { id, payload in
                let baseUrl = BaseURL.resolve()
                guard let url = URL(string: "\(baseUrl)/playlists/\(id)/tracks") else {
                    throw URLError(.badURL)
                }
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")
                request.httpBody = try JSONEncoder().encode(payload)
                return try await performRequest(request)
            },
            deleteTrack: { playlistId, trackId in
                let baseUrl = BaseURL.resolve()
                guard let url = URL(string: "\(baseUrl)/playlists/\(playlistId)/tracks/\(trackId)")
                else { throw URLError(.badURL) }
                var request = URLRequest(url: url)
                request.httpMethod = "DELETE"
                try await performRequestNoContent(request)
            },
            moveTrack: { playlistId, trackId, newPosition in
                let baseUrl = BaseURL.resolve()
                guard let url = URL(string: "\(baseUrl)/playlists/\(playlistId)/tracks/\(trackId)")
                else { throw URLError(.badURL) }
                var request = URLRequest(url: url)
                request.httpMethod = "PATCH"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")
                let body = ["newPosition": newPosition]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)
                try await performRequestNoContent(request)
            },
            addInvite: { playlistId, userId in
                let baseUrl = BaseURL.resolve()
                guard let url = URL(string: "\(baseUrl)/playlists/\(playlistId)/invites") else {
                    throw URLError(.badURL)
                }
                var request = URLRequest(url: url)
                request.httpMethod = "POST"
                request.addValue("application/json", forHTTPHeaderField: "Content-Type")
                let body = ["userId": userId]
                request.httpBody = try JSONSerialization.data(withJSONObject: body)
                try await performRequestNoContent(request)
            }
        )
    }
}

// MARK: - Keychain Helper (Reused from AuthenticationClient or made public)
// For now, I'll copy the minimal one or assume it's available.
// In a real project, this should be in a shared place within AppSupportClients.

private struct KeychainHelper {
    func read(_ key: String) -> String? {
        let query =
            [
                kSecClass: kSecClassGenericPassword,
                kSecAttrAccount: key,
                kSecReturnData: true,
                kSecMatchLimit: kSecMatchLimitOne,
            ] as [String: Any]

        var dataTypeRef: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &dataTypeRef)

        if status == errSecSuccess, let data = dataTypeRef as? Data {
            return String(data: data, encoding: .utf8)
        }
        return nil
    }
}

// MARK: - Mocks for testValue
extension Playlist {
    static func mock() -> Self {
        .init(
            id: "mock-id", ownerId: "owner-id", name: "Mock Playlist",
            description: "Mock Description")
    }
}

extension PlaylistResponse {
    static func mock() -> Self {
        .init(playlist: .mock(), tracks: [])
    }
}

extension Track {
    static func mock() -> Self {
        .init(
            id: "track-id", title: "Mock Track", artist: "Mock Artist", provider: "deezer",
            providerTrackId: "1", thumbnailUrl: nil)
    }
}

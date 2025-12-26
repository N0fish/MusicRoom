package main

import "errors"

type Config struct {
	Port             string
	OpenAPIFile      string
	AuthURL          string
	UserURL          string
	VoteURL          string
	PlaylistURL      string
	MockURL          string
	RealtimeURL      string
	MusicProviderURL string

	JWTSecret    []byte
	RateLimitRPS int
}

func loadConfigFromEnv() (Config, error) {
	cfg := Config{
		Port:             getenv("PORT", "8080"),
		OpenAPIFile:      getenv("OPENAPI_FILE", "openapi.yaml"),
		AuthURL:          getenv("AUTH_SERVICE_URL", "http://auth-service:3001"),
		UserURL:          getenv("USER_SERVICE_URL", "http://user-service:3005"),
		VoteURL:          getenv("VOTE_SERVICE_URL", "http://vote-service:3003"),
		PlaylistURL:      getenv("PLAYLIST_SERVICE_URL", "http://playlist-service:3002"),
		MockURL:          getenv("MOCK_SERVICE_URL", "http://mock-service:3006"),
		RealtimeURL:      getenv("REALTIME_SERVICE_URL", "http://realtime-service:3004"),
		MusicProviderURL: getenv("MUSIC_PROVIDER_SERVICE_URL", "http://music-provider-service:3007"),
		JWTSecret:        []byte(getenv("JWT_SECRET", "")),
		RateLimitRPS:     getenvInt("RATE_LIMIT_RPS", 20),
	}

	if len(cfg.JWTSecret) == 0 {
		return Config{}, errors.New("api-gateway: JWT_SECRET is empty, cannot start without JWT validation")
	}

	return cfg, nil
}

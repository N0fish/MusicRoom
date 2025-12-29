package main

import (
	"errors"
	"net/netip"
	"strings"
)

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
	frontendBaseURL  string

	JWTSecret    []byte
	RateLimitRPS int

	TrustedProxyCIDRs []netip.Prefix
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
		frontendBaseURL:  getenv("FRONTEND_BASE_URL", "http://localhost:5175"),
		JWTSecret:        []byte(getenv("JWT_SECRET", "")),
		RateLimitRPS:     getenvInt("RATE_LIMIT_RPS", 20),
	}

	if len(cfg.JWTSecret) == 0 {
		return Config{}, errors.New("api-gateway: JWT_SECRET is empty, cannot start without JWT validation")
	}

	cidrs := getenv("TRUSTED_PROXY_CIDRS", "127.0.0.1/32,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")
	pfx, err := parseCIDRList(cidrs)
	if err != nil {
		return Config{}, err
	}
	cfg.TrustedProxyCIDRs = pfx

	return cfg, nil
}

func parseCIDRList(raw string) ([]netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]netip.Prefix, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pr, err := netip.ParsePrefix(p)
		if err != nil {
			return nil, errors.New("api-gateway: invalid TRUSTED_PROXY_CIDRS entry: " + p)
		}
		out = append(out, pr)
	}
	return out, nil
}

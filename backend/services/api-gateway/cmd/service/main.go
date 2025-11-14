package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"net/http/httputil"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := getenv("GATEWAY_PORT", "8080")
	authURL := getenv("AUTH_URL", "http://localhost:3001")
	tplURL := getenv("PLAYLIST_URL", "http://localhost:3002")
	voteURL := getenv("VOTE_URL", "http://localhost:3003")
	userURL := getenv("USER_URL", "http://localhost:3005")
	mockURL := getenv("MOCK_URL", "http://localhost:3006")

	// это chi-роутер, логгер, CORS-middlewar
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(cors)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","service":"api-gateway"}`))
	})

	// Swagger
	openapiPath := getenv("OPENAPI_FILE", "./backend/services/api-gateway/openapi.yaml")
	r.Get("/docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		// fmt.Println("saluut")
		w.Header().Set("Content-Type", "application/yaml")
		http.ServeFile(w, r, openapiPath)
	})

	r.Mount("/auth", proxy(authURL))
	r.Mount("/playlists", proxy(tplURL))
	r.Mount("/events", proxy(voteURL))
	r.Mount("/users", proxy(userURL))
	r.Mount("/mock", proxy(mockURL))

	log.Printf("api-gateway on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func proxy(target string) http.Handler {
	u, err := url.Parse(target)
	if err != nil {
		log.Fatalf("invalid proxy target %q: %v", target, err)
	}

	p := httputil.NewSingleHostReverseProxy(u)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = u.Host
		p.ServeHTTP(w, r)
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // все домены могут отправлять запросы, поменять ближе к завершению
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-Id")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		if strings.ToUpper(r.Method) == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

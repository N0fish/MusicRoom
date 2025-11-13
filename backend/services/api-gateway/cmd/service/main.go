package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"net/http/httputil"
)

func main(){
	port := getenv("GATEWAY_PORT","8080")
	authURL := getenv("AUTH_URL","http://localhost:3001")
	tplURL := getenv("PLAYLIST_URL","http://localhost:3002")
	voteURL := getenv("VOTE_URL","http://localhost:3003")
	userURL := getenv("USER_URL","http://localhost:3005")

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(cors)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request){ w.Write([]byte(`{"status":"ok","service":"api-gateway"}`)) })

	r.Mount("/auth", proxy(authURL))
	r.Mount("/playlists", proxy(tplURL))
	r.Mount("/events", proxy(voteURL))
	r.Mount("/users", proxy(userURL))

	log.Printf("api-gateway on :%s", port)
	http.ListenAndServe(":"+port, r)
}

func proxy(target string) http.Handler {
	u, _ := url.Parse(target)
	p := httputil.NewSingleHostReverseProxy(u)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		r.Host = u.Host
		p.ServeHTTP(w, r)
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		w.Header().Set("Access-Control-Allow-Origin","*")
		w.Header().Set("Access-Control-Allow-Headers","Content-Type, Authorization, X-User-Id")
		w.Header().Set("Access-Control-Allow-Methods","GET,POST,PUT,PATCH,DELETE,OPTIONS")
		if strings.ToUpper(r.Method) == "OPTIONS" { w.WriteHeader(204); return }
		next.ServeHTTP(w,r)
	})
}

func getenv(k, def string) string { if v:=os.Getenv(k); v!="" { return v }; return def }

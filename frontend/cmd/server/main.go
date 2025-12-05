package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.gohtml
var tplFS embed.FS

//go:embed all:static
var staticFS embed.FS

type App struct {
	API string
	WS  string
}

func main() {
	api := getenv("API_URL", "http://localhost:8080")
	ws := getenv("WS_URL", "ws://localhost:3004/ws")
	port := getenv("PORT", "5175")

	app := &App{API: api, WS: ws}

	r := chi.NewRouter()
	r.Get("/", app.page("home.gohtml"))
	r.Get("/auth", app.page("auth.gohtml"))
	r.Get("/auth/callback", app.page("auth.gohtml"))
	r.Get("/playlists", app.page("playlists.gohtml"))
	r.Get("/event", app.page("event.gohtml"))
	r.Get("/realtime", app.page("realtime.gohtml"))
	r.Get("/me", app.page("me.gohtml"))

	// static assets (из embed FS)
	r.Get("/static/*", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/static/")
		b, err := staticFS.ReadFile(path.Join("static", p))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(p, ".js") {
			w.Header().Set("content-type", "application/javascript")
		}
		if strings.HasSuffix(p, ".css") {
			w.Header().Set("content-type", "text/css")
		}
		if strings.HasSuffix(p, ".svg") {
			w.Header().Set("content-type", "image/svg+xml")
		}
		w.Write(b)
	})

	log.Printf("Go frontend on :%s (API=%s, WS=%s)", port, api, ws)
	log.Fatal(http.ListenAndServe(":"+port, r))

	// Это сертификаты для https. Они нужны если мы решим использовать https в качестве общего протокола для сервисов.
	// tlsEnabled := strings.EqualFold(getenv("TLS_ENABLED", "false"), "true")
	// if tlsEnabled {
	// 	certFile := getenv("TLS_CERT_FILE", "../../../certs/cert.pem")
	// 	keyFile := getenv("TLS_KEY_FILE", "../../../certs/key.pem")
	// 	log.Printf("Go frontend on HTTPS :%s (API=%s, WS=%s)", port, api, ws)
	// 	log.Fatal(http.ListenAndServeTLS(":"+port, certFile, keyFile, r))
	// } else {
	// 	log.Printf("Go frontend on HTTP :%s (API=%s, WS=%s)", port, api, ws)
	// 	log.Fatal(http.ListenAndServe(":"+port, r))
	// }
}

func (a *App) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tpl, err := template.ParseFS(tplFS, "templates/base.gohtml", "templates/"+name)
		if err != nil {
			http.Error(w, "template error", 500)
			return
		}

		data := map[string]any{
			"API":  a.API,
			"WS":   a.WS,
			"Path": r.URL.Path,
		}
		if err := tpl.ExecuteTemplate(w, "base", data); err != nil {
			http.Error(w, err.Error(), 500)
		}
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

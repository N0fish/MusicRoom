package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

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

type Playlist struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"ownerId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"isPublic"`
	EditMode    string    `json:"editMode"` // "everyone" | "invited"
	CreatedAt   time.Time `json:"createdAt"`
}

func main() {
	api := getenv("API_URL", "http://localhost:8080")
	ws := getenv("WS_URL", "ws://localhost:3004/ws")
	port := getenv("PORT", "5175")

	app := &App{API: api, WS: ws}

	r := chi.NewRouter()
	r.Get("/", app.page("home.gohtml", nil))
	r.Get("/auth", app.page("auth.gohtml", nil))
	r.Get("/auth/callback", app.page("auth.gohtml", nil))
	r.Get("/playlists", app.playlistsPage())
	r.Get("/event", app.page("event.gohtml", nil))
	r.Get("/realtime", app.page("realtime.gohtml", nil))
	r.Get("/me", app.page("me.gohtml", nil))
	r.Get("/friends", app.page("friends.gohtml", nil))

	r.Get("/static/playlists.js", func(w http.ResponseWriter, r *http.Request) {
		tpl, err := template.ParseFS(staticFS, "static/playlists.js")
		if err != nil {
			http.Error(w, "template error", 500)
			return
		}
		w.Header().Set("content-type", "application/javascript")
		data := map[string]any{
			"API": app.API,
			"WS":  app.WS,
		}
		if err := tpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	// static assets (из embed FS)
	r.Get("/static/*", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/static/")
		if p == "playlists.js" { // handled separately
			http.NotFound(w, r)
			return
		}
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
		if strings.HasSuffix(p, ".png") {
			w.Header().Set("content-type", "image/png")
		}
		w.Write(b)
	})

	log.Printf("Go frontend on :%s (API=%s, WS=%s)", port, api, ws)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func (a *App) playlistsPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// playlists, err := a.getPlaylists()
		// if err != nil {
		// 	log.Printf("Failed to get playlists: %v", err)
		// 	http.Error(w, "Failed to get playlists", 500)
		// 	return
		// }

		data := map[string]any{
			"Playlists": []Playlist{},
		}
		a.page("playlists.gohtml", data)(w, r)
	}
}

func (a *App) getPlaylists() ([]Playlist, error) {
	req, err := http.NewRequest("GET", a.API+"/playlists", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("x-user-id", "user1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var playlists []Playlist
	if err := json.NewDecoder(resp.Body).Decode(&playlists); err != nil {
		return nil, err
	}
	return playlists, nil
}

func (a *App) page(name string, data map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tpl, err := template.ParseFS(tplFS, "templates/base.gohtml", "templates/"+name)
		if err != nil {
			http.Error(w, "template error", 500)
			return
		}

		if data == nil {
			data = map[string]any{}
		}
		data["API"] = a.API
		data["WS"] = a.WS
		data["Path"] = r.URL.Path

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

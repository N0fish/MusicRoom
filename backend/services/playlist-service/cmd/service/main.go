package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Playlist struct {
	ID string `json:"id"`
	OwnerID string `json:"ownerId"`
	Name string `json:"name"`
	Visibility string `json:"visibility"`
	CreatedAt time.Time `json:"createdAt"`
}

type Track struct {
	ID string `json:"id"`
	PlaylistID string `json:"playlistId"`
	Title string `json:"title"`
	Artist string `json:"artist"`
	Position int `json:"position"`
	CreatedAt time.Time `json:"createdAt"`
}

func main() {
	port := getenv("PORT","3002")
	dsn := getenv("DATABASE_URL","postgres://postgres:postgres@localhost:5432/musicroom?sslmode=disable")
	redisURL := getenv("REDIS_URL","redis://localhost:6379")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn); if err!=nil { log.Fatalf("pg: %v", err) }
	defer pool.Close()
	autoMigrate(ctx, pool)

	opt, err := redis.ParseURL(redisURL); if err!=nil { log.Fatalf("redis: %v", err) }
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request){ json.NewEncoder(w).Encode(map[string]any{"status":"ok","service":"playlist-service"}) })

	r.Post("/playlists", func(w http.ResponseWriter, r *http.Request){
		owner := r.Header.Get("x-user-id")
		if owner=="" { owner = "anon" }
		var body struct{ Name string `json:"name"`; Visibility string `json:"visibility"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err!=nil { http.Error(w, err.Error(), 400); return }
		if body.Visibility=="" { body.Visibility = "public" }
		var id string
		err := pool.QueryRow(ctx, `INSERT INTO playlists(owner_id,name,visibility) VALUES($1,$2,$3) RETURNING id`, owner, body.Name, body.Visibility).Scan(&id)
		if err!=nil { http.Error(w, err.Error(), 500); return }
		pl := Playlist{ID:id, OwnerID:owner, Name:body.Name, Visibility:body.Visibility}
		event := map[string]any{"type":"playlist.created","payload":pl}
		b,_ := json.Marshal(event)
		rdb.Publish(ctx, "broadcast", string(b))
		json.NewEncoder(w).Encode(pl)
	})

	r.Get("/playlists/{id}", func(w http.ResponseWriter, r *http.Request){
		id := chi.URLParam(r, "id")
		var pl Playlist
		err := pool.QueryRow(ctx, `SELECT id, owner_id, name, visibility, created_at FROM playlists WHERE id=$1`, id).Scan(&pl.ID,&pl.OwnerID,&pl.Name,&pl.Visibility,&pl.CreatedAt)
		if err!=nil { http.Error(w, "not found", 404); return }
		rows, _ := pool.Query(ctx, `SELECT id, playlist_id, title, artist, position, created_at FROM tracks WHERE playlist_id=$1 ORDER BY position ASC`, id)
		defer rows.Close()
		type TrackOut struct{ ID, PlaylistID, Title, Artist string; Position int; CreatedAt time.Time }
		var tracks []TrackOut
		for rows.Next() {
			var t TrackOut
			rows.Scan(&t.ID,&t.PlaylistID,&t.Title,&t.Artist,&t.Position,&t.CreatedAt)
			tracks = append(tracks, t)
		}
		json.NewEncoder(w).Encode(map[string]any{"playlist":pl,"tracks":tracks})
	})

	r.Post("/playlists/{id}/tracks", func(w http.ResponseWriter, r *http.Request){
		pid := chi.URLParam(r, "id")
		var body struct{ Title, Artist string }
		if err := json.NewDecoder(r.Body).Decode(&body); err!=nil { http.Error(w, err.Error(), 400); return }
		var pos int
		pool.QueryRow(ctx, `SELECT COALESCE(MAX(position)+1,0) FROM tracks WHERE playlist_id=$1`, pid).Scan(&pos)
		var tid string
		err := pool.QueryRow(ctx, `INSERT INTO tracks(playlist_id,title,artist,position) VALUES($1,$2,$3,$4) RETURNING id`, pid, body.Title, body.Artist, pos).Scan(&tid)
		if err!=nil { http.Error(w, err.Error(), 500); return }
		track := Track{ID:tid, PlaylistID:pid, Title:body.Title, Artist:body.Artist, Position:pos}
		event := map[string]any{"type":"track.added","payload":map[string]any{"playlistId":pid,"track":track}}
		b,_ := json.Marshal(event)
		rdb.Publish(ctx, "broadcast", string(b))
		json.NewEncoder(w).Encode(track)
	})

	log.Printf("playlist-service on :%s", port)
	http.ListenAndServe(":"+port, r)
}

func autoMigrate(ctx context.Context, pool *pgxpool.Pool){
	pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`)
	pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS playlists(
		id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
		owner_id TEXT NOT NULL,
		name TEXT NOT NULL,
		visibility TEXT NOT NULL DEFAULT 'public',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS tracks(
		id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
		playlist_id uuid NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
		title TEXT NOT NULL,
		artist TEXT NOT NULL,
		position INT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
}

func getenv(k, def string) string { if v:=os.Getenv(k); v!="" { return v }; return def }

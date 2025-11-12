package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main(){
	port := getenv("PORT","3003")
	dsn := getenv("DATABASE_URL","postgres://postgres:postgres@localhost:5432/musicroom?sslmode=disable")
	redisURL := getenv("REDIS_URL","redis://localhost:6379")
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn); if err!=nil { log.Fatalf("pg: %v", err) }
	defer pool.Close()
	autoMigrate(ctx, pool)

	opt, err := redis.ParseURL(redisURL); if err!=nil { log.Fatalf("redis: %v", err) }
	rdb := redis.NewClient(opt); defer rdb.Close()

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request){ json.NewEncoder(w).Encode(map[string]any{"status":"ok","service":"vote-service"}) })

	r.Post("/events", func(w http.ResponseWriter, r *http.Request){
		var body struct{ Name, Visibility string }
		if err := json.NewDecoder(r.Body).Decode(&body); err!=nil { http.Error(w, err.Error(), 400); return }
		if body.Visibility=="" { body.Visibility = "public" }
		var id string
		err := pool.QueryRow(ctx, `INSERT INTO events(name,visibility) VALUES($1,$2) RETURNING id`, body.Name, body.Visibility).Scan(&id)
		if err!=nil { http.Error(w, err.Error(), 500); return }
		evt := map[string]any{"type":"event.created","payload":map[string]any{"id":id,"name":body.Name}}
		b,_ := json.Marshal(evt); rdb.Publish(ctx, "broadcast", string(b))
		json.NewEncoder(w).Encode(map[string]string{"id":id,"name":body.Name,"visibility":body.Visibility})
	})

	r.Post("/events/{id}/votes", func(w http.ResponseWriter, r *http.Request){
		id := chi.URLParam(r, "id")
		var body struct{ Track, VoterId string }
		if err := json.NewDecoder(r.Body).Decode(&body); err!=nil { http.Error(w, err.Error(), 400); return }
		_, err := pool.Exec(ctx, `INSERT INTO votes(event_id, track, voter_id) VALUES($1,$2,$3)`, id, body.Track, body.VoterId)
		if err!=nil { http.Error(w, "duplicate vote", 409); return }
		evt := map[string]any{"type":"vote.cast","payload":map[string]any{"eventId":id,"track":body.Track,"voterId":body.VoterId}}
		b,_ := json.Marshal(evt); rdb.Publish(ctx, "broadcast", string(b))
		w.WriteHeader(201); w.Write([]byte(`{"ok":true}`))
	})

	r.Get("/events/{id}/tally", func(w http.ResponseWriter, r *http.Request){
		id := chi.URLParam(r, "id")
		rows, err := pool.Query(ctx, `SELECT track, COUNT(*) FROM votes WHERE event_id=$1 GROUP BY track ORDER BY COUNT(*) DESC`, id)
		if err!=nil { http.Error(w, err.Error(), 500); return }
		defer rows.Close()
		var out []map[string]any
		for rows.Next(){
			var track string; var count int
			rows.Scan(&track, &count)
			out = append(out, map[string]any{"track":track, "count":count})
		}
		json.NewEncoder(w).Encode(out)
	})

	log.Printf("vote-service on :%s", port)
	http.ListenAndServe(":"+port, r)
}

func autoMigrate(ctx context.Context, pool *pgxpool.Pool){
	pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`)
	pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS events(
		id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
		name TEXT NOT NULL,
		visibility TEXT NOT NULL DEFAULT 'public'
	)`)
	pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS votes(
		id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
		event_id uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		track TEXT NOT NULL,
		voter_id TEXT NOT NULL,
		UNIQUE(event_id, voter_id, track)
	)`)
}

func getenv(k, def string) string { if v:=os.Getenv(k); v!="" { return v }; return def }

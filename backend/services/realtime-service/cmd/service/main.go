package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{ CheckOrigin: func(r *http.Request) bool { return true } }

func main(){
	port := getenv("PORT","3004")
	redisURL := getenv("REDIS_URL","redis://localhost:6379")
	ctx := context.Background()

	opt, err := redis.ParseURL(redisURL); if err!=nil { log.Fatalf("redis: %v", err) }
	rdb := redis.NewClient(opt); defer rdb.Close()
	sub := rdb.Subscribe(ctx, "broadcast"); defer sub.Close()

	clients := make(map[*websocket.Conn]bool)
	broadcast := func(msg []byte){
		for c := range clients { c.WriteMessage(websocket.TextMessage, msg) }
	}

	go func(){
		ch := sub.Channel()
		for m := range ch { broadcast([]byte(m.Payload)) }
	}()

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request){ json.NewEncoder(w).Encode(map[string]any{"status":"ok","service":"realtime-service"}) })

	r.Get("/ws", func(w http.ResponseWriter, r *http.Request){
		conn, err := upgrader.Upgrade(w, r, nil)
		if err!=nil { return }
		clients[conn] = true
		conn.WriteJSON(map[string]any{"type":"welcome","now":0})
		defer func(){ delete(clients, conn); conn.Close() }()
		for { if _, _, err := conn.ReadMessage(); err != nil { break } }
	})

	r.Post("/events", func(w http.ResponseWriter, r *http.Request){
		var v any
		json.NewDecoder(r.Body).Decode(&v)
		b,_ := json.Marshal(v)
		rdb.Publish(ctx, "broadcast", string(b))
		w.Write([]byte(`{"ok":true}`))
	})

	log.Printf("realtime-service on :%s", port)
	http.ListenAndServe(":"+port, r)
}

func getenv(k, def string) string { if v:=os.Getenv(k); v!="" { return v }; return def }

package realtime

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	// В проде стоит ограничивать origin, но за gateway'ом это ок ??????
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	hub *Hub
	rdb *redis.Client
	ctx context.Context
}

func NewServer(hub *Hub, rdb *redis.Client, ctx context.Context) *Server {
	return &Server{
		hub: hub,
		rdb: rdb,
		ctx: ctx,
	}
}

// Router создаёт chi.Router с нашими маршрутами.
func (s *Server) Router(middlewares ...func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()

	for _, mw := range middlewares {
		r.Use(mw)
	}

	r.Get("/health", s.handleHealth)
	r.Get("/ws", s.handleWS)
	r.Post("/events", s.handleEvents)

	return r
}

// RunRedisSubscriber подписывается на канал "broadcast" и шлёт сообщения в hub.
func (s *Server) RunRedisSubscriber() {
	sub := s.rdb.Subscribe(s.ctx, "broadcast")
	defer sub.Close()

	ch := sub.Channel()
	for msg := range ch {
		s.hub.broadcast <- []byte(msg.Payload)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "realtime-service",
	})
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("realtime-service: ws upgrade: %v", err)
		return
	}

	client := &Client{
		hub:  s.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	s.hub.register <- client

	welcome := map[string]any{
		"type": "welcome",
		"now":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if b, err := json.Marshal(welcome); err == nil {
		client.send <- b
	}

	// Запускаем две горутины: читаем и пишем.
	go client.writePump()
	go client.readPump()
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	if err := s.rdb.Publish(s.ctx, "broadcast", string(data)).Err(); err != nil {
		http.Error(w, "redis error", http.StatusInternalServerError)
		log.Printf("realtime-service: publish error: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

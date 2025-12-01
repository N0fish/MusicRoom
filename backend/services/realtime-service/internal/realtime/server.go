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
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	hub            *Hub
	rdb            *redis.Client
	ctx            context.Context
	frontendOrigin string
}

func NewServer(hub *Hub, rdb *redis.Client, ctx context.Context, frontendOrigin string) *Server {
	return &Server{
		hub:            hub,
		rdb:            rdb,
		ctx:            ctx,
		frontendOrigin: frontendOrigin,
	}
}

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
	if !websocket.IsWebSocketUpgrade(r) {
		writeError(w, http.StatusBadRequest, "websocket upgrade required")
		return
	}

	if s.frontendOrigin != "" {
		origin := r.Header.Get("Origin")
		if origin != "" && origin != s.frontendOrigin {
			log.Printf("realtime-service: forbidden origin %q (allowed %q)", origin, s.frontendOrigin)
			writeError(w, http.StatusForbidden, "origin not allowed")
			return
		}
	}

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

	go client.writePump()
	go client.readPump()
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode error")
		return
	}

	if err := s.rdb.Publish(s.ctx, "broadcast", string(data)).Err(); err != nil {
		log.Printf("realtime-service: publish error: %v", err)
		writeError(w, http.StatusInternalServerError, "redis error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

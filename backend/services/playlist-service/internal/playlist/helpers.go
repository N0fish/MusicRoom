package playlist

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func (s *Server) getPlaylistAccessInfo(ctx context.Context, playlistID string) (ownerID string, isPublic bool, editMode string, err error) {
	err = s.db.QueryRow(ctx, `
		SELECT owner_id, is_public, edit_mode
		FROM playlists
		WHERE id = $1
	`, playlistID).Scan(&ownerID, &isPublic, &editMode)
	return
}

func (s *Server) userIsInvited(ctx context.Context, playlistID, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	var uid string
	err := s.db.QueryRow(ctx, `
		SELECT user_id
		FROM playlist_members
		WHERE playlist_id = $1 AND user_id = $2
	`, playlistID, userID).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Server) publishEvent(ctx context.Context, event map[string]any) {
	if s.rdb == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("playlist-service: marshal event: %v", err)
		return
	}
	if err := s.rdb.Publish(ctx, "broadcast", string(data)).Err(); err != nil {
		log.Printf("playlist-service: publish event: %v", err)
	}
}

type DB = pgxpool.Pool
type RedisClient = redis.Client

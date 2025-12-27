package playlist

import (
	"context"
	"log"
	"time"
)

// StartTicker starts a background worker that checks for finished tracks
// and advances them automatically.
func (s *Server) StartTicker(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				s.checkAndAdvanceTracks(ctx)
			}
		}
	}()
}

func (s *Server) checkAndAdvanceTracks(ctx context.Context) {
	// Find playlists where current track has finished
	// playing_started_at + duration < now
	rows, err := s.db.Query(ctx, `
		SELECT p.id
		FROM playlists p
		JOIN tracks t ON t.id = p.current_track_id
		WHERE p.current_track_id IS NOT NULL 
		  AND p.playing_started_at IS NOT NULL
		  AND t.status = 'playing'
		  AND t.duration_ms > 0
		  AND (p.playing_started_at + (t.duration_ms * interval '1 millisecond')) < NOW()
	`)
	if err != nil {
		log.Printf("playlist-service: ticker query error: %v", err)
		return
	}
	defer rows.Close()

	var playlistIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			log.Printf("playlist-service: ticker scan error: %v", err)
			continue
		}
		playlistIDs = append(playlistIDs, id)
	}

	for _, id := range playlistIDs {
		log.Printf("playlist-service: ticker advancing playlist %s", id)
		if _, err := s.NextTrack(ctx, id); err != nil {
			log.Printf("playlist-service: ticker advance error for %s: %v", id, err)
		}
	}
}

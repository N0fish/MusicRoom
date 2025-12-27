package vote

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var digits [20]byte
	pos := len(digits)
	for i > 0 {
		pos--
		digits[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		digits[pos] = '-'
	}
	return string(digits[pos:])
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
}

func writeVoteError(w http.ResponseWriter, err error) {
	var ve *voteError
	if errors.As(err, &ve) {
		writeError(w, ve.status, ve.msg)
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *HTTPServer) publishEvent(ctx context.Context, eventType string, payload any) {
	if s.rdb == nil {
		return
	}

	body := map[string]any{
		"type":    eventType,
		"payload": payload,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return
	}

	if err := s.rdb.Publish(ctx, "broadcast", string(data)).Err(); err != nil {
		// Log error preferably
	}
}

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
)

type UserProfile struct {
	ID          string         `json:"id"`
	Email       string         `json:"email"`
	DisplayName string         `json:"displayName"`
	Bio         string         `json:"bio"`
	Visibility  string         `json:"visibility"`
	Preferences map[string]any `json:"preferences"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

func main() {
	port := getenv("PORT", "3005")
	dsn := getenv("DATABASE_URL", "postgres://musicroom:musicroom@localhost:5432/musicroom?sslmode=disable")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	autoMigrate(ctx, pool)

	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "service": "user-service"})
	})

	// Current user profile by X-User-Id header (будет заменено на JWT later)
	r.Get("/users/me", func(w http.ResponseWriter, r *http.Request) {
		userID := currentUserID(r)
		if userID == "" {
			http.Error(w, "missing user id", http.StatusUnauthorized)
			return
		}
		profile, err := loadProfile(ctx, pool, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(profile)
	})

	// Update current user profile
	r.Put("/users/me/profile", func(w http.ResponseWriter, r *http.Request) {
		userID := currentUserID(r)
		if userID == "" {
			http.Error(w, "missing user id", http.StatusUnauthorized)
			return
		}

		var body struct {
			DisplayName *string                `json:"displayName"`
			Bio         *string                `json:"bio"`
			Visibility  *string                `json:"visibility"`
			Preferences map[string]interface{} `json:"preferences"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		profile, err := upsertProfile(ctx, pool, userID, body.DisplayName, body.Bio, body.Visibility, body.Preferences)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(profile)
	})

	// Public profile by id
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		profile, err := loadProfile(ctx, pool, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// TODO: friends visibility. Пока: public -> видно всем, friends/private -> только владельцу.
		if profile.Visibility != "public" {
			current := currentUserID(r)
			if current == "" || current != profile.ID {
				http.Error(w, "profile is not public", http.StatusForbidden)
				return
			}
		}

		json.NewEncoder(w).Encode(profile)
	})

	log.Printf("user-service on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func currentUserID(r *http.Request) string {
	// Пока используем заголовок, как и другие сервисы.
	uid := r.Header.Get("X-User-Id")
	if uid == "" {
		uid = r.Header.Get("x-user-id")
	}
	return uid
}

func autoMigrate(ctx context.Context, pool *pgxpool.Pool) {
	// Расширения могут уже существовать (создаёт auth-service), ошибки логируем, но не падаем.
	if _, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`); err != nil {
		log.Printf("extension pgcrypto: %v", err)
	}

	_, err := pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS user_profiles (
	user_id     uuid PRIMARY KEY REFERENCES auth_users(id) ON DELETE CASCADE,
	display_name text,
	bio          text,
	visibility   text NOT NULL DEFAULT 'public',
	preferences  jsonb NOT NULL DEFAULT '{}'::jsonb,
	created_at   timestamptz NOT NULL DEFAULT now(),
	updated_at   timestamptz NOT NULL DEFAULT now()
)`)
	if err != nil {
		log.Printf("migrate user_profiles: %v", err)
	}
}

// avatar_url   text,

func loadProfile(ctx context.Context, pool *pgxpool.Pool, userID string) (*UserProfile, error) {
	const q = `
SELECT
  u.id,
  u.email,
  COALESCE(p.display_name, ''),
  COALESCE(p.bio, ''),
  COALESCE(p.visibility, 'public'),
  COALESCE(p.preferences, '{}'::jsonb),
  COALESCE(p.created_at, u.created_at),
  COALESCE(p.updated_at, u.created_at)
FROM auth_users u
LEFT JOIN user_profiles p ON p.user_id = u.id
WHERE u.id = $1
`
	row := pool.QueryRow(ctx, q, userID)

	var (
		id, email, displayName, bio, visibility string
		prefsJSON                               []byte
		createdAt, updatedAt                    time.Time
	)
	if err := row.Scan(&id, &email, &displayName, &bio, &visibility, &prefsJSON, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	var prefs map[string]any
	if err := json.Unmarshal(prefsJSON, &prefs); err != nil {
		prefs = map[string]any{}
	}

	return &UserProfile{
		ID:          id,
		Email:       email,
		DisplayName: displayName,
		Bio:         bio,
		Visibility:  visibility,
		Preferences: prefs,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func upsertProfile(
	ctx context.Context,
	pool *pgxpool.Pool,
	userID string,
	displayName *string,
	bio *string,
	visibility *string,
	preferences map[string]interface{},
) (*UserProfile, error) {
	// Загружаем текущий профиль (или создаём "пустой" только из auth_users)
	profile, err := loadProfile(ctx, pool, userID)
	if err != nil {
		return nil, err
	}

	if displayName != nil {
		profile.DisplayName = *displayName
	}
	if bio != nil {
		profile.Bio = *bio
	}
	if visibility != nil && *visibility != "" {
		profile.Visibility = *visibility
	}
	if preferences != nil {
		// Полностью заменяем preferences. При желании можно сделать merge.
		profile.Preferences = preferences
	}

	prefsJSON, _ := json.Marshal(profile.Preferences)

	const q = `
INSERT INTO user_profiles(user_id, display_name, bio, visibility, preferences)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id)
DO UPDATE SET
  display_name = EXCLUDED.display_name,
  bio          = EXCLUDED.bio,
  visibility   = EXCLUDED.visibility,
  preferences  = EXCLUDED.preferences,
  updated_at   = now()
`
	if _, err := pool.Exec(ctx, q, userID, profile.DisplayName, profile.Bio, profile.Visibility, prefsJSON); err != nil {
		return nil, err
	}

	// Обновляем timestamps после апдейта
	profile.UpdatedAt = time.Now().UTC()

	return profile, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

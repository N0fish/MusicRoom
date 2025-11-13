package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID string `json:"id"`
	Email string `json:"email"`
	Password string `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

type Credentials struct {
	Email string `json:"email"`
	Password string `json:"password"`
}

func main() {
	port := getenv("PORT", "3001")
	dsn := getenv("DATABASE_URL", "postgres://musicroom:musicroom@postgres:5432/musicroom?sslmode=disable")
	jwtSecret := getenv("JWT_SECRET", "supersecretdev")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil { log.Fatalf("pg connect: %v", err) }
	defer pool.Close()

	autoMigrate(ctx, pool)

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(map[string]any{"status":"ok","service":"auth-service"}) })

	r.Post("/auth/signup", func(w http.ResponseWriter, r *http.Request) {
		var c Credentials
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if len(c.Password) < 6 || len(c.Email) < 3 {
			http.Error(w, "invalid credentials", 400)
			return
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)

		var id string
		err := pool.QueryRow(ctx,
			`INSERT INTO auth_users(email,password)
					VALUES($1,$2)
					ON CONFLICT(email) DO NOTHING
					RETURNING id`,
			c.Email, string(hash),
		).Scan(&id)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "email already registered", 409)
				return
			}
			http.Error(w, err.Error(), 500)
			return
		}

		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"email": c.Email,
			"id":    id,
			"exp":   time.Now().Add(24 * time.Hour).Unix(),
		})
		signed, _ := tok.SignedString([]byte(jwtSecret))

		json.NewEncoder(w).Encode(map[string]any{"token": signed})
	})

	r.Post("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var c Credentials
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil { http.Error(w, err.Error(), 400); return }
		var id, hash string
		err := pool.QueryRow(ctx, `SELECT id, password FROM auth_users WHERE email=$1`, c.Email).Scan(&id, &hash)
		if err != nil { http.Error(w, "invalid credentials", 401); return }
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) != nil { http.Error(w, "invalid credentials", 401); return }
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"email": c.Email, "id": id, "exp": time.Now().Add(24*time.Hour).Unix()})
		signed, _ := tok.SignedString([]byte(jwtSecret))
		json.NewEncoder(w).Encode(map[string]any{"token": signed})
	})

	log.Printf("auth-service on :%s", port)
	http.ListenAndServe(":"+port, r)
}

func autoMigrate(ctx context.Context, pool *pgxpool.Pool) {
	_, err := pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto`)
	if err != nil { log.Printf("extension: %v", err) }
	_, err = pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS auth_users(
		id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
		email TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil { log.Printf("migrate: %v", err) }
}

func getenv(k, def string) string { if v:=os.Getenv(k); v!="" { return v }; return def }

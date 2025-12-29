#!/usr/bin/env bash
set -euo pipefail

# Path to the project root
ROOT_DIR="$(cd "$(dirname "$0")/.."; pwd)"

echo "Project root: $ROOT_DIR"
echo "⚙️  Generating .env files..."


# ENV for docker-compose (root)
cat > "${ROOT_DIR}/.env" <<EOF
POSTGRES_USER=musicroom
POSTGRES_PASSWORD=musicroom
POSTGRES_DB=musicroom
EOF

echo "✅ Created .env (root)"

# env for each service
JWT_SECRET="supersecretdev"
DB_URL="postgres://musicroom:musicroom@postgres:5432/musicroom?sslmode=disable"
REDIS_URL="redis://redis:6379"
YOUTUBE_API_KEY="AIzaSyBeDpLXg_R02KCnUEJlO7_-c26RCFIJj_Q"

FRONTEND_PORT=5175
GATEWAY_PORT=8080

LOCAL_IP="${LOCAL_IP:-localhost}"


# List of services
SERVICES=(
  "backend/services/api-gateway"
  "backend/services/auth-service"
  "backend/services/user-service"
  "backend/services/playlist-service"
  "backend/services/realtime-service"
  "backend/services/vote-service"
  "backend/services/mock-service"
  "backend/services/music-provider-service"
  "frontend"
)

for svc in "${SERVICES[@]}"; do
  SVC_PATH="${ROOT_DIR}/${svc}"
  mkdir -p "$SVC_PATH"

  ENV_FILE="${SVC_PATH}/.env"

  if [[ -f "$ENV_FILE" ]]; then
    echo "➡️  Skipped (exists): $svc/.env"
    continue
  fi

  case "$svc" in

  # API GATEWAY
  "backend/services/api-gateway")
    cat > "$ENV_FILE" <<EENV
PORT=${GATEWAY_PORT}
OPENAPI_FILE=openapi.yaml

# Internal services URLs (inside docker-compose network)
AUTH_SERVICE_URL=http://auth-service:3001
USER_SERVICE_URL=http://user-service:3005
PLAYLIST_SERVICE_URL=http://playlist-service:3002
VOTE_SERVICE_URL=http://vote-service:3003
MOCK_SERVICE_URL=http://mock-service:3006
REALTIME_SERVICE_URL=http://realtime-service:3004
MUSIC_PROVIDER_SERVICE_URL=http://music-provider-service:3007
FRONTEND_BASE_URL=http://localhost:5175

# JWT secret must be the same as in auth-service
JWT_SECRET=${JWT_SECRET}

# Simple per-IP rate limit in requests per second
RATE_LIMIT_RPS=20

# CORS
CORS_ALLOWED_ORIGIN=*
# CORS_ALLOWED_ORIGIN=http://localhost:5175

# limit
USER_PATCH_BODY_LIMIT=4096
USER_PATCH_RPS=5
AVATAR_UPLOAD_RPS=5
FRIEND_REQUEST_RPS=5
AUTHED_RPS=30
PLAYLIST_AUTHED_RPS=30
VOTE_AUTHED_RPS=30
MUSIC_AUTHED_RPS=30

# CERTS
# TLS_ENABLED=true
# TLS_CERT_FILE=/certs/cert.pem
# TLS_KEY_FILE=/certs/key.pem

TRUSTED_PROXY_CIDRS=127.0.0.1/32,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
# TRUSTED_PROXY_CIDRS=10.0.0.0/8 // для теста на localhost
EENV
      ;;

    # AUTH SERVICE
    "backend/services/auth-service")
      cat > "$ENV_FILE" <<EENV
PORT=3001
DATABASE_URL=${DB_URL}

# JWT settings
JWT_SECRET=${JWT_SECRET}
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h

# OAuth2 Google (заполни своими значениями)
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=http://localhost:${GATEWAY_PORT}/auth/google/callback
# Пока localhost, в дальнейшем при https заменить на LOCAL_IP в скрипте

# OAuth2 42 (Intra) (заполни своими значениями)
FT_CLIENT_ID=
FT_CLIENT_SECRET=
FT_REDIRECT_URL=http://localhost:${GATEWAY_PORT}/auth/42/callback

# Frontend URL used after OAuth callbacks and email flows
OAUTH_FRONTEND_REDIRECT=http://localhost:${FRONTEND_PORT}/auth/callback
FRONTEND_BASE_URL=http://localhost:${FRONTEND_PORT}

EMAIL_VERIFICATION_URL=http://localhost:${GATEWAY_PORT}/auth/verify-email
PASSWORD_RESET_URL=http://localhost:${GATEWAY_PORT}/auth/reset-password

# SMTP (если не заполнено — сервис использует LogEmailSender)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=
SMTP_PASS=
SMTP_FROM="Your Service MusicRoom"
EENV
      ;;

    # USER SERVICE
    "backend/services/user-service")
      cat > "$ENV_FILE" <<EENV
PORT=3005
DATABASE_URL=${DB_URL}
AVATAR_DIR=./avatars
DEFAULT_AVATAR_URL=/avatars/default.png
EENV
      ;;

    # PLAYLIST SERVICE
    "backend/services/playlist-service")
      cat > "$ENV_FILE" <<EENV
PORT=3002
DATABASE_URL=${DB_URL}
REDIS_URL=${REDIS_URL}
EENV
      ;;

    # VOTE SERVICE
    "backend/services/vote-service")
      cat > "$ENV_FILE" <<EENV
PORT=3003
DATABASE_URL=${DB_URL}
REDIS_URL=${REDIS_URL}
USER_SERVICE_URL=http://user-service:3005
PLAYLIST_SERVICE_URL=http://playlist-service:3002
EENV
      ;;

    # REALTIME SERVICE
    "backend/services/realtime-service")
      cat > "$ENV_FILE" <<EENV
PORT=3004
REDIS_URL=${REDIS_URL}
FRONTEND_BASE_URL=http://localhost:${FRONTEND_PORT}
EENV
      ;;

    # MUSIC PROVIDER SERVICE
    "backend/services/music-provider-service")
      cat > "$ENV_FILE" <<EENV
PORT=3007
YOUTUBE_API_KEY=${YOUTUBE_API_KEY}
REDIS_URL=${REDIS_URL}
YOUTUBE_SEARCH_URL=https://www.googleapis.com/youtube/v3/search
EENV
      ;;

    # MOCK SERVICE
    "backend/services/mock-service")
      cat > "$ENV_FILE" <<EENV
PORT=3006
EENV
      ;;

    # FRONTEND
    "frontend")
      cat > "$ENV_FILE" <<EENV
# API / WS
API_URL=http://${LOCAL_IP}:${GATEWAY_PORT}
WS_URL=ws://${LOCAL_IP}:3004/ws
PORT=${FRONTEND_PORT}
EENV
      ;;

  esac

  echo "✅ Created $svc/.env"
done

echo "All .env files created successfully!"

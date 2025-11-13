#!/usr/bin/env bash
# пока не работает
set -euo pipefail
cat > .env.root <<EOF
POSTGRES_USER=musicroom
POSTGRES_PASSWORD=musicroom
POSTGRES_DB=musicroom
JWT_SECRET=supersecretdev
SERVICES := backend/services/api-gateway \
						backend/services/auth-service \
						backend/services/playlist-service \
						backend/services/realtime-service \
						backend/services/vote-service \
						frontend
EOF
for svc in ${SERVICES}; do
  mkdir -p "../$svc"
  if [[ ! -f "../$svc/.env" ]]; then
    cat > "../$svc/.env" <<EENV
PORT=8080
DATABASE_URL=postgres://music:music@postgres:5432/music?sslmode=disable
REDIS_URL=redis://redis:6379
JWT_SECRET=${JWT_SECRET:-supersecretdev}
ENV=dev
EENV
  fi
done
echo "Env files created."

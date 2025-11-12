#!/usr/bin/env bash
set -euo pipefail

echo "[1] Signup"
curl -sS -X POST http://localhost:3001/auth/signup -H 'content-type: application/json'   -d '{"email":"test@example.com","password":"secret123"}' | jq . || true

echo "[2] Login"
TOKEN=$(curl -sS -X POST http://localhost:3001/auth/login -H 'content-type: application/json'   -d '{"email":"test@example.com","password":"secret123"}' | jq -r .token)
echo "TOKEN=${TOKEN}"

echo "[3] Create playlist"
PL=$(curl -sS -X POST http://localhost:3002/playlists -H 'content-type: application/json' -H 'x-user-id: user1'   -d '{"name":"Party","visibility":"public"}')
echo "$PL" | jq .
PLID=$(echo "$PL" | jq -r .id)

echo "[4] Add track"
curl -sS -X POST http://localhost:3002/playlists/${PLID}/tracks -H 'content-type: application/json'   -d '{"title":"Song A","artist":"Artist 1"}' | jq .

echo "[5] Create event"
EV=$(curl -sS -X POST http://localhost:3003/events -H 'content-type: application/json'   -d '{"name":"Friday Night","visibility":"public"}')
echo "$EV" | jq .
EVID=$(echo "$EV" | jq -r .id)

echo "[6] Cast vote"
curl -sS -X POST http://localhost:3003/events/${EVID}/votes -H 'content-type: application/json'   -d '{"track":"Song A","voterId":"user1"}' | jq .

echo "[7] Tally"
curl -sS http://localhost:3003/events/${EVID}/tally | jq .

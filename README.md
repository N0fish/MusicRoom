# MusicRoom — Go Microservices

Microservice architecture of a music application: authorization, playlists, voting, and realtime via WebSocket.

## env + up + url
```bash
make start
```

## Init env
```bash
make env
```
```bash
make up
```

### logs
```bash
make logs
```
```bash
# or
docker compose logs -f auth-service
```
```bash
make down
```
```bash
make down-v
```
```bash
docker compose ps
```
```bash
docker stop <id>
```

## re
```bash
make re
```
## re BD
```bash
make re-v
```

| Service                    | Purpose                 | Port |
|----------------------------|-------------------------|------|
| **api-gateway**            | Single point of entry   | 8080 |
| **auth-service**           | Authorization / JWT     | 3001 |
| **user-service**           | User profile            | 3005 |
| **playlist-service**       | Playlists and tracks    | 3002 |
| **vote-service**           | Events and voting       | 3003 |
| **realtime-service**       | WebSocket notifications | 3004 |
| **mock-service**           | Mock / statistical data | 3006 |
| **music-provider-service** | Provider service        | 3007 |
| **postgres**               | Shared database         | 5432 |
| **redis**                  | Pub/Sub messages        | 6379 |


```http
https://api.intra.42.fr/apidoc
```
```http
https://profile.intra.42.fr/oauth/applications/new
```
```http
https://console.cloud.google.com
```
```http
https://musicroom-4k3a.onrender.com/health
```
```http
https://musicroom-frontend.onrender.com
```
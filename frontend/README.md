# MusicRoom Go Frontend (SSR + HTMX)

Сервер на Go (chi + html/template), без Node. Использует:
- API Gateway: `http://localhost:8080`
- Realtime WS: `ws://localhost:3004/ws`
- Tailwind и HTMX через CDN.

## Запуск
```bash
go run ./cmd/server
# или
make run
```
Открой: http://localhost:5175

## Переменные окружения
- `PORT` (по умолчанию 5175)
- `API_URL` (по умолчанию `http://localhost:8080`)
- `WS_URL` (по умолчанию `ws://localhost:3004/ws`)

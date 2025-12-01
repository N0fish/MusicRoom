# Realtime API (WebSocket)

Этот документ описывает, как фронтенд и мобильное приложение должны
подключаться к **realtime-service** и какие события ожидать.

Realtime-service:
- принимает события от других микросервисов через Redis (`channel: "broadcast"`),
- рассылает их всем подключённым WebSocket-клиентам,
- сам **ничего не принимает** от клиентов (клиентские сообщения игнорируются).

---

## Подключение

### URL для DEV
docker-compose, локальный запуск:
```text
ws://localhost:3004/ws
```
WS_URL=ws://localhost:3004/ws

---

hub.go - управление клиентами
client.go - readPump/writePump + WebSocket клиент
server.go - Router, WS endpoint, /events, /health, Redis подписка
helpers.go - writeJSON и мелкие утилиты

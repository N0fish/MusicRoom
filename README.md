# MusicRoom — Go Microservices

<!-- Instruction for agents: Please refer to Agents/AGENTS.md for project-wide instructions. -->

Микросервисная архитектура музыкального приложения: авторизация, плейлисты, голосование и realtime через WebSocket.

## Запуск
```bash
# Поднять всё в Docker
make up
```

### Команды
```bash
make logs  # все логи
```
```bash
make logs  # логи
# or
docker compose logs -f auth-service  # логи конкретного сервиса
```
```bash
make down  # остановить и очистить тома
```

docker compose ps  # проверить состояние
```
```bash
docker stop <id> # если есть незакрытые порты
```

Все сервисы должны быть в состоянии **Up**:

| Сервис            | Назначение             | Порт |
|--------------------|------------------------|------|
| **api-gateway**    | Единая точка входа     | 8080 |
| **auth-service**   | Авторизация / JWT      | 3001 |
| **user-service**    | Профиль пользователя  | 3005 |
| **playlist-service** | Плейлисты и треки    | 3002 |
| **vote-service**   | События и голосование  | 3003 |
| **realtime-service** | WebSocket уведомления | 3004 |
| **postgres**       | Общая база данных      | 5432 |
| **redis**          | Pub/Sub сообщения      | 6379 |

- API Gateway: http://localhost:8080
- Auth: http://localhost:3001
- User: http://localhost:3005
- Playlist: http://localhost:3002
- Vote: http://localhost:3003
- Realtime WS: ws://localhost:3004/ws
- Postgres: localhost:5432 (user: postgres / password: postgres, db: musicroom)
- Redis: localhost:6379

---

## Архитектура

```sql
                 ┌────────────┐
     requests →  │ API Gateway│ ← фронт / мобилка
                 └────┬───────┘
                      │
        ┌─────────────┼──────────────┬───────────────┐
        ↓             ↓              ↓               ↓
 ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐
 │auth-service│ │user-service│ │playlist-s. │ │vote-service│
 └────┬───────┘ └────┬───────┘ └─────┬──────┘ └──────┬─────┘
      │              │               │               │
      │     SQL      │               │               │
      └──→ Postgres ←────────────────┘               │
             ↑                                       │
             │             SQL                       │
             │              ↑                        │
             │              │                        │
             └──────→ Redis pub/sub ←────────────────┘
                         │
                   ┌───────────────┐
                   │realtime-s. WS │
                   └───────────────┘
```
`Postgres` - каждый сервис может делать SQL-запросы.

```sql
         ┌─────────────────────────────────┐
         │         frontend                │
         │  HTTP → API Gateway             │
         │  WS   → realtime-service        │
         └───────────────┬─────────────────┘
                         │
                         │ HTTP
                         ▼
           ┌─────────────────────────────┐
requests → │         API Gateway   :8080 │ ← фронт / мобилка
           │ /auth      → auth-service   │  
           │ /playlists → playlist-s.    │
           │ /events    → vote-service   │
           │ /users     → user-service   │
           └────────────┬────────────────┘
                        │
     ┌──────────────────┼─────────────────────────────┐
     ↓                  ↓                             ↓
  ┌────────────┐   ┌────────────┐               ┌────────────┐
  │auth-service│   │playlist-s. │               │vote-service│
  │      :3001 │   │      :3002 │               │      :3003 │
  └────────────┘   └────────────┘               └────────────┘
        ↓                ↓                             ↓
        ┌────────────────┴───────────────┐             ↓
        │                                │             ↓
        ↓                                ↓             ↓
  ┌────────────┐                    ┌────────────┐  ┌────────────┐
  │user-service│                    │realtime-s. │  │   Redis    │
  │      :3005 │                    │      :3004 │  │      :6379 │
  └────────────┘                    └────────────┘  └────────────┘
         ↓
         │ (SQL)
         ↓
   ┌───────────┐
   │ Postgres  │ :5432 
   └───────────┘
```

---

## Сервисы:
### 1. Auth
### `auth-service` — Авторизация (порт 3001)
- `/auth/signup` — регистрация пользователя
- `/auth/login` — вход, возвращает JWT
- Использует PostgreSQL (`auth_users`)

- Пример:
#### регистрация
```bash
curl -X POST http://localhost:3001/auth/signup   -H 'content-type: application/json'   -d '{"email":"test@example.com","password":"secret123"}'
```

#### логин (вернётся JWT, пригодится потом, но сейчас мидлвари не требуют)
```bash
curl -X POST http://localhost:3001/auth/login   -H 'content-type: application/json'   -d '{"email":"test@example.com","password":"secret123"}'
```

---

### 2. User
### `user-service` — Профили пользователей (порт 3005)
- `/users/me` — получить профиль текущего пользователя (по `X-User-Id` / JWT).
- `/users/me/profile` — создать/обновить профиль (displayName, bio, visibility, preferences).
- `/users/{id}` — публичный профиль другого пользователя (с учётом `visibility`).

Профили хранятся в таблице `user_profiles` и связаны с `auth_users` по `user_id`.

#### Через API Gateway
#### Получить свой профиль
```bash
curl http://localhost:3005/users/me \
  -H 'X-User-Id: <USER_ID>'
```

#### Обновить профиль
```bash
curl -X PUT http://localhost:3005/users/me/profile \
  -H 'content-type: application/json' \
  -H 'X-User-Id: <USER_ID>' \
  -d '{
    "displayName": "Alla",
    "bio": "Люблю вкусно кушать",
    "visibility": "public",
    "preferences": { "genres": ["japon", "ramen"] }
  }'
```

#### Посмотреть публичный профиль другого пользователя
```bash
curl http://localhost:3005/users/<OTHER_USER_ID>
```

---

### 3. Playlist
### `playlist-service` — Плейлисты и треки (порт 3002)
- `/playlists` — создать плейлист  
- `/playlists/:id/tracks` — добавить трек  
- Публикует события в Redis (для realtime)

- Пример:
#### Создать плейлист
```bash
curl -X POST http://localhost:3002/playlists   -H 'content-type: application/json' -H 'x-user-id: user1'   -d '{"name":"Party","visibility":"public"}'
```

####  Получить плейлист
```bash
curl http://localhost:3002/playlists/<playlistId>
```

####  Добавить трек
```bash
curl -X POST http://localhost:3002/playlists/<playlistId>/tracks -H 'content-type: application/json'   -d '{"title":"Song A","artist":"Artist 1"}'
```

---

### 3. Vote
### `vote-service` — События и голосование (порт 3003)
- `/events` — создать событие  
- `/events/:id/votes` — проголосовать  
- `/events/:id/tally` — посмотреть результаты  
- Также публикует события в Redis.

#### создать ивент
```bash
curl -X POST http://localhost:3003/events   -H 'content-type: application/json'   -d '{"name":"Friday Night","visibility":"public"}'
```

#### проголосовать
```bash
curl -X POST http://localhost:3003/events/<eventId>/votes -H 'content-type: application/json'   -d '{"track":"Song A","voterId":"user1"}'
```

#### сводка голосов
```bash
curl http://localhost:3003/events/<eventId>/tally
```

---

### 5. Realtime
### `realtime-service` — WebSocket уведомления (порт 3004)
- Клиенты подключаются к `ws://localhost:3004/ws`
- Принимает события от Redis и рассылает в браузеры
- Проверка:
  - открыть `ws.html` в браузере,
  - нажать **Connect**,
  - добавить трек или проголосовать — появится событие `playlist.created` или `vote.cast`.

---

### 6. `api-gateway` — Единая точка входа (порт 8080)
- Проксирует:
  - `/auth` → `auth-service`
  - `/playlists` → `playlist-service`
  - `/events` → `vote-service`
- Можно обращаться **только к `localhost:8080`**, не к внутренним сервисам.

Пример:
#### создать плейлист
```bash
curl -X POST http://localhost:8080/playlists   -H 'content-type: application/json' -H 'x-user-id: user1'   -d '{"name":"Party","visibility":"public"}'
```

---

## Последовательность тестирования

1. Проверить, что всё поднялось:
   ```bash
   docker compose ps
   ```

2. Проверить `auth-service`:
   ```bash
   curl http://localhost:3001/health
   ```

3. Зарегистрироваться и залогиниться:
   ```bash
   curl -X POST http://localhost:3001/auth/signup -H 'content-type: application/json' -d '{"email":"test@example.com","password":"secret123"}'
   curl -X POST http://localhost:3001/auth/login -H 'content-type: application/json' -d '{"email":"test@example.com","password":"secret123"}'
   ```

4. Проверить user-service (health):
   ```bash
   curl http://localhost:3005/health
   ```

5. Получить профиль пользователя через шлюз:
   ```bash
   export USER_ID=<USER_ID>

   curl http://localhost:8080/users/me \
   -H "X-User-Id: $USER_ID"
   ```

6. Обновить профиль пользователя:
   ```bash
   curl -X PUT http://localhost:8080/users/me/profile \
   -H "content-type: application/json" \
   -H "X-User-Id: $USER_ID" \
   -d '{
      "displayName": "Alla",
      "bio": "Люблю собачек,
      "visibility": "public",
      "preferences": { "genres": ["dog", "cat"] }
   }'
   ```

7. Создать плейлист через шлюз:
   ```bash
   curl -X POST http://localhost:8080/playlists -H 'content-type: application/json' -H 'x-user-id: user1' -d '{"name":"Party","visibility":"public"}'
   ```

8. Добавить трек:
   ```bash
   curl -X POST http://localhost:8080/playlists/$PLAYLIST_ID/tracks -H 'content-type: application/json' -d '{"title":"Song A","artist":"Artist 1"}'
   ```

9. Создать ивент и проголосовать:
   ```bash
   curl -X POST http://localhost:8080/events -H 'content-type: application/json' -d '{"name":"Friday Night","visibility":"public"}'
   curl -X POST http://localhost:8080/events/$EVENT_ID/votes -H 'content-type: application/json' -d '{"track":"Song A","voterId":"user1"}'
   ```

10. Проверить результаты:
   ```bash
   curl http://localhost:8080/events/$EVENT_ID/tally
   ```

11. Проверить realtime:
   - открыть `ws.html`
   - нажать **Connect**
   - выполнить пункты 4–6 и наблюдать входящие события

---

## Нужно сделать:

- Подключить JWT middleware для Playlist/Vote сервисов  
- Добавить Swagger (OpenAPI) документацию  
- Настроить rate limiting и метрики  
- Написать unit и e2e тесты  
- Добавить CI (golangci-lint, миграции и автосборка)

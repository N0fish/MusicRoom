# MusicRoom — Go Microservices

<!-- Instruction for agents: Please refer to Agents/AGENTS.md for project-wide instructions. -->

Микросервисная архитектура музыкального приложения: авторизация, плейлисты, голосование и realtime через WebSocket.

## Сделать все вместе env + up + url
```bash
make start
```

## Init env в сервисы
```bash
make env
```

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
# or
docker compose logs -f auth-service  # логи конкретного сервиса
```
```bash
make down  # остановить и очистить тома
```
```bash
docker compose ps  # проверить состояние
```
```bash
docker stop <id> # если есть незакрытые порты
```

## Очистить базу данных
```bash
make rmbd
```

## Пересобрать
```bash
make re
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
| **mock-service**     | Mock / статистические данные | 3006 |
| **postgres**       | Общая база данных      | 5432 |
| **redis**          | Pub/Sub сообщения      | 6379 |

- API Gateway: http://localhost:8080
- Auth: http://localhost:3001
- User: http://localhost:3005
- Playlist: http://localhost:3002
- Vote: http://localhost:3003
- Realtime WS: ws://localhost:3004/ws
- Mock API: http://localhost:3006
- Postgres: localhost:5432 (user: postgres / password: postgres, db: musicroom)
- Redis: localhost:6379

### Запустить только на `go` по сервису (без докер):
Пример:
```bash
go run ./backend/services/mock-service/cmd/service
```

---

## Архитектура

```sql
                 frontend/mobile                
                       │
                       ▼
                ┌─────────────┐         ┌───────────────────┐
     requests → │ API Gateway │ /mock → │   mock-service    │
                └─────┬───────┘         └───────────────────┘
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
           │ /playlists → playlist-s.    │         ┌───────────────────┐
           │ /events    → vote-service   │ /mock → │   mock-service    │
           │ /users     → user-service   │         │             :3006 │
           └────────────┬────────────────┘         └───────────────────┘
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
### 1. API Gateway
### `api-gateway` — Единая точка входа (порт 8080) - единый backend / API.
- Проксирует:
  - `/auth` → `auth-service`
  - `/users` → `user-service`
  - `/playlists` → `playlist-service`
  - `/events` → `vote-service`
  - `/realtime` → `realtime-service`
  - `/mock` → `mock-service`
- Фрокт и мобилка могут обращаться **только к `localhost:8080`**, не к внутренним сервисам.

Смотри инфу по использованию этого сервиса в `backend/services/api-gateway/api_front-mobil.md`

---

### 2. Auth
### `auth-service` — Авторизация (порт 3001)
- POST /auth/register
- POST /auth/login
- POST /auth/refresh
- GET /auth/me
- POST /auth/request-email-verification
- GET /auth/verify-email
- POST /auth/forgot-password
- POST /auth/reset-password
- GET /auth/google/login
- GET /auth/google/callback
- GET /auth/42/login
- GET /auth/42/callback

---

### 3. User
### `user-service` — Профили пользователей (порт 3005)
- GET /users/me
- PATCH /users/me
- GET /users/{id}

Ex : GET /users/me
```json
{
  "id": "uuid-профиля",
  "userId": "uuid-пользователя",
  "displayName": "Alla",
  "avatarUrl": "https://example.com/avatar.png",
  "publicBio": "DJ from Paris",
  "friendsBio": "Только для друзей",
  "privateBio": "Личные заметки",
  "visibility": "public",
  "preferences": {
    "genres": ["techno", "house"],
    "artists": ["Syuzi Dogs"],
    "moods": ["party"]
  },
  "createdAt": "...",
  "updatedAt": "..."
}
```

---

### 4. Playlist
### `playlist-service` — Плейлисты и треки (порт 3002)
маршруты в разработке:
- GET /playlists
- POST /playlists
- PATCH /playlists/{id}
- /playlists, /playlists/:id/tracks,
- /events, /events/:id/votes, /events/:id/tally,

---

### 5. Vote
### `vote-service` — События и голосование (порт 3003)
в разработке :
- GET /events/{id}/vote — проголосовать  
- `/events` — создать событие  
- `/events/:id/tally` — посмотреть результаты  
- Также публикует события в Redis.

---

### 6. Realtime
### `realtime-service` — WebSocket уведомления (порт 3004)
- Клиенты подключаются к `ws://localhost:3004/ws`
- Принимает события от Redis и рассылает в браузеры
- Проверка:
  - открыть `ws.html` в браузере,
  - нажать **Connect**,
  - добавить трек или проголосовать — появится событие `playlist.created` или `vote.cast`.

---

### 7. Mock
### `mock-service` — Заглушечные данные для фронта/мобилки (порт 3006) СТАРАЯ ВЕРСИЯ
- надо переделать до актуальной версии. 

Сервис возвращает **фиксированные тестовые данные**, чтобы фронт и мобильное приложение
могли показывать заполненные экраны, даже если база пустая или реальный backend ещё не готов.

Эндпоинты (через API Gateway):

- `GET /mock/initial` — стартовый набор данных:
  - текущий пользователь (mock),
  - список плейлистов,
  - список событий.
- `GET /mock/user` — только мок-пользователь.
- `GET /mock/playlists` — только мок-плейлисты.
- `GET /mock/events` — только мок-события.

---

## Нужно сделать:
Дохуя всего надо сделать

- Добавить друзей в user service
- Отправка email с подтверждением почты
- сервис голосования
- сервис плейлиста
- реалтайм сервис
- может быть мок сервис доделать, хотя он не обязателен
- понять что будет происходить если нет инернета и как пользователь может продолжать пользоваться приложением
- написать тесты. думаю займусь этим в самом конце
- сделать уникальный ник для юзера с его почты, не изменяемый, чтобы использовать для URL
- поиск юзеров (выпалающий список) из 3х букв ?
- список друзей
- что делать с приватность? как сделать приватный профиль, чтобы не отображалась никакая информация ? 
- сделать фото по умолчанию
- как вообще ко мне попадает фото в юзер сервис, которое загружает пользователесь. Или сделать несколько аватарок на выбор ? не делать фото. И вообще оно надо в этом приложении ?
- местоположения пользователя и его время когда он может учавствовать в голосовании
- ограничение символов на бэке до 400символов или байт
- проблемы конкуренции в голосовании, это вообще о чем?
- какое максимальное колличество пользователей могут использовать приложение
- сделать сертификаты и перейти на https
- какой сервер выбрать, чтобы уйти от localhost
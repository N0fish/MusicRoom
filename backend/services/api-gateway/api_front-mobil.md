# Мини-чек-лист для фронта/мобилки

Регистрация / логин: `имеется`
1. `POST /auth/register` или `POST /auth/login`
2. Сохранить `accessToken`, `refreshToken`
3. `GET /auth/me` → получить `userId`

Профиль: `пока нет - в разработке`
4. `GET /users/me` → вытащить профиль
5. `PATCH /users/me` → сохранить изменения
6. `GET /users/{id}` → смотреть чужие профили (public view)

Плейлисты: `пока нет - в разработке`
7. `GET /playlists` → список публичных
8. `POST /playlists` (с токеном) → создать свой
9. `PATCH /playlists/{id}` (с токеном) → обновить

Токены: `имеется`
10. Если любой защищённый запрос возвращает 401 →
`POST /auth/refresh` → обновить токены → повторить исходный запрос.
11. Если `refresh` тоже падает с 401 → `разлогиниться`.

---

### Пока всего три сценария:
1. Регистрация + логин
2. Заполнение / правка профиля
3. Создание плейлиста
Все запросы только через api-gateway (http://localhost:8080)

## Сценарий 1: Регистрация нового пользователя и логин:
1.1. Регистрация
- Запрос
```http
POST /auth/register
Content-Type: application/json
```
тело
```json
{
  "email": "user@gmail.com",
  "password": "myStrongPassword"
}
```
- Ожидаемый ответ 201 Created:
```json
{
  "accessToken": "jwt-access...",
  "refreshToken": "jwt-refresh..."
}
```
Что делает фронт/мобилка:
Сахранить
- accessToken — в памяти (state, React Query, etc.),
- refreshToken — в более долговечном месте (secure storage в мобилке, httpOnly cookie / localStorage — как вы там решите по безопасности).
- Переключает UI в состояние «пользователь залогинен».

1.2. Логин (если уже зарегистрирован)
- Запрос
```http
POST /auth/login
Content-Type: application/json
```
тело
```json
{
  "email": "user@gmail.com",
  "password": "myStrongPassword"
}
```
- Ожидаемый ответ 200 OK:
```json
{
  "accessToken": "jwt-access...",
  "refreshToken": "jwt-refresh..."
}
```
Дальше то же самое: сохранить токены, перейти в залогиненное состояние.

1.3. Получить «кто я» (проверка токена, базовая инфа)
- Запрос
```http
GET /auth/me
Authorization: Bearer <accessToken>
```
тело
```json
{
  "userId": "uuid-пользователя",
  "email": "user@gmail.com",
  "emailVerified": true
}
```
Использование:
- userId — ключ для /users/{id}, плейлистов, и т.п.
Можно дергать при загрузке приложения:
- если 200 → пользователь авторизован,
- если 401 → токен сдох → пробуем refresh.

1.4. Обновление access токена по refresh
Когда любой запрос с Authorization: Bearer ... вернул 401, фронт:
1. Пытаемся обновить токены:
```http
POST /auth/refresh
Content-Type: application/json
```
тело
```json
{
  "refreshToken": "jwt-refresh..."
}
```
2. Ответ 200 OK:
```json
{
  "accessToken": "new-access...",
  "refreshToken": "new-refresh..."
}
```
3. Сохраняет новые токены.
4. Повторяет исходный запрос с новым accessToken.
Если /auth/refresh тоже вернул 401 → нужно разлогинить юзера и показать экран логина.


## Сценарий 2: Заполнение / обновление профиля
Во всех запросах профиля нужен accessToken.
2.1. Получить свой профиль
- Запрос
```http
GET /users/me
Authorization: Bearer <accessToken>
```
- Ответ 200 OK:
```json
{
  "id": "uuid-профиля",
  "userId": "uuid-пользователя",
  "displayName": "Alla",
  "bio": "DJ from Paris",
  "avatarUrl": "https://example.com/avatar.png",
  "visibility": "public",            // "public" | "friends" | "private"
  "preferences": {
    "genres": ["techno", "house"],
    "artists": ["Syuzi Dogs"],
    "moods": ["party"]
  },
  "createdAt": "2025-11-17T19:00:00Z",
  "updatedAt": "2025-11-17T19:00:00Z"
}
```
Использование:
- Наполнить экран «Мой профиль».
- Поля displayName, bio, avatarUrl, visibility, preferences — редактируемые.

2.2. Обновить свой профиль
- Запрос
```http
PATCH /users/me
Authorization: Bearer <accessToken>
Content-Type: application/json
```
- Тело отправляем только изменённые поля (остальные можно не включать):
```json
{
  "displayName": "DJ Alla",
  "bio": "I play techno & lofi.",
  "avatarUrl": "https://example.com/new-avatar.png",
  "visibility": "friends",
  "preferences": {
    "genres": ["techno", "lofi"],
    "artists": ["Syuzi Dogs"],
    "moods": ["study", "party"]
  }
}
```
- Ответ 200 OK:
```json
{
  "id": "uuid-профиля",
  "userId": "uuid-пользователя",
  "displayName": "DJ Alla",
  "bio": "I play techno & lofi.",
  "avatarUrl": "https://example.com/new-avatar.png",
  "visibility": "friends",
  "preferences": {
    "genres": ["techno", "lofi"],
    "artists": ["Syuzi Dogs"],
    "moods": ["study", "party"]
  },
  "createdAt": "...",
  "updatedAt": "..."
}
```
Фронт / мобилка после успешного ответа  может:
- закрыть форму,
- обновить локальный стейт профиля.

2.3. Посмотреть профиль другого пользователя
- Запрос
```http
GET /users/{id}
```
{id} — это userId из /auth/me или из какого-нибудь «списка пользователей».
- Ответ 200 OK:
```json
{
  "id": "uuid-пользователя",
  "displayName": "DJ Lilo",
  "bio": "Loves house music",
  "avatarUrl": "https://example.com/alice.png",
  "visibility": "public",
  "preferences": {
    "genres": ["house"],
    "artists": ["..."],
    "moods": ["chill"]
  }
}
```
Сервер сам решает, какие поля показать, исходя из `visibility`.
Фронту не нужно думать про права — он просто рисует то, что пришло.


## Сценарий 3: Создание плейлиста
3.1. Получить список публичных плейлистов
Можно вызывать без токена.
- Запрос
```http
GET /playlists
```
- Ответ 200 OK:
```json
[
  {
    "id": "playlist-uuid-1",
    "ownerId": "user-uuid-1",
    "name": "Friday Night",
    "description": "Techno party",
    "isPublic": true,
    "createdAt": "...",
    "updatedAt": "..."
  },
  {
    "id": "playlist-uuid-2",
    "ownerId": "user-uuid-2",
    "name": "Study Lofi",
    "description": "Chill lofi beats",
    "isPublic": true,
    "createdAt": "...",
    "updatedAt": "..."
  }
]
```

3.2. Создать новый плейлист
Требуется JWT.
- Запрос
```http
POST /playlists
Authorization: Bearer <accessToken>
Content-Type: application/json
```
тело
```json
{
  "name": "My First Playlist",
  "description": "Something cool",
  "isPublic": true
}
```
- Ответ 201 Created:
```json
{
  "id": "new-playlist-uuid",
  "ownerId": "user-uuid",
  "name": "My First Playlist",
  "description": "Something cool",
  "isPublic": true,
  "createdAt": "...",
  "updatedAt": "..."
}
```
Фронт:
- Добавляет плейлист в список,
- Может сразу перекинуть на экран управления этим плейлистом.

3.3. Обновить плейлист
- Запрос
```http
PATCH /playlists/{id}
Authorization: Bearer <accessToken>
Content-Type: application/json
```
тело
```json
{
  "name": "Renamed Playlist",
  "description": "Updated description",
  "isPublic": false
}
```
- Ответ 200 OK:
```json
{
  "id": "playlist-uuid",
  "ownerId": "user-uuid",
  "name": "Renamed Playlist",
  "description": "Updated description",
  "isPublic": false,
  "createdAt": "...",
  "updatedAt": "..."
}
```
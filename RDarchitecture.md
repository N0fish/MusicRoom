### Сервисы:
- `api-gateway` — общий REST API, валидация, JWT-проверка, rate limit, Swagger; единая точка входа для фронта и мобильного клиента (по сабжекту API — основной контракт для клиентов).

- `auth-service` — регистрация + логин + refresh токены, email verification, forgot/reset password, OAuth (Google + 42), выдача JWT.

- `user-service` — хранение пользовательского профиля (displayName, bio уровней visibility, avatarUrl, музыкальные предпочтения). Профиль создаётся автоматически. Данные можно редактировать через /users/me. OAuth данные (Google/42) не заполняют профиль — пользователь редактирует его сам.

- `playlist-service` — создание и редактирование плейлистов (public/private), база для realtime-сотрудничества. Логика совместного редактирования реализуется через realtime-service.

- `vote-service` — создание событий и голосование за треки. Результаты обновляются в realtime через redis → realtime-service.

- `realtime-service` — WebSocket-хаб. Передаёт realtime-события (голосование, обновления плейлиста, изменения состояния), обеспечивает remote control. Работает поверх Redis Pub/Sub.

### Хранилища: 
- `Postgres` (персистентные данные) + Redis (эфемерка и Pub/Sub).
Покрывает все обязательные сервисы из предмета.

### User-service - карточка пользователя:
- профиль (имя, аватар, описание);
- публичность профиля;
- предпочтения (жанры, интересы);
- приватные данные, которые не зависят от регистрации.

`GET /user/me`
```json
{
  "id": "7f71e2c9",
  "displayName": "DJ Neko",
  "bio": "Music lover",
  "visibility": "public",
  "preferences": {
    "genres": ["techno", "house"]
  }
}
```
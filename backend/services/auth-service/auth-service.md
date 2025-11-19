### Auth-service - служба идентификации:
- регистрация (создание учётной записи);
- логин
- валидация пароля;
- выдача JWT;
- refresh-токены;
- email verification;
- password reset;
- OAuth (Google/42).

`POST /auth/register`
```json
{ "email": "user@gmail.com", "password": "secret123" }
```
- создаёт пользователя в таблице auth_users;
- создаёт verification_token;
- логирует ссылку «подтвердить email»;
- возвращает accessToken + refreshToken.

`POST /auth/login`
- Вход по email + пароль
```json
->{ "email": "user@gmail.com", "password": "secret123" }
<-{ "accessToken": "...", "refreshToken": "..." }
```

`POST /auth/logout` - Он `не нужен` и его `нет`
это делается на вашей стороне, клиент удаляет у себя:
`accessToken`
`refreshToken`
И все, он больше не авторизован
Клиент без токена не может вызвать ни один защищённый эндпоинт.
(фронт удаляет токены (localStorage / secure storage))
(мобилка стирает secure storage)
Следовательно приложение переходит на экран логина

`POST /auth/refresh`
- принимает refresh-token → выдаёт новый access-token.
```json
{ "refreshToken": "..." }
```

`POST /auth/request-email-verification`
```json
{ "email": "..." }
```
GET /auth/verify-email?token=...

`POST /auth/forgot-password`
- принимает email → создаёт токен сброса, отправляет письмо.
```json
{ "email": "..." }
```

`POST /auth/reset-password`
- принимает токен и новый пароль → обновляет пароль.
```json
{
  "token": "reset-token",
  "newPassword": "newSecret123"
}
```

`GET /auth/google/login`
`GET /auth/google/callback`
`GET /auth/42/login`
`GET /auth/42/callback`
- приём callback, создание/поиск пользователя, выдача токена.

`GET /auth/me` - минимальная техническая информация о текущей сессии
- Возвращает техническую информацию о текущей сессии
```json
{
  "userId": "uuid...",
  "email": "user@gmail.com",
  "emailVerified": true
}
```
!!!Это не профиль пользователя.!!!
Профиль находится в `user-service` -> (`GET /users/me`).

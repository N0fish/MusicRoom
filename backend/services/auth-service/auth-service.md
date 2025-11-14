### Auth-service - служба идентификации:
- регистрация (создание учётной записи);
- логин / logout;
- валидация пароля;
- выдача JWT;
- refresh-токены;
- email verification;
- password reset;
- OAuth (Google/Facebook).

`POST /auth/register`
- тело: email, password (+ возможно имя);
- создаёт запись в auth-хранилище;
- отправляет письмо для подтверждения.

`POST /auth/login`
- тело: email, password;
- если успех — отдаёт JWT (и, возможно, refresh-token).

`POST /auth/refresh`
- принимает refresh-token → выдаёт новый access-token.

`POST /auth/logout`
- опционально — инвалидация refresh-токена.

`POST /auth/password/forgot`
- принимает email → создаёт токен сброса, отправляет письмо.

`POST /auth/password/reset`
- принимает токен и новый пароль → обновляет пароль.

`GET /auth/oauth/google` / `GET /auth/oauth/facebook`
- старт OAuth-флоу;

`GET /auth/oauth/google/callback`
- приём callback, создание/поиск пользователя, выдача токена.

`GET /auth/me` - минимальная техническая информация о текущей сессии
- по токену возвращает базовую инфу о пользователе (user_id, email).
- /auth/me != /users/me
/auth/me — это endpoint, который отвечает на вопрос:
- - Кто сейчас залогинен?
- - Какой у него user_id?
- - Проверен email или нет?

```json
{
  "userId": "7f71e2c9",
  "email": "alla@test.com",
  "emailVerified": true
}
```
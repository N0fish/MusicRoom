### Auth-service - служба идентификации

Отвечает за аутентификацию и техническое управление учётными записями:

- регистрация (создание учётной записи);
- логин (email + пароль);
- валидация пароля (bcrypt);
- выдача JWT (access / refresh);
- refresh-токены;
- email verification;
- password reset;
- OAuth (Google / 42);
- техническая информация о текущей сессии.

---

#### Формат JWT
Auth-service выдаёт пару токенов:
```json
{
  "accessToken": "…",
  "refreshToken": "…"
}
typ = "access" — токен для вызова защищённых эндпоинтов через API Gateway.
typ = "refresh" — токен для обновления пары токенов.
Флаг emailVerified берётся из БД в момент выдачи токена.
После `успешного подтверждения email нужно перевыпустить токены`
(логин или POST /auth/refresh), чтобы в новых токенах emailVerified стал true.
// (желательно обновлять токены, иначе я буду смотреть в базу данных... для тестов, есть закоментированная часть кода. но по хорошему это проверяет не база данных, а JWT за счет рефреша)


#### Регистрация и логин
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
Ошибки:
- 401 — неверные учётные данные;
- 400 — пустой email / пароль;
- 500 — внутренняя ошибка.

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
- парсит и валидирует JWT
- принимает refresh-token → выдаёт новый access-token.
- находит пользователя по userId;
- выдаёт новую пару accessToken + refreshToken
```json
{ "refreshToken": "..." }
```
Ошибки:
- 400 — неверный JSON / отсутствует поле;
- 401 — невалидный / просроченный refresh-токен;
- 500 — внутренняя ошибка.

`POST /auth/request-email-verification`
Повторная отправка письма с подтверждением email.
- генерирует новый verification_token
- отправляет письмо с ссылкой вида
```json
{ "email": "..." }
```
`GET /auth/verify-email?token=...` - Подтверждение email по ссылке из письма.
Query:
- token — токен из письма.
Ошибки:
- 400 — нет токена / токен не найден;
- 500 — внутренняя ошибка.
<!-- Эндпоинт можно дёргать сколько угодно раз с одним токеном, но после первого успешного запроса токен обнуляется, и следующие вызовы вернут ошибку «invalid token». --> в старой версии.

`POST /auth/forgot-password`
- принимает email → создаёт токен сброса, отправляет письмо.
- создаёт reset_token и reset_expires_at (например, +1 час);
```json
{ "email": "..." }
```
Ответ (всегда):
```json
{ "status": "reset link sent" }
```

`POST /auth/reset-password`
- принимает токен и новый пароль → обновляет пароль.
```json
{
  "token": "reset-token",
  "newPassword": "newSecret123"
}
```
Ошибки:
- 400 — неверный JSON / пустые поля / короткий пароль / невалидный или просроченный токен;
- 500 — внутренняя ошибка.

`GET /auth/google/login`
Редирект на страницу авторизации Google.
`GET /auth/google/callback`
Приём callback от Google. `/auth/callback#accessToken=...&refreshToken=...`
`GET /auth/42/login`
Аналогично Google, но через API 42.
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

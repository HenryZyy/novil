# novil

A self-hosted server for reading comics and books. The Go backend in
`apps/backend` serves a JSON API used by the web frontend and, later, mobile
apps.

## Running the backend

```sh
cd apps/backend
cp .env.example .env   # then set NOVIL_ADMIN_PASSWORD
go run ./cmd/novil
```

With Docker: create `apps/backend/.env` the same way, then run
`docker compose up --build` in the repository root. Create the `config` and
`library` folders first, or Docker creates them owned by root and the server
can't write to `config`.

On the first start, the admin from `NOVIL_ADMIN_USER`/`NOVIL_ADMIN_PASSWORD`
is created. See `apps/backend/.env.example` for all settings.

## API

All endpoints except `/health` and `/api/auth/login` need a token:

```
Authorization: Bearer <token>
```

### Login flow

1. `POST /api/auth/login` with `{"username": "...", "password": "..."}`
   returns `{"token", "expiresAt", "user"}`. Tokens last 7 days.
2. If `user.mustChangePassword` is true (every new account, and after an
   admin reset), all other endpoints answer `403` with code
   `password_change_required` until the user calls
   `PUT /api/me/password` with `{"currentPassword", "newPassword"}`.
3. A password change returns a new session like the login does. **Replace the
   stored token**: all older tokens of that user stop working.

`POST /api/auth/logout` revokes the token it is sent with; delete it on the
client too. The user's other devices stay logged in.

Login and password changes are rate limited per IP: 5 attempts at once, then
one more every 12 seconds. Beyond that the API answers `429` with a
`Retry-After` header (seconds to wait).

### Errors

Errors are JSON: `{"error": "human-readable message"}`. Auth errors also have
a `code` to check instead of the message:

| Status | Code | Meaning |
|---|---|---|
| 401 | `unauthorized` | No token, or it is invalid or expired → log in again |
| 401 | `invalid_credentials` | Wrong username or password at login |
| 403 | `invalid_credentials` | Wrong current password when changing it |
| 403 | `password_change_required` | Show the change-password screen |
| 403 | `forbidden` | Only admins can do this |
| 429 | `too_many_requests` | Too many password attempts, wait `Retry-After` seconds |

### Endpoints

| Method & path | Who | Description |
|---|---|---|
| `GET /health` | anyone | `{"status":"ok"}` |
| `POST /api/auth/login` | anyone | Log in, see above |
| `POST /api/auth/logout` | logged in¹ | Log out this token |
| `GET /api/me` | logged in¹ | The current user |
| `PUT /api/me/password` | logged in¹ | Change your own password |
| `GET /api/users` | admin | List users |
| `POST /api/users` | admin | `{"username", "password", "role"}` creates a user (`role`: `user` (default) or `admin`). They must change the password after logging in |
| `PUT /api/users/{id}/password` | admin | `{"password"}` resets a password and logs the user out |
| `DELETE /api/users/{id}` | admin | Delete a user and their progress (not yourself) |
| `GET /api/libraries` | user | List libraries |
| `POST /api/libraries` | admin | `{"name", "path"}` adds a folder on the server |
| `POST /api/libraries/{id}/scan` | admin | Rescan a library's files |
| `GET /api/series?library={id}` | user | List series, optionally of one library |
| `GET /api/series/{id}` | user | One series |
| `GET /api/series/{id}/books` | user | The books in a series |
| `GET /api/books/{id}` | user | One book |
| `GET /api/books/{id}/file` | user | Download the book file |
| `GET /api/books/{id}/pages/{n}` | user | Page `n` (from 1) of a CBZ as an image |
| `GET /api/books/{id}/progress` | user | Your progress: `{"bookId", "page", "completed", "updatedAt"}` |
| `PUT /api/books/{id}/progress` | user | `{"page", "completed"}` saves your progress |

¹ Also works while the user still has to change their password.

"user" means any logged-in user who has set their own password; admins can
do everything users can. Progress is per user.

Since the token is a header, `<img src="/api/books/1/pages/1">` doesn't work
directly: fetch the image with the header and show it via
`URL.createObjectURL(blob)`.

For browser apps on another origin (like the Vite dev server), list it in
`NOVIL_CORS_ORIGINS`.

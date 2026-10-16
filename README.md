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

Browsers don't need the header: logging in also sets the token as an
HttpOnly `novil_token` cookie, which the server accepts too.

### Login flow

1. `POST /api/auth/login` with `{"username": "...", "password": "..."}`
   returns `{"token", "expiresAt", "user"}`. Tokens last 7 days.
2. If `user.mustChangePassword` is true (every new account, and after an
   admin reset), all other endpoints answer `403` with code
   `password_change_required` until the user calls
   `PUT /api/me/password` with `{"currentPassword", "newPassword"}`.
3. A password change returns a new session like the login does. **Replace the
   stored token**: all older tokens of that user stop working.

`POST /api/auth/logout` revokes the token it is sent with and clears the
cookie; delete a stored token on the client too. The user's other devices
stay logged in.

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
| `PUT /api/me/avatar` | user | Upload your profile picture, see below |
| `DELETE /api/me/avatar` | user | Remove your profile picture |
| `GET /api/users/{id}/avatar` | user | A user's profile picture as a JPEG, `404` without one |
| `DELETE /api/users/{id}/avatar` | admin | Remove someone's profile picture |
| `GET /api/users` | admin | List users |
| `POST /api/users` | admin | `{"username", "password", "role"}` creates a user (`role`: `user` (default) or `admin`). They must change the password after logging in |
| `PATCH /api/users/{id}` | admin | `{"username"}` renames a user, `{"role": "admin"}` or `"user"` changes their role (not your own). Either or both; applies on the user's next request |
| `PUT /api/users/{id}/password` | admin | `{"password"}` resets a password and logs the user out |
| `DELETE /api/users/{id}` | admin | Delete a user and their progress (not yourself) |
| `GET /api/libraries` | user | List libraries |
| `POST /api/libraries` | admin | `{"name", "path"}` adds a folder on the server |
| `POST /api/libraries/{id}/scan` | admin | Rescan a library's files |
| `PATCH /api/libraries/{id}` | admin | `{"allowUploads": true}` turns uploads into this library on or off |
| `POST /api/libraries/{id}/books?series=…&filename=…` | admin | Upload a book: the raw file as the body. Only `.cbz`, `.epub` and `.pdf`, and the content must really be that format |
| `GET /api/series?library={id}` | user | List series, optionally of one library |
| `GET /api/series/{id}` | user | One series |
| `GET /api/series/{id}/books` | user | The books in a series |
| `PATCH /api/series/{id}` | admin | `{"hidden": true}` hides all its books, `false` shows them |
| `GET /api/books?search=…&library={id}&hidden=…` | admin | All books, by series. `search` matches title, author and series; `hidden` is `hidden` or `visible` |
| `GET /api/books/{id}` | user | One book |
| `PATCH /api/books/{id}` | admin | Edit metadata or hide the book, see below |
| `GET /api/books/{id}/cover` | user | The cover as a JPEG, see below |
| `PUT /api/books/{id}/cover` | admin | Upload a cover image |
| `DELETE /api/books/{id}/cover` | admin | Go back to the automatic cover |
| `GET /api/books/{id}/file` | user | Download the book file |
| `GET /api/books/{id}/pages/{n}` | user | Page `n` (from 1) of a CBZ as an image |
| `GET /api/books/{id}/progress` | user | Your progress: `{"bookId", "page", "completed", "updatedAt"}` |
| `PUT /api/books/{id}/progress` | user | `{"page", "completed"}` saves your progress |

¹ Also works while the user still has to change their password.

"user" means any logged-in user who has set their own password; admins can
do everything users can. Only admins add books; every user sees and reads
all of them. Progress is per user.

### Book metadata

Books carry `title`, `author`, `language`, `released` (publication date as
`2006`, `2006-01` or `2006-01-02`, or empty), `description` (plain text),
`addedAt` (when uploaded or first scanned), `pageCount` (CBZ only, else 0),
`size` (bytes) and `format`.

Scans and uploads read them from the file: the package document of an EPUB,
the `ComicInfo.xml` of a CBZ. PDFs only get their file name as the title for
now. Empty fields are unknown.

Admins can correct them per book with `PATCH /api/books/{id}`: a string
replaces the file's value, `null` goes back to it, and fields left out stay
unchanged. Scans never overwrite these edits. `edited` lists the fields an
admin set:

```json
{"title": "Dune (Deutsch)", "language": "de", "released": null}
```

### Hidden books

Admins can hide a book from everyone else with `PATCH /api/books/{id}`
(`{"hidden": true}`), or a whole series with `PATCH /api/series/{id}`. Users
don't see hidden books anywhere and get `404` for them; a series whose books
are all hidden is gone for them too. Admins still see everything: books carry
`hidden`, and series `hiddenCount`. Scans keep books hidden.

### Covers

`GET /api/books/{id}/cover` returns a JPEG of at most 800×1200 for every
format: the first page of a CBZ or PDF, the cover image of an EPUB (or the
first image on its first pages), or one an admin uploaded instead. `404` means
the book has none, so show a placeholder. A series' `coverBookId` names the
book whose cover stands for it.

Covers are made once and kept until the file changes. Send them with
`If-None-Match` (browsers do this on their own) to get a cheap `304` while
unchanged. The first PDF cover after a start takes a few seconds; `503` with
`Retry-After` means try again shortly.

Admins replace a cover with `PUT /api/books/{id}/cover` (the image file as
the body: JPEG, PNG, GIF or WebP, up to 20 MB) and go back to the automatic one
with `DELETE /api/books/{id}/cover`. Both return the book; `customCover` says
whether an uploaded cover is in use.

### Profile pictures

Profile pictures are stored on the server, so they're the same on every
device. Upload one with `PUT /api/me/avatar`, the image file as the body
(JPEG, PNG, GIF or WebP, up to 20 MB). The server cuts the middle square out
and scales it to at most 256×256, so there's no need to resize it first.

Users carry `hasAvatar`; show `GET /api/users/{id}/avatar` when it's true and
initials otherwise. Like covers, the picture answers `If-None-Match` with
`304` until it changes. Upload and both deletes return the updated user.

Thanks to the cookie, `<img src="/api/books/1/pages/1">` and download links
work in the browser as they are. Other clients send the header.

For browser apps on another origin (like the Vite dev server), list it in
`NOVIL_CORS_ORIGINS`.

# ChronosHub

An availability calendar and booking app built with Go, chi and PostgreSQL (Supabase or local). Visitors see open and booked time slots and can request a slot. An admin manages entries. You can install it as a Progressive Web App.

## Features

- A month grid with a schedule sidebar. On phones, days show colored dots and the day details open in a bottom sheet.
- Light and dark themes that follow the system setting, plus a manual toggle.
- Installable PWA: app manifest, icons, a service worker with offline fallback, and cached schedule data.
- Admin sessions are signed with HMAC-SHA256 using `SESSION_SECRET`. Requester emails are visible only to the admin.
- No build step or CDN: plain HTML templates, CSS and JavaScript served by the Go binary.

## Running locally

```bash
cp .env.example .env        # adjust DATABASE_URL, ADMIN_PASSWORD, SESSION_SECRET
go run ./cmd/main.go        # http://localhost:8080
```

Or with Docker (includes Postgres):

```bash
docker compose up --build
```

The `kalender_tasks` table is created on startup. If it is empty, it is filled with sample data from `migrations/001_create_tasks.sql`.

## Configuration

| Variable         | Description                                               |
| ---------------- | --------------------------------------------------------- |
| `PORT`           | HTTP port (default `8080`)                                |
| `ENV`            | `production` enables secure cookies and config warnings   |
| `DATABASE_URL`   | PostgreSQL connection string                              |
| `ADMIN_PASSWORD` | Password for admin sign-in                                |
| `SESSION_SECRET` | Random string of 32+ characters used to sign admin sessions |

## HTTP API

| Method   | Path                     | Auth  | Description                        |
| -------- | ------------------------ | ----- | ---------------------------------- |
| `GET`    | `/api/tasks?date=YYYY-MM-DD` | –  | Entries for a day                  |
| `POST`   | `/api/request-slot`      | –     | Visitor booking request            |
| `POST`   | `/api/login`             | –     | Admin sign in (sets session cookie)|
| `POST`   | `/api/logout`            | –     | Admin sign out                     |
| `POST`   | `/api/tasks`             | Admin | Create entry                       |
| `PUT`    | `/api/tasks/{id}/toggle` | Admin | Toggle booked / available          |
| `DELETE` | `/api/tasks/{id}`        | Admin | Delete entry                       |
| `GET`    | `/healthz`               | –     | Health check                       |

API clients can authenticate with the `X-Admin-Password` header instead of the cookie.

## PWA notes

The service worker (`static/sw.js`, served at `/sw.js`) caches the app files and serves stale copies while it refetches them in the background. Pages and `/api/tasks` are fetched from the network first and fall back to the cache when offline. **Bump `VERSION` in `sw.js`** whenever you change the cached CSS, JS or icons. Service workers need HTTPS, except on `localhost`.

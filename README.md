# Ankra Guestbook

A deliberately small web app that proves a real deployment: it renders, it talks to
PostgreSQL, and it tells you which pod answered.

- `GET  /`         guestbook page — entries, pod name, live DB status
- `POST /`         adds an entry (form post)
- `GET  /healthz`  readiness — fails until PostgreSQL answers

## Configuration

`DATABASE_URL`, or the discrete `PGHOST` / `PGPORT` / `PGUSER` / `PGPASSWORD` /
`PGDATABASE` / `PGSSLMODE` variables. `PORT` defaults to 8080. `POD_NAME` and
`APP_VERSION` are surfaced in the UI.

The app retries the database for 60s on boot rather than crash-looping, and the
schema is created on first connect.

## Local

```sh
docker run -d --name pg -e POSTGRES_PASSWORD=guestbook -e POSTGRES_DB=guestbook -p 5432:5432 postgres:17-alpine
PGPASSWORD=guestbook go run .
```

# 3do

3do is a self-hosted 3D print queue tracker. It is intentionally scoped as a
single-node app first: one web process, SQLite, and one documented data volume.

## Current Scaffold

- Go HTTP server with a small static frontend.
- SQLite schema migrations on startup.
- Local password auth, server-side sessions, and admin user management.
- Queue item and printer APIs.
- Upload handling for model/source files under `/data/uploads`.
- External model links for Printables, MakerWorld, Thingiverse, GitHub, direct
  URLs, and other sources.
- Dockerfile, Compose file, `.env.example`, and healthcheck.

## Paved-Road Deployment

```sh
cp .env.example .env
secret="$(openssl rand -base64 32)"
sed -i.bak "s|^SESSION_SECRET=$|SESSION_SECRET=${secret}|" .env
rm .env.bak
docker compose up -d --build
```

Open `http://localhost:8080` and create the first admin account. The bootstrap
screen is available only while the users table is empty; after the first admin
exists, sign in with that account and manage users from the app.

Durable data lives in `./data` when using the included Compose file:

- `./data/3do.db` - SQLite database
- `./data/uploads/` - uploaded STL/3MF/G-code/source files

Back up the entire `./data` directory. Do not treat the container filesystem as
durable storage.

## Local Development

```sh
make dev
make test
make build
```

The app defaults to `./data` locally, so local development does not require
Docker. `make dev` provides a local-only session secret when `SESSION_SECRET` is
unset; set your own value when testing production-like config.

## Environment

| Variable | Default | Purpose |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP listen address |
| `APP_URL` | `http://localhost:8080` | Public URL for links and reverse proxy setups |
| `DATA_DIR` | `./data` | Durable data root |
| `DATABASE_PATH` | `${DATA_DIR}/3do.db` | SQLite database path |
| `UPLOAD_MAX_MB` | `512` | Max upload request size |
| `SESSION_SECRET` | unset | Required secret for session token hashing; must be at least 32 characters |

`SESSION_SECRET` is validated on startup. The server refuses to start when it is
unset, shorter than 32 characters, or still set to the sample placeholder.

## Initial Admin Bootstrap

The paved-road path is the browser bootstrap screen shown on first launch. For
headless installs, create the first admin with the API before any other users
exist:

```sh
curl -X POST http://localhost:8080/api/bootstrap/admin \
  -H 'Content-Type: application/json' \
  -d '{"display_name":"Admin","email":"admin@example.com","password":"use-a-long-unique-password"}'
```

Passwords must be at least 12 characters. Passwords are stored with bcrypt, and
session cookies contain only opaque random tokens; the database stores HMAC
hashes of those tokens.

## API Sketch

- `GET /healthz`
- `GET /api/bootstrap`
- `POST /api/bootstrap/admin`
- `GET /api/session`
- `POST /api/login`
- `POST /api/logout`
- `GET /api/users` (admin)
- `POST /api/users` (admin)
- `PATCH /api/users/{id}` (admin)
- `DELETE /api/users/{id}` (admin)
- `GET /api/queue-items`
- `POST /api/queue-items`
- `GET /api/printers`
- `POST /api/printers`

`POST /api/queue-items` accepts either JSON or multipart form data. Multipart
uploads can include repeated `files` fields plus newline-separated `links`.
Queue, printer, and user-management APIs require an active session.

## Next Build Steps

1. Add item detail editing, notes, and status transition history in the UI.
2. Add stronger validation and duplicate detection using stored checksums.
3. Add migrations tests around upgrade compatibility.
4. Add optional metadata extractors/thumbnails for model files.

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

### Build from source

```sh
cp .env.example .env
secret="$(openssl rand -base64 32)"
sed -i.bak "s|^SESSION_SECRET=$|SESSION_SECRET=${secret}|" .env
rm .env.bak
docker compose up -d --build
```

### Run the published image

Published images are available from GHCR as `ghcr.io/zachatrocity/3do`.

Useful tags:

- `edge` - latest image from the default branch.
- `main` - default branch image.
- `sha-<short-sha>` - immutable image for a specific commit.
- `latest` - default branch only.
- `vX.Y.Z` - release tag builds.

Pull and run the default-branch image:

```sh
docker pull ghcr.io/zachatrocity/3do:edge
mkdir -p data
docker run -d \
  --name 3do \
  --restart unless-stopped \
  -p 8080:8080 \
  -e APP_URL=http://localhost:8080 \
  -e ADDR=:8080 \
  -e DATA_DIR=/data \
  -e DATABASE_PATH=/data/3do.db \
  -e UPLOAD_MAX_MB=512 \
  -e SESSION_SECRET="$(openssl rand -base64 32)" \
  -v "$PWD/data:/data" \
  ghcr.io/zachatrocity/3do:edge
```

Or use Compose with the published image:

```yaml
services:
  3do:
    image: ghcr.io/zachatrocity/3do:edge
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      APP_URL: http://localhost:8080
      ADDR: :8080
      DATA_DIR: /data
      PUID: "1000"
      PGID: "1000"
      DATABASE_PATH: /data/3do.db
      UPLOAD_MAX_MB: "512"
      SESSION_SECRET: ${SESSION_SECRET:?set SESSION_SECRET to at least 32 characters}
    volumes:
      - ./data:/data
    healthcheck:
      test: ["CMD", "/app/3do", "healthcheck"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
```

Open `http://localhost:8080` and create the first admin account. The bootstrap
screen is available only while the users table is empty; after the first admin
exists, sign in with that account and manage users from the app.

Durable data lives in `./data` when using the included Compose file:

- `./data/3do.db` - SQLite database
- `./data/uploads/` - uploaded STL/3MF/G-code/source files

The container starts as root only long enough to create and chown `DATA_DIR`,
then runs the long-lived 3do process as `PUID:PGID` through `su-exec`. The
default `PUID=1000` and `PGID=1000` match the first regular user on many Linux
hosts. If your self-hosting user has a different UID or GID, set those values in
`.env` before first startup:

```sh
id -u
id -g
```

Files in `./data` should be owned by that configured UID/GID. Do not use
`chmod 777` as the normal deployment fix; the entrypoint is responsible for
preparing the mounted volume.

Back up the entire `./data` directory. The database stores upload metadata and
SHA-256 checksums, while the files themselves live under `./data/uploads`; keep
those together when backing up or restoring. Do not treat the container
filesystem as durable storage.

For a quiet backup, stop the container and copy `./data` as one unit:

```sh
docker compose down
tar -czf 3do-data-backup.tgz data
docker compose up -d
```

Restore by stopping the container, replacing `./data` with the backup contents,
and starting the same or newer compatible 3do image. Never restore only
`3do.db` without `uploads/`, or only `uploads/` without `3do.db`; that leaves the
queue with broken file references or orphaned files.

## Local Development

```sh
make dev
make test
make build
make docker-smoke
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
| `PUID` | `1000` | Container runtime UID for files under `DATA_DIR` |
| `PGID` | `1000` | Container runtime GID for files under `DATA_DIR` |
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

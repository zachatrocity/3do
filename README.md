# 3do

3do is a self-hosted 3D print queue tracker. It is intentionally scoped as a
single-node app first: one web process, SQLite, and one documented data volume.

## Current Scaffold

- Go HTTP server with a small static frontend.
- SQLite schema migrations on startup.
- Queue item and printer APIs.
- Upload handling for model/source files under `/data/uploads`.
- External model links for Printables, MakerWorld, Thingiverse, GitHub, direct
  URLs, and other sources.
- Dockerfile, Compose file, `.env.example`, and healthcheck.

## Paved-Road Deployment

```sh
cp .env.example .env
docker compose up -d --build
```

Open `http://localhost:8080`.

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
Docker.

## Environment

| Variable | Default | Purpose |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP listen address |
| `APP_URL` | `http://localhost:8080` | Public URL for links and reverse proxy setups |
| `DATA_DIR` | `./data` | Durable data root |
| `DATABASE_PATH` | `${DATA_DIR}/3do.db` | SQLite database path |
| `UPLOAD_MAX_MB` | `512` | Max upload request size |
| `SESSION_SECRET` | unset | Reserved for auth/session work |

## API Sketch

- `GET /healthz`
- `GET /api/queue-items`
- `POST /api/queue-items`
- `GET /api/printers`
- `POST /api/printers`

`POST /api/queue-items` accepts either JSON or multipart form data. Multipart
uploads can include repeated `files` fields plus newline-separated `links`.

## Next Build Steps

1. Add local authentication and user management.
2. Add item detail editing, notes, and status transition history in the UI.
3. Add stronger validation and duplicate detection using stored checksums.
4. Add migrations tests around upgrade compatibility.
5. Add optional metadata extractors/thumbnails for model files.

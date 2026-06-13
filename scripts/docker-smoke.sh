#!/bin/sh
set -eu

image="${IMAGE:-3do:smoke}"
container="${CONTAINER:-3do-smoke}"
host_port="${HOST_PORT:-18080}"
data_dir="$(mktemp -d)"
host_uid="$(id -u)"
host_gid="$(id -g)"

cleanup() {
	docker rm -f "$container" >/dev/null 2>&1 || true
	if [ -d "$data_dir" ]; then
		docker run --rm -v "${data_dir}:/data" alpine:3.22 chown -R "${host_uid}:${host_gid}" /data >/dev/null 2>&1 || true
		rm -rf "$data_dir"
	fi
}
trap cleanup EXIT

docker build -t "$image" .

docker run -d \
	--name "$container" \
	-p "127.0.0.1:${host_port}:8080" \
	-e APP_URL="http://127.0.0.1:${host_port}" \
	-e PORT=8080 \
	-e DATA_DIR=/data \
	-e PUID="$host_uid" \
	-e PGID="$host_gid" \
	-e DATABASE_PATH=/data/3do.db \
	-e SESSION_SECRET=local-smoke-test-session-secret-32-chars-minimum \
	-v "${data_dir}:/data" \
	"$image" >/dev/null

if ! curl --retry 30 --retry-all-errors --retry-delay 1 -fsS "http://127.0.0.1:${host_port}/healthz" >/dev/null; then
	docker logs "$container" || true
	exit 1
fi

if [ ! -f "${data_dir}/3do.db" ]; then
	ls -la "$data_dir" || true
	docker logs "$container" || true
	echo "expected SQLite database at ${data_dir}/3do.db" >&2
	exit 1
fi

if [ ! -d "${data_dir}/uploads" ]; then
	ls -la "$data_dir" || true
	docker logs "$container" || true
	echo "expected uploads directory at ${data_dir}/uploads" >&2
	exit 1
fi

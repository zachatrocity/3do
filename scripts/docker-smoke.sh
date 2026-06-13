#!/bin/sh
set -eu

image="${IMAGE:-3do:smoke}"
container="${CONTAINER:-3do-smoke}"
host_port="${HOST_PORT:-18080}"
data_dir="$(mktemp -d)"

cleanup() {
	docker rm -f "$container" >/dev/null 2>&1 || true
	rm -rf "$data_dir"
}
trap cleanup EXIT

docker build -t "$image" .

docker run -d \
	--name "$container" \
	-p "127.0.0.1:${host_port}:8080" \
	-e APP_URL="http://127.0.0.1:${host_port}" \
	-e ADDR=:8080 \
	-e DATA_DIR=/data \
	-e DATABASE_PATH=/data/3do.db \
	-e SESSION_SECRET=local-smoke-test-session-secret-32-chars-minimum \
	-v "${data_dir}:/data" \
	"$image" >/dev/null

for _ in $(seq 1 30); do
	if curl -fsS "http://127.0.0.1:${host_port}/healthz" >/dev/null; then
		break
	fi
	sleep 1
done

curl -fsS "http://127.0.0.1:${host_port}/healthz" >/dev/null
test -f "${data_dir}/3do.db"
test -d "${data_dir}/uploads"

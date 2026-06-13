#!/bin/sh
set -eu

is_uint() {
	case "${1:-}" in
		''|*[!0-9]*)
			return 1
			;;
	esac
}

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"
DATA_DIR="${DATA_DIR:-/data}"
DATABASE_PATH="${DATABASE_PATH:-${DATA_DIR}/3do.db}"
UPLOAD_DIR="${UPLOAD_DIR:-${DATA_DIR}/uploads}"

if ! is_uint "$PUID"; then
	echo "PUID must be a numeric user id" >&2
	exit 1
fi

if ! is_uint "$PGID"; then
	echo "PGID must be a numeric group id" >&2
	exit 1
fi

DATABASE_DIR="$(dirname "$DATABASE_PATH")"

mkdir -p "$DATA_DIR" "$UPLOAD_DIR" "$DATABASE_DIR"
chown -R "$PUID:$PGID" "$DATA_DIR" "$UPLOAD_DIR" "$DATABASE_DIR" || true

can_write_as_runtime_user() {
	su-exec "$PUID:$PGID" sh -eu -c '
		for dir do
			test_file="${dir}/.3do-write-test-$$"
			: > "$test_file"
			rm -f "$test_file"
		done
	' sh "$DATA_DIR" "$UPLOAD_DIR" "$DATABASE_DIR"
}

if ! can_write_as_runtime_user; then
	echo "error: ${DATA_DIR}, ${UPLOAD_DIR}, or ${DATABASE_DIR} is not writable by ${PUID}:${PGID}" >&2
	echo "error: set PUID/PGID to the host user that owns the mounted data directory or fix host permissions" >&2
	exit 1
fi

exec su-exec "$PUID:$PGID" "$@"

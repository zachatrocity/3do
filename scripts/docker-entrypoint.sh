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

if ! is_uint "$PUID"; then
	echo "PUID must be a numeric user id" >&2
	exit 1
fi

if ! is_uint "$PGID"; then
	echo "PGID must be a numeric group id" >&2
	exit 1
fi

mkdir -p "$DATA_DIR"
chown -R "$PUID:$PGID" "$DATA_DIR"

exec su-exec "$PUID:$PGID" "$@"

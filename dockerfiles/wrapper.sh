#!/bin/sh

PUID=${PUID:-1000}
PGID=${PGID:-1000}

groupmod -o -g "${PGID}" outblocks >/dev/null
usermod -o -u "${PUID}" outblocks >/dev/null

if [ "${CI:-false}" = "true" ] || [ "${CI:-false}" = "1" ] ||
    [ "${RUN_AS_ROOT:-false}" = "true" ] || [ "${RUN_AS_ROOT:-false}" = "1" ] ||
    [ "$(id -u || true)" != "0" ]; then

    exec /bin/ok "$@"
    return
fi

exec gosu outblocks /bin/ok "$@"

#!/bin/sh

PUID=${PUID:-1000}
PGID=${PGID:-1000}

groupmod -o -g "${PGID}" outblocks > /dev/null
usermod -o -u "${PUID}" outblocks > /dev/null

if [ "${RUN_AS_ROOT:-0}" = "1" ]; then
    exec "$@"
else
    exec gosu outblocks "$@"
fi

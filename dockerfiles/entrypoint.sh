#!/bin/sh

PUID=${PUID:-1000}
PGID=${PGID:-1000}

groupmod -o -g "${PGID}" outblocks > /dev/null
usermod -o -u "${PUID}" outblocks > /dev/null

exec gosu outblocks "$@"

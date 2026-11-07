#!/bin/sh
set -e

# D-Bus-Systembus starten
mkdir -p /run/dbus

dbus-daemon --system --nofork &
DBUS_PID=$!

# Aufräumen, wenn der Container beendet wird
cleanup() {
    kill "$DBUS_PID" 2>/dev/null || true
}

trap cleanup TERM INT EXIT

# Python/Flask als unprivilegierter Benutzer starten
exec su -s /bin/sh app -c 'exec "$0" "$@"' "$@"


#!/bin/bash
# start.sh APP(wps|wpp|et) DISPLAYNUM PORT DOC [WORKDIR] -> Xvfb + server REPL + app WPS trong HOME tam.
# Khong dung HOME/ho so WPS that. WORKDIR mac dinh /tmp/axiom-wps-probe/<APP>.
set -e
APP=$1; DN=$2; PORT=$3; DOC=$4
[ -n "$DOC" ] || { echo "usage: start.sh wps|wpp|et DISPLAYNUM PORT DOC [WORKDIR]" >&2; exit 2; }
D=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$D/../../.." && pwd)
W=${5:-/tmp/axiom-wps-probe/$APP}
rm -rf "$W"; mkdir -p "$W/home" "$W/jobs"
python3 "$REPO/scripts/wps/install_jsaddon.py" --home "$W/home" --core-url "http://127.0.0.1:$PORT" --test-profile >/dev/null
setsid nohup python3 "$D/server.py" "$PORT" "$W" > "$W/server.log" 2>&1 &
echo $! > "$W/server.pid"
setsid nohup Xvfb ":$DN" -screen 0 1280x900x24 -nolisten tcp > "$W/xvfb.log" 2>&1 &
echo $! > "$W/xvfb.pid"
for _ in $(seq 50); do [ -e "/tmp/.X11-unix/X$DN" ] && break; sleep 0.1; done
cp "$DOC" "$W/home/"
env -u XDG_RUNTIME_DIR -u XDG_CONFIG_HOME -u XDG_DATA_HOME HOME="$W/home" DISPLAY=":$DN" \
  setsid nohup "$APP" "$W/home/$(basename "$DOC")" > "$W/app.log" 2>&1 &
echo $! > "$W/app.pid"
echo "started $APP on :$DN port $PORT work $W"
echo "lan dau: hop thoai tin cay add-in -> DISPLAY=:$DN xdotool mousemove 696 461 click 1"

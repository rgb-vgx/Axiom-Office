#!/usr/bin/env bash
# Extension bridge LibreOffice tren Linux (LibreOffice_arch.md muc 12): dong goi .oxt, cai/go bang unopkg
# cua nguoi dung (khong can root), xem trang thai va log. Khong bao gio tat LibreOffice cua nguoi dung.
#
#   scripts/libreoffice.sh package | install | uninstall | status | log
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UNOPKG="${UNOPKG:-$(command -v unopkg || echo /usr/lib/libreoffice/program/unopkg)}"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/axiom-office"
RUNTIME_DIR="${XDG_RUNTIME_DIR:-$HOME/.cache}/axiom-office"
EXTENSION_ID="org.axiomoffice.bridge"

soffice_running() {
    pgrep -u "$(id -u)" -f 'soffice\.bin' >/dev/null 2>&1
}

assert_closed() {
    if soffice_running; then
        echo "LibreOffice dang chay (pid $(pgrep -u "$(id -u)" -f 'soffice\.bin' | tr '\n' ' ')). Dong LibreOffice roi chay lai - script khong tu tat." >&2
        exit 1
    fi
}

check_python_uno() {
    # Extension la component Python: can loader Python cua LibreOffice (goi python3-uno tren Debian/Ubuntu).
    if ! python3 -c 'import uno' >/dev/null 2>&1; then
        echo "Thieu Python UNO cua LibreOffice. Cai: sudo apt install python3-uno (Debian/Ubuntu) hoac" \
             "sudo dnf install libreoffice-pyuno (Fedora)." >&2
        exit 1
    fi
}

package() {
    python3 "$ROOT/scripts/package_oxt.py" --print-path
}

install() {
    assert_closed
    check_python_uno
    [ -x "$UNOPKG" ] || { echo "Khong thay unopkg ($UNOPKG) - LibreOffice da cai chua?" >&2; exit 1; }
    local oxt
    oxt="$(package)"
    "$UNOPKG" add --force "$oxt"
    echo "Installed $oxt"
}

uninstall() {
    assert_closed
    "$UNOPKG" remove "$EXTENSION_ID" && echo "Removed $EXTENSION_ID" || echo "unopkg remove that bai (extension chua cai?)"
}

status() {
    echo "unopkg: $UNOPKG"
    "$UNOPKG" list 2>/dev/null | grep -E "Identifier|Version|is registered" | grep -A2 "$EXTENSION_ID" || echo "(chua cai $EXTENSION_ID)"
    echo
    echo "Sessions ($RUNTIME_DIR/sessions):"
    ls "$RUNTIME_DIR/sessions" 2>/dev/null || echo "  (trong)"
    for port in 47851 47852 47853; do
        if curl -s --max-time 2 "http://127.0.0.1:$port/health" >/tmp/axiom-health.json 2>/dev/null; then
            echo "Health $port: $(cat /tmp/axiom-health.json)"
        else
            echo "Health $port: khong tra loi"
        fi
    done
}

log() {
    tail -n 40 "$DATA_DIR/bridge.log" 2>/dev/null || echo "Chua co $DATA_DIR/bridge.log"
}

case "${1:-package}" in
    package) package ;;
    install) install ;;
    uninstall) uninstall ;;
    status) status ;;
    log) log ;;
    *) echo "dung: $0 package|install|uninstall|status|log" >&2; exit 2 ;;
esac

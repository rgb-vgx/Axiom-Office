#!/usr/bin/env bash
# Cai Axiom Office cho LibreOffice tren Linux (LibreOffice_arch.md giai doan L3) - khong can root.
#
#   ./install.sh [--endpoint URL] [--model NAME] [--api-key KEY|-] [--systemd] [--no-wps]   cai / nang cap
#                 (--api-key - = doc khoa tu stdin; --systemd = bat Core chay thuong truc bang systemd --user;
#                  --no-wps = khong dang ky add-in WPS du may co WPS)
#   ./install.sh --uninstall [--purge]                                           go (--purge xoa ca cau hinh, du lieu)
#
# Chay tu thu muc giai nen cua axiom-office-linux-x64-<ver>.tar.gz (co core/ va *.oxt canh script):
#   - Agent Core -> ~/.local/share/axiom-office/core (extension tu khoi dong khi can)
#   - extension -> unopkg cua nguoi dung
#   - cau hinh  -> ~/.config/axiom-office/config.json (quyen 0600; giu khoa cu, chi ghi khoa duoc truyen)
#   - MCP       -> ~/.local/share/axiom-office/mcp (in san doan cau hinh cho Claude Code/Desktop)
#   - WPS       -> co /opt/kingsoft/wps-office thi dang ky add-in (Core phuc vu) vao publish.xml cua nguoi dung;
#                  in huong dan sudo neu oem.ini chua bat JsApiPlugin (khong tu chay sudo)
# Khong bao gio tat LibreOffice cua nguoi dung: dang chay thi dung lai va nhac dong.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/axiom-office"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/axiom-office"
CORE_DIR="$DATA_DIR/core"
# MCP server ban Linux la subcommand `mcp` cua chinh Agent Core, khong con thu muc rieng.
# MCP_DIR chi con de don sach ban cai cu (truoc day la axiom-office-mcp .NET tuong trung).
MCP_DIR="$DATA_DIR/mcp"
EXTENSION_ID="org.axiomoffice.bridge"
UNIT_NAME="axiom-office-core.service"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
UNOPKG="${UNOPKG:-$(command -v unopkg || echo /usr/lib/libreoffice/program/unopkg)}"
# Thu muc cai WPS (test doi bang AXIOM_WPS_ROOT); khong ton tai = khong co WPS.
WPS_ROOT="${AXIOM_WPS_ROOT:-/opt/kingsoft/wps-office}"

ENDPOINT="" MODEL="" API_KEY="" UNINSTALL=0 PURGE=0 SYSTEMD=0 NOWPS=0
while [ $# -gt 0 ]; do
    case "$1" in
        --endpoint) ENDPOINT="$2"; shift 2 ;;
        --model) MODEL="$2"; shift 2 ;;
        --api-key) API_KEY="$2"; shift 2 ;;
        --systemd) SYSTEMD=1; shift ;;
        --uninstall) UNINSTALL=1; shift ;;
        --purge) PURGE=1; shift ;;
        --no-wps) NOWPS=1; shift ;;
        -h|--help) awk 'NR > 1 && /^#/ { print; next } NR > 1 { exit }' "$0"; exit 0 ;;
        *) echo "Tham so la: $1 (xem --help)" >&2; exit 2 ;;
    esac
done
# --api-key - : doc khoa tu stdin (khong luu vao lich su shell / danh sach tien trinh)
if [ "$API_KEY" = "-" ]; then IFS= read -r API_KEY || true; fi

die() { echo "Loi: $*" >&2; exit 1; }

assert_libreoffice_closed() {
    # Test bo qua kiem tra nay bang AXIOM_INSTALL_SKIP_LO_CHECK=1.
    [ "${AXIOM_INSTALL_SKIP_LO_CHECK:-0}" = "1" ] && return 0
    if pgrep -u "$(id -u)" -f '[s]office\.bin' >/dev/null 2>&1; then
        die "LibreOffice dang chay - dong tat ca cua so LibreOffice roi chay lai (script khong tu tat)."
    fi
}

stop_core() {
    # Core cu dang chay tu thu muc cai dat -> dung truoc khi ghi de (Core chi phuc vu LibreOffice, khong mat du lieu).
    local pid
    for pid in $(pgrep -u "$(id -u)" -f "$CORE_DIR/AxiomOffice.Core" 2>/dev/null || true); do
        kill -TERM "$pid" 2>/dev/null || true
    done
    for _ in 1 2 3 4 5 6 7 8 9 10; do
        pgrep -u "$(id -u)" -f "$CORE_DIR/AxiomOffice.Core" >/dev/null 2>&1 || return 0
        sleep 0.5
    done
    die "Agent Core cu khong dung ($CORE_DIR)."
}

write_config() {
    mkdir -p "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"
    # Python chi dung thu vien chuan (python3 luon co khi co python3-uno). Khoa API truyen qua env, khong qua argv.
    AXIOM_CFG="$CONFIG_DIR/config.json" AXIOM_ENDPOINT="$ENDPOINT" AXIOM_MODEL="$MODEL" AXIOM_KEY="$API_KEY" \
        python3 - <<'EOF'
import json, os
path = os.environ["AXIOM_CFG"]
try:
    with open(path, encoding="utf-8") as handle:
        data = json.load(handle)
except (OSError, ValueError):
    data = {}
for name, env in (("LlmEndpoint", "AXIOM_ENDPOINT"), ("LlmModel", "AXIOM_MODEL"), ("LlmApiKey", "AXIOM_KEY")):
    if os.environ.get(env):
        data[name] = os.environ[env]
if os.environ.get("AXIOM_ENDPOINT"):
    data.setdefault("LlmProvider", "openai")
fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
with os.fdopen(fd, "w", encoding="utf-8") as handle:
    json.dump(data, handle, indent=2, ensure_ascii=False)
os.chmod(path, 0o600)
missing = [name for name in ("LlmEndpoint", "LlmModel") if not data.get(name)]
if missing:
    print("  Chua cau hinh LLM (%s): mo pane Ask AI > Cai dat, hoac chay lai voi --endpoint/--model/--api-key."
          % ", ".join(missing))
EOF
}

systemd_available() {
    command -v systemctl >/dev/null 2>&1 && systemctl --user show-environment >/dev/null 2>&1
}

enable_systemd() {
    [ "$SYSTEMD" = 1 ] || return 0
    systemd_available || { echo "  (bo qua --systemd: khong co systemd --user trong phien nay)"; return 0; }
    mkdir -p "$UNIT_DIR"
    # Unit tro thang toi binary vua cai; %h khong dung duoc vi DATA_DIR co the bi doi bang XDG_DATA_HOME.
    sed "s|@CORE_EXE@|$CORE_DIR/AxiomOffice.Core|" "$HERE/axiom-office-core.service.in" > "$UNIT_DIR/$UNIT_NAME"
    systemctl --user daemon-reload
    systemctl --user enable --now "$UNIT_NAME"
    echo "  Agent Core chay thuong truc: systemctl --user status $UNIT_NAME"
}

disable_systemd() {
    if [ -f "$UNIT_DIR/$UNIT_NAME" ]; then
        systemd_available && systemctl --user disable --now "$UNIT_NAME" >/dev/null 2>&1 || true
        rm -f "$UNIT_DIR/$UNIT_NAME"
        systemd_available && systemctl --user daemon-reload >/dev/null 2>&1 || true
        echo "Da go unit $UNIT_NAME"
    fi
}

core_port() {
    # Cong Core trong cau hinh vua ghi (CorePort), mac dinh 47840.
    AXIOM_CFG="$CONFIG_DIR/config.json" python3 -c \
        'import json, os; print(json.load(open(os.environ["AXIOM_CFG"])).get("CorePort", 47840) or 47840)' \
        2>/dev/null || echo 47840
}

register_wps_addin() {
    # Dang ky add-in WPS khi may co WPS (khong bao gio dung Office.conf cua nguoi dung).
    [ "$NOWPS" = 1 ] && return 0
    [ -d "$WPS_ROOT" ] || return 0
    [ -f "$HERE/wps/install_jsaddon.py" ] || { echo "  (bo qua WPS: khong co wps/install_jsaddon.py)"; return 0; }
    PORT="$(core_port)"
    case "$PORT" in ''|*[!0-9]*) PORT=47840 ;; esac
    echo "WPS: dang ky add-in (Core http://127.0.0.1:$PORT)"
    python3 "$HERE/wps/install_jsaddon.py" --home "$HOME" --core-url "http://127.0.0.1:$PORT"
    OEM="$WPS_ROOT/office6/cfgs/oem.ini"
    if ! grep -q '^[[:space:]]*JsApiPlugin[[:space:]]*=[[:space:]]*true' "$OEM" 2>/dev/null; then
        echo "  WPS can bat JS add-in mot lan (quyen admin, JsApiPlugin=true trong $OEM):"
        echo "    sudo cp -n \"$OEM\" \"$OEM.bak\""
        echo "    sudo sed -i '/^\\[Support\\]/a JsApiPlugin=true' \"$OEM\""
    fi
    if [ "$SYSTEMD" != 1 ]; then
        echo "  WPS: mo WPS sau khi Core da chay (neu khong add-in bi bo qua); nen chay ./install.sh --systemd"
    fi
}

unregister_wps_addin() {
    # Go dang ky add-in WPS (giu add-in khac); chi khi co trinh cai trong goi.
    [ "$NOWPS" = 1 ] && return 0
    [ -f "$HERE/wps/install_jsaddon.py" ] || return 0
    python3 "$HERE/wps/install_jsaddon.py" --home "$HOME" --uninstall 2>/dev/null || true
    echo "Da go dang ky add-in WPS (giu add-in khac)"
}

install() {
    local oxt
    oxt="$(ls "$HERE"/AxiomOffice-LibreOffice-*.oxt 2>/dev/null | sort | tail -1)"
    [ -n "$oxt" ] || die "khong thay AxiomOffice-LibreOffice-*.oxt canh install.sh"
    [ -x "$HERE/core/AxiomOffice.Core" ] || die "khong thay core/AxiomOffice.Core canh install.sh"
    [ -x "$UNOPKG" ] || die "khong thay unopkg ($UNOPKG) - cai LibreOffice truoc (vd: sudo apt install libreoffice)."
    python3 -c 'import uno' >/dev/null 2>&1 \
        || die "thieu Python UNO: sudo apt install python3-uno (Debian/Ubuntu) hoac sudo dnf install libreoffice-pyuno (Fedora)."
    assert_libreoffice_closed

    echo "1/4 Agent Core -> $CORE_DIR"
    stop_core
    mkdir -p "$DATA_DIR"
    rm -rf "$CORE_DIR.new"
    cp -a "$HERE/core" "$CORE_DIR.new"
    rm -rf "$CORE_DIR"
    mv "$CORE_DIR.new" "$CORE_DIR"

    echo "2/4 Extension LibreOffice ($(basename "$oxt"))"
    "$UNOPKG" add --force "$oxt"

    echo "3/4 Cau hinh -> $CONFIG_DIR/config.json"
    write_config

    echo "4/4 MCP server: nam trong Core ($CORE_DIR/AxiomOffice.Core mcp)"
    if [ -d "$MCP_DIR" ]; then
        rm -rf "$MCP_DIR"
        echo "    (da don $MCP_DIR cua ban cai cu)"
    fi

    enable_systemd

    register_wps_addin

    echo
    echo "Xong. Mo LibreOffice Writer/Calc/Impress: menu Axiom Office > Ask AI (hoac sidebar)."
    if [ "$SYSTEMD" != 1 ] && systemd_available; then
        echo "(Core chay khi can; muon Core san sang tu dau phien: ./install.sh --systemd)"
    fi
    echo
    print_mcp_config
}

# Cau hinh MCP cho Claude Code / Claude Desktop (dung tool file + tool live tren LibreOffice dang mo).
print_mcp_config() {
    echo "Them MCP server (Claude Code: claude mcp add, hoac file cau hinh MCP):"
    echo "  {"
    echo "    \"mcpServers\": {"
    echo "      \"office\": { \"command\": \"$CORE_DIR/AxiomOffice.Core\", \"args\": [\"mcp\", \"all\"] }"
    echo "    }"
    echo "  }"
    echo "  (\"args\": [\"mcp\",\"word\"] / [\"mcp\",\"excel\"] / [\"mcp\",\"ppt\"] neu chi muon mot nhom tool)"
}

uninstall() {
    assert_libreoffice_closed
    disable_systemd
    stop_core
    unregister_wps_addin
    "$UNOPKG" remove "$EXTENSION_ID" 2>/dev/null && echo "Da go extension $EXTENSION_ID" || echo "(extension chua cai)"
    rm -rf "$CORE_DIR" "$MCP_DIR"
    echo "Da xoa $CORE_DIR va $MCP_DIR"
    if [ "$PURGE" = 1 ]; then
        rm -rf "$DATA_DIR" "$CONFIG_DIR" "${XDG_RUNTIME_DIR:-$HOME/.cache}/axiom-office"
        echo "Da xoa cau hinh va du lieu (hoi thoai, memory, log)."
    else
        echo "Giu cau hinh ($CONFIG_DIR) va du lieu ($DATA_DIR) - them --purge de xoa."
    fi
}

if [ "$UNINSTALL" = 1 ]; then uninstall; else install; fi

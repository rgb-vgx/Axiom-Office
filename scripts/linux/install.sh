#!/usr/bin/env bash
# Cai Axiom Office cho LibreOffice tren Linux (LibreOffice_arch.md giai doan L3) - khong can root.
#
#   ./install.sh [--endpoint URL] [--model NAME] [--api-key KEY|-] [--systemd]   cai / nang cap
#                 (--api-key - = doc khoa tu stdin; --systemd = bat Core chay thuong truc bang systemd --user)
#   ./install.sh --uninstall [--purge]                                           go (--purge xoa ca cau hinh, du lieu)
#
# Chay tu thu muc giai nen cua axiom-office-linux-x64-<ver>.tar.gz (co core/ va *.oxt canh script):
#   - Agent Core -> ~/.local/share/axiom-office/core (extension tu khoi dong khi can)
#   - extension -> unopkg cua nguoi dung
#   - cau hinh  -> ~/.config/axiom-office/config.json (quyen 0600; giu khoa cu, chi ghi khoa duoc truyen)
#   - MCP       -> ~/.local/share/axiom-office/mcp (in san doan cau hinh cho Claude Code/Desktop)
# Khong bao gio tat LibreOffice cua nguoi dung: dang chay thi dung lai va nhac dong.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/axiom-office"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/axiom-office"
CORE_DIR="$DATA_DIR/core"
MCP_DIR="$DATA_DIR/mcp"
EXTENSION_ID="org.axiomoffice.bridge"
UNIT_NAME="axiom-office-core.service"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
UNOPKG="${UNOPKG:-$(command -v unopkg || echo /usr/lib/libreoffice/program/unopkg)}"

ENDPOINT="" MODEL="" API_KEY="" UNINSTALL=0 PURGE=0 SYSTEMD=0
while [ $# -gt 0 ]; do
    case "$1" in
        --endpoint) ENDPOINT="$2"; shift 2 ;;
        --model) MODEL="$2"; shift 2 ;;
        --api-key) API_KEY="$2"; shift 2 ;;
        --systemd) SYSTEMD=1; shift ;;
        --uninstall) UNINSTALL=1; shift ;;
        --purge) PURGE=1; shift ;;
        -h|--help) sed -n '2,12p' "$0"; exit 0 ;;
        *) echo "Tham so la: $1 (xem --help)" >&2; exit 2 ;;
    esac
done
# --api-key - : doc khoa tu stdin (khong luu vao lich su shell / danh sach tien trinh)
if [ "$API_KEY" = "-" ]; then IFS= read -r API_KEY || true; fi

die() { echo "Loi: $*" >&2; exit 1; }

assert_libreoffice_closed() {
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

    if [ -x "$HERE/mcp/axiom-office-mcp" ]; then
        echo "4/4 MCP server -> $MCP_DIR"
        rm -rf "$MCP_DIR.new"
        cp -a "$HERE/mcp" "$MCP_DIR.new"
        rm -rf "$MCP_DIR"
        mv "$MCP_DIR.new" "$MCP_DIR"
    else
        echo "4/4 MCP server: khong co trong goi, bo qua"
    fi

    enable_systemd

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
    echo "      \"office\": { \"command\": \"$MCP_DIR/axiom-office-mcp\", \"args\": [\"all\"] }"
    echo "    }"
    echo "  }"
    echo "  (\"args\": [\"word\"] / [\"excel\"] / [\"ppt\"] neu chi muon mot nhom tool)"
}

uninstall() {
    assert_libreoffice_closed
    disable_systemd
    stop_core
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

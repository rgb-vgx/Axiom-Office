#!/usr/bin/env bash
# Dong goi ban Linux: dist/axiom-office-linux-x64-<ver>.tar.gz gom
#   core/ (Agent Core linux-x64 - binary Go, may dich khong can runtime + skills/ dung san),
#   mcp/ (axiom-office-mcp cho Claude Code/Desktop), AxiomOffice-LibreOffice-<ver>.oxt, install.sh, README.txt.
# Can Go 1.26+ (Agent Core) va .NET SDK 10 (MCP server). Chay duoc tren Linux hoac Git Bash/WSL.
# AXIOM_CORE=dotnet: dong goi Agent Core ban .NET thay vi ban Go.
#
#   scripts/linux/package.sh [--rid linux-x64|linux-arm64]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RID="linux-x64"
[ "${1:-}" = "--rid" ] && RID="$2"
DOTNET="${DOTNET:-$(command -v dotnet || echo "$HOME/.dotnet/dotnet")}"
PYTHON="$(command -v python3 || command -v python)"

VERSION="$(sed -n 's/^VERSION = "\(.*\)"/\1/p' "$ROOT/src/AxiomOffice.LibreOffice/python/pythonpath/axiom/__init__.py" | tr -d '\r')"
[ -n "$VERSION" ] || { echo "khong doc duoc VERSION" >&2; exit 1; }
NAME="axiom-office-${RID}-${VERSION}"
STAGE="$ROOT/dist/$NAME"

rm -rf "$STAGE"
mkdir -p "$STAGE"

# Agent Core: mac dinh ban Go (mot binary, may dich khong can .NET runtime); AXIOM_CORE=dotnet dung ban .NET.
if [ "${AXIOM_CORE:-go}" = "go" ]; then
    echo "== Agent Core (Go, $RID)"
    GO="${GO:-$(command -v go || echo /usr/local/go/bin/go)}"
    [ -x "$GO" ] || { echo "khong thay go (cai Go 1.26+ hoac dat GO=/duong/dan/go)" >&2; exit 1; }
    case "$RID" in
        *-arm64) GOARCH=arm64 ;;
        *) GOARCH=amd64 ;;
    esac
    mkdir -p "$STAGE/core"
    GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 "$GO" build -C "$ROOT/core-go" -trimpath \
        -ldflags "-s -w -X main.version=$VERSION" -o "$STAGE/core/AxiomOffice.Core" ./cmd/axiom-core
else
    echo "== Agent Core ($RID, self-contained .NET)"
    "$DOTNET" publish "$ROOT/src/AxiomOffice.Core" -c Release -r "$RID" --self-contained \
        -p:DebugType=none -o "$STAGE/core" -v q -nologo
fi
chmod +x "$STAGE/core/AxiomOffice.Core"

# Skill dung san: Core nap `skills/` canh binary truoc (nguon "builtin"), khong can cau hinh SkillDirs.
echo "== Skills"
mkdir -p "$STAGE/core/skills"
cp -a "$ROOT/skills/." "$STAGE/core/skills/"
find "$STAGE/core/skills" -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true

echo "== MCP server ($RID, self-contained)"
"$DOTNET" publish "$ROOT/src/AxiomOffice.Mcp" -c Release -r "$RID" --self-contained \
    -p:DebugType=none -o "$STAGE/mcp" -v q -nologo
chmod +x "$STAGE/mcp/axiom-office-mcp"

echo "== Extension LibreOffice"
"$PYTHON" "$ROOT/scripts/package_oxt.py" --out "$STAGE" >/dev/null

cp "$ROOT/scripts/linux/install.sh" "$STAGE/install.sh"
cp "$ROOT/scripts/linux/axiom-office-core.service.in" "$STAGE/axiom-office-core.service.in"
chmod +x "$STAGE/install.sh"
cat >"$STAGE/README.txt" <<EOF
Axiom Office $VERSION cho LibreOffice tren Linux ($RID)

Yeu cau: LibreOffice 7.x tro len + Python UNO (Debian/Ubuntu: sudo apt install libreoffice python3-uno).

Cai (khong can root; dong LibreOffice truoc):
    ./install.sh --endpoint http://localhost:20128/v1 --model <ten-model> --api-key -
    (--api-key - doc khoa tu stdin; bo qua cac tham so LLM thi cau hinh sau trong pane Ask AI > Cai dat)
    ./install.sh --systemd      # tuy chon: cho Agent Core chay thuong truc (systemd --user)

Dung: mo Writer/Calc/Impress -> menu Axiom Office > Ask AI (pane nam trong sidebar).
MCP cho Claude Code/Desktop: install.sh in san doan cau hinh mcpServers khi cai xong.
Go:  ./install.sh --uninstall [--purge]

Vi tri: Core ~/.local/share/axiom-office/core, MCP ~/.local/share/axiom-office/mcp,
        cau hinh ~/.config/axiom-office/config.json, log ~/.local/share/axiom-office/bridge.log
EOF

echo "== Tarball"
tar -C "$ROOT/dist" -czf "$ROOT/dist/$NAME.tar.gz" "$NAME"
echo "$ROOT/dist/$NAME.tar.gz ($(du -h "$ROOT/dist/$NAME.tar.gz" | cut -f1))"

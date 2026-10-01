#!/usr/bin/env bash
# Dong goi ban Linux: dist/axiom-office-linux-x64-<ver>.tar.gz gom
#   core/ (Agent Core linux-x64 - binary Go, may dich khong can runtime + skills/ dung san),
#   AxiomOffice-LibreOffice-<ver>.oxt, install.sh, README.txt.
# MCP server nam ngay trong core/: chay `core/AxiomOffice.Core mcp all`. Khong con .NET o day.
# Can Go 1.26+. Chay duoc tren Linux hoac Git Bash/WSL.
#
#   scripts/linux/package.sh [--rid linux-x64|linux-arm64]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RID="linux-x64"
[ "${1:-}" = "--rid" ] && RID="$2"
PYTHON="$(command -v python3 || command -v python)"

VERSION="$(sed -n 's/^VERSION = "\(.*\)"/\1/p' "$ROOT/src/AxiomOffice.LibreOffice/python/pythonpath/axiom/__init__.py" | tr -d '\r')"
[ -n "$VERSION" ] || { echo "khong doc duoc VERSION" >&2; exit 1; }
NAME="axiom-office-${RID}-${VERSION}"
STAGE="$ROOT/dist/$NAME"

rm -rf "$STAGE"
mkdir -p "$STAGE"

# Agent Core: mot binary Go, may dich khong can runtime (core-go/README.md).
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
chmod +x "$STAGE/core/AxiomOffice.Core"

# Skill dung san: Core nap `skills/` canh binary truoc (nguon "builtin"), khong can cau hinh SkillDirs.
echo "== Skills"
mkdir -p "$STAGE/core/skills"
cp -a "$ROOT/skills/." "$STAGE/core/skills/"
find "$STAGE/core/skills" -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true


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
MCP cho Claude Code/Desktop: dung chinh core/AxiomOffice.Core voi tham so "mcp all"; install.sh in san
doan cau hinh mcpServers khi cai xong.
Go:  ./install.sh --uninstall [--purge]

Vi tri: Core + MCP ~/.local/share/axiom-office/core,
        cau hinh ~/.config/axiom-office/config.json, log ~/.local/share/axiom-office/bridge.log
EOF

echo "== Tarball"
tar -C "$ROOT/dist" -czf "$ROOT/dist/$NAME.tar.gz" "$NAME"
echo "$ROOT/dist/$NAME.tar.gz ($(du -h "$ROOT/dist/$NAME.tar.gz" | cut -f1))"

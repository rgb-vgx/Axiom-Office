"""Sinh bang "lenh registry chua co handler o add-in WPS" va "tool Core bi an tren WPS" tu nguon that.

Truoc 31/10/2026 cac con so nay duoc chep tay vao STATUS.md va docs/architecture/wps-bridge.md roi lech dan (STATUS
ghi 26 muc, thuc te 51; wps-bridge.md ghi registry 84 lenh, thuc te 115). Nay chung sinh tu:
  - `catalog/live-commands.json` - registry lenh bridge (nguon chuan);
  - `CHUA_CO_HANDLER` trong `tests/wps/test_addin.mjs` - lenh add-in WPS chua co handler, kem ly do (spec do neu
    danh sach lech handler that);
  - `core-go/internal/tools/needs.go` - lenh BAT BUOC cua tung tool goi bridge truc tiep (Core an tool thieu lenh).

Ghi vao vung giua hai moc BEGIN/END cua:
  - `docs/architecture/wps-bridge.md` (muc 6) - bang day du;
  - `STATUS.md` - mot dong tom tat.

    python3 scripts/generate_wps_gaps.py [--check]
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
CATALOG = os.path.join(ROOT, "catalog", "live-commands.json")
SPEC = os.path.join(ROOT, "tests", "wps", "test_addin.mjs")
NEEDS = os.path.join(ROOT, "core-go", "internal", "tools", "needs.go")
DOC = os.path.join(ROOT, "docs", "architecture", "wps-bridge.md")
STATUS = os.path.join(ROOT, "STATUS.md")

BEGIN = "<!-- BEGIN SINH TU DONG (scripts/generate_wps_gaps.py) - KHONG SUA TAY -->"
END = "<!-- END SINH TU DONG -->"

APP_NAMES = {"wps": "Writer (`writer.*`)", "et": "Spreadsheets (`et.*`)", "wpp": "Presentation (`wpp.*`)",
             "": "Dùng chung (`kind` rỗng)"}


def missing_handlers() -> dict:
    """{ten lenh: ly do} tu khoi `const CHUA_CO_HANDLER = { ... };`."""
    with open(SPEC, encoding="utf-8") as handle:
        text = handle.read()
    block = re.search(r"^const CHUA_CO_HANDLER = \{\n(.*?)^\};", text, re.S | re.M)
    if not block:
        raise SystemExit("khong tim thay CHUA_CO_HANDLER trong " + SPEC)
    entries = {}
    for line in block.group(1).splitlines():
        line = line.strip()
        if not line or line.startswith("//"):
            continue
        match = re.match(r'^"([\w.]+)":\s*"((?:[^"\\]|\\.)*)",?$', line)
        if not match:
            raise SystemExit("dong khong doc duoc trong CHUA_CO_HANDLER: " + line)
        entries[match.group(1)] = match.group(2)
    return entries


def tool_needs() -> dict:
    """{ten tool: [lenh Required]} tu cac ham Commands() trong needs.go."""
    with open(NEEDS, encoding="utf-8") as handle:
        text = handle.read()
    tools = {}
    pattern = re.compile(r"// Commands: (\S+) .*?\nfunc \(t \*\w+\) Commands\(\) CommandNeeds \{(.*?)\n\}", re.S)
    for match in pattern.finditer(text):
        name, body = match.group(1), match.group(2)
        required = re.search(r"Required:\s*\[\]string\{(.*?)\}", body, re.S)
        if not required:
            raise SystemExit("khong doc duoc Required cua " + name)
        tools[name] = re.findall(r'"([\w.]+)"', required.group(1))
    if not tools:
        raise SystemExit("khong tim thay ham Commands() nao trong " + NEEDS)
    return tools


def render_doc(catalog: list, gaps: dict, needs: dict) -> str:
    kinds = {item["name"]: item.get("kind") or "" for item in catalog}
    agent = {item["name"]: bool(item.get("agent")) for item in catalog}
    unknown = sorted(set(gaps) - set(kinds))
    if unknown:
        raise SystemExit("CHUA_CO_HANDLER co lenh khong nam trong catalog: " + ", ".join(unknown))
    lines = [BEGIN, ""]
    lines.append("Registry có **%d** lệnh; add-in WPS có handler cho **%d**, còn **%d** chưa có. Add-in chỉ khai với Core "
                 "những lệnh có handler (`filterRunnable` trong `main.js`), nên agent **không** được mời gọi lệnh chưa có."
                 % (len(catalog), len(catalog) - len(gaps), len(gaps)))
    lines.append("")
    for kind in ("wps", "et", "wpp", ""):
        names = sorted(name for name in gaps if kinds[name] == kind)
        if not names:
            continue
        lines.append("**%s — %d lệnh**" % (APP_NAMES[kind], len(names)))
        lines.append("")
        lines.append("| Lệnh | Cho agent | Lý do (từ `CHUA_CO_HANDLER`) |")
        lines.append("|---|---|---|")
        for name in names:
            reason = gaps[name].replace("|", "\\|")
            lines.append("| `%s` | %s | %s |" % (name, "có" if agent[name] else "không", reason))
        lines.append("")
    lines.append("**Tool của Core bị ẩn trên WPS** (thiếu lệnh bắt buộc theo `core-go/internal/tools/needs.go`):")
    lines.append("")
    lines.append("| Tool | Lệnh bắt buộc WPS chưa có |")
    lines.append("|---|---|")
    hidden = 0
    for name in sorted(needs):
        lacking = [command for command in needs[name] if command in gaps]
        if lacking:
            hidden += 1
            lines.append("| `%s` | %s |" % (name, ", ".join("`%s`" % command for command in lacking)))
    if not hidden:
        lines.append("| — | — |")
    lines.append("")
    lines.append(END)
    return "\n".join(lines)


def render_status(catalog: list, gaps: dict, needs: dict) -> str:
    hidden = sorted(name for name, required in needs.items() if any(command in gaps for command in required))
    return "%s\n%d/%d lệnh registry có handler trên WPS (%d chưa có); tool Core bị ẩn trên WPS vì thiếu lệnh: %s. " \
           "Bảng đầy đủ: `docs/architecture/wps-bridge.md` §6.\n%s" % (
               BEGIN, len(catalog) - len(gaps), len(catalog), len(gaps),
               ", ".join("`%s`" % name for name in hidden) or "không", END)


def replace_block(path: str, block: str) -> tuple:
    with open(path, encoding="utf-8") as handle:
        text = handle.read()
    start, end = text.find(BEGIN), text.find(END)
    if start < 0 or end < start:
        raise SystemExit("thieu moc BEGIN/END trong " + path)
    updated = text[:start] + block + text[end + len(END):]
    return text, updated


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="chi kiem, khong ghi (CI)")
    args = parser.parse_args()
    with open(CATALOG, encoding="utf-8") as handle:
        catalog = json.load(handle)
    gaps, needs = missing_handlers(), tool_needs()
    stale = []
    for path, block in ((DOC, render_doc(catalog, gaps, needs)), (STATUS, render_status(catalog, gaps, needs))):
        old, new = replace_block(path, block)
        if old == new:
            continue
        if args.check:
            stale.append(os.path.relpath(path, ROOT))
        else:
            with open(path, "w", encoding="utf-8") as handle:
                handle.write(new)
            print("da ghi " + os.path.relpath(path, ROOT))
    if stale:
        print("lech nguon (chay python3 scripts/generate_wps_gaps.py): " + ", ".join(stale))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

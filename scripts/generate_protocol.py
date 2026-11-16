"""Sinh cac ban dung chung tu manifest su kien Core.

Nguon duy nhat: `core-go/internal/protocol/events.json` (sua tay). File do cung
duoc nhung vao Agent Core Go qua `//go:embed` (core-go/internal/protocol/manifest.go).

Ra cac ban sinh, KHONG sua tay:
  - `src/AxiomOffice.WPS/protocol_gen.js` - bien toan cuc ES5 `AxiomProtocol.EVENTS`.
  - `tests/core/protocol_gen.py` - dict `EVENTS` cho test Python 3.12.

    python3 scripts/generate_protocol.py [--check]
"""
from __future__ import annotations

import argparse
import json
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
SOURCE = os.path.join(ROOT, "core-go", "internal", "protocol", "events.json")
JS = os.path.join(ROOT, "src", "AxiomOffice.WPS", "protocol_gen.js")
PY = os.path.join(ROOT, "tests", "core", "protocol_gen.py")

HEADER = ("// SINH TU DONG tu core-go/internal/protocol/events.json bang "
          "scripts/generate_protocol.py - KHONG sua tay.\n")
HEADER_PY = ('"""SINH TU DONG tu core-go/internal/protocol/events.json bang '
             'scripts/generate_protocol.py - KHONG sua tay."""\n\n')


def json_string(value) -> str:
    # JSON escape la tap con cua string literal ca JS lan Python, nen dung
    # json.dumps de khong phai tu noi chuoi bang tay.
    return json.dumps(value, ensure_ascii=False)


def load() -> list:
    with open(SOURCE, encoding="utf-8") as handle:
        events = json.load(handle)
    seen = set()
    for event in events:
        name = event.get("type")
        if not name:
            raise SystemExit("events.json co muc thieu 'type'")
        if name in seen:
            raise SystemExit("events.json trung 'type': " + name)
        seen.add(name)
        if "summary" not in event:
            raise SystemExit("events.json thieu 'summary' cho " + name)
        if event.get("payload") is None:
            event["payload"] = {}
    return events


def render_js(events: list) -> str:
    lines = [HEADER,
             "// ES5 output: chi var va object literal.\n",
             "var AxiomProtocol = AxiomProtocol || {};\n",
             "AxiomProtocol.EVENTS = {"]
    for index, event in enumerate(events):
        comma = "," if index < len(events) - 1 else ""
        lines.append("  %s: {" % json_string(event["type"]))
        lines.append("    \"summary\": %s," % json_string(event["summary"]))
        payload = list(event["payload"].items())
        if payload:
            lines.append("    \"payload\": {")
            for field_index, (field, field_type) in enumerate(payload):
                field_comma = "," if field_index < len(payload) - 1 else ""
                lines.append("      %s: %s%s" % (json_string(field), json_string(field_type), field_comma))
            lines.append("    }")
        else:
            lines.append("    \"payload\": {}")
        lines.append("  }" + comma)
    lines.append("};")
    return "\n".join(lines) + "\n"


def render_py(events: list) -> str:
    lines = ["EVENTS = {"]
    for event in events:
        lines.append("    %s: {" % json_string(event["type"]))
        lines.append("        \"summary\": %s," % json_string(event["summary"]))
        payload = list(event["payload"].items())
        if payload:
            lines.append("        \"payload\": {")
            for field, field_type in payload:
                lines.append("            %s: %s," % (json_string(field), json_string(field_type)))
            lines.append("        },")
        else:
            lines.append("        \"payload\": {},")
        lines.append("    },")
    lines.append("}")
    return HEADER_PY + "\n".join(lines) + "\n"


def read_text(path: str) -> str:
    if not os.path.exists(path):
        return ""
    with open(path, encoding="utf-8", newline="") as handle:
        return handle.read().replace("\r\n", "\n")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="chi kiem, khong ghi (CI)")
    args = parser.parse_args()

    events = load()
    stale = []
    for path, rendered in ((JS, render_js(events)), (PY, render_py(events))):
        relative = os.path.relpath(path, ROOT)
        if read_text(path) == rendered:
            continue
        if args.check:
            stale.append(relative)
            continue
        with open(path, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(rendered)
        print("da ghi " + relative)

    if stale:
        print("lech nguon (chay python3 scripts/generate_protocol.py): " + ", ".join(stale))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

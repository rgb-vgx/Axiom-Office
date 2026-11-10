"""Sinh khoi <contextMenus> cua ribbon.xml WPS tu bang QuickActions cua Core.

Nguon duy nhat: `core-go/internal/agent/quickactions.go` (sua tay). Hai menu chuot phai (ContextMenuText,
ContextMenuList) truoc day chep tay giong het nhau, moi muc lap lai nhan cua bang Core: them mot lenh nhanh
phai sua ba cho. Nay khoi <contextMenus> nam giua hai moc BEGIN/END SINH TU DONG va duoc sinh;
`make check-gen` bao lech.

Quy tac dung (giong ban viet tay, tests/wps/test_pane.mjs ghim):
  - Moi contextMenu (idMso ContextMenuText hau to "t", ContextMenuList hau to "l") co menu "Sua bang AI"
    (id axiomCtxQuick_<s>) roi nut "Hoi AI ve doan nay" (axiomCtxAsk_<s>).
  - Trong menu: cac muc theo DUNG thu tu cua Core; cac muc Group "tone" gom vao menu con "Doi giong"
    (axiomCtxTone_<s>) dat o vi tri muc tone dau tien; vach ngan axiomCtxQuickSep_<s> dat ngay truoc muc
    khong Edits dau tien. Group khac "" va "tone" -> ValueError.

    python3 scripts/generate_ribbon_menus.py [--check] [--ribbon PATH]
"""
from __future__ import annotations

import argparse
import os
import re
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

GO = os.path.join(ROOT, "core-go", "internal", "agent", "quickactions.go")
RIBBON = os.path.join(ROOT, "src", "AxiomOffice.WPS", "ribbon.xml")

BEGIN = "  <!-- BEGIN SINH TU DONG (scripts/generate_ribbon_menus.py) - KHONG SUA TAY -->"
END = "  <!-- END SINH TU DONG -->"

GO_HEADER = "var QuickActions = []QuickAction{"
GO_FOOTER = "\n}\n"

ENTRY_RE = re.compile(r"^[ \t]*\{ID:", re.MULTILINE)
ID_RE = re.compile(r'ID:\s*"((?:[^"\\]|\\.)*)"')
LABEL_RE = re.compile(r'Label:\s*"((?:[^"\\]|\\.)*)"')
EDITS_RE = re.compile(r"Edits:\s*true\b")
GROUP_RE = re.compile(r'Group:\s*"((?:[^"\\]|\\.)*)"')


def parse_quick_actions(go_source: str) -> list[dict]:
    # Chi doc bang QuickActions; bo qua DocumentActions va dong comment.
    start = go_source.find(GO_HEADER)
    if start < 0:
        raise ValueError("khong tim thay %s trong ma Go" % GO_HEADER)
    start += len(GO_HEADER)
    stop = go_source.find(GO_FOOTER, start)
    if stop < 0:
        raise ValueError("khong tim thay dong dong bang QuickActions trong ma Go")
    body = go_source[start:stop]
    lines = [line for line in body.splitlines() if not line.lstrip().startswith("//")]
    body = "\n".join(lines)

    starts = [match.start() for match in ENTRY_RE.finditer(body)]
    actions = []
    for index, begin in enumerate(starts):
        end = starts[index + 1] if index + 1 < len(starts) else len(body)
        entry = body[begin:end]
        id_match = ID_RE.search(entry)
        label_match = LABEL_RE.search(entry)
        if not id_match or not label_match:
            raise ValueError("muc QuickActions thieu ID hoac Label: %r" % entry.strip())
        group_match = GROUP_RE.search(entry)
        actions.append({
            "id": id_match.group(1),
            "label": label_match.group(1),
            "edits": bool(EDITS_RE.search(entry)),
            "group": group_match.group(1) if group_match else "",
        })
    return actions


def render_button(action: dict, suffix: str, indent: str) -> str:
    return '%s<button id="axiomQuick_%s_%s" label="%s" tag="quick:%s" onAction="ribbon.OnAction"/>\n' % (
        indent, action["id"], suffix, action["label"], action["id"])


def render_menu(actions: list[dict], suffix: str, idmso: str) -> str:
    tone = [action for action in actions if action["group"] == "tone"]
    for action in actions:
        if action["group"] not in ("", "tone"):
            raise ValueError("muc %r co Group la %r (chi nhan \"\" va \"tone\")" % (
                action["id"], action["group"]))

    lines = ['    <contextMenu idMso="%s">\n' % idmso]
    lines.append('      <menu id="axiomCtxQuick_%s" label="Sửa bằng AI" tag="quickMenu" '
                 'getImage="ribbon.OnGetImage" insertBeforeMso="Cut">\n' % suffix)
    tone_done = False
    separator_done = False
    for action in actions:
        if action["group"] == "tone":
            if tone_done:
                continue
            tone_done = True
            lines.append('        <menu id="axiomCtxTone_%s" label="Đổi giọng">\n' % suffix)
            for item in tone:
                lines.append(render_button(item, suffix, "          "))
            lines.append("        </menu>\n")
            continue
        if not action["edits"] and not separator_done:
            separator_done = True
            lines.append('        <menuSeparator id="axiomCtxQuickSep_%s"/>\n' % suffix)
        lines.append(render_button(action, suffix, "        "))
    lines.append("      </menu>\n")
    lines.append('      <button id="axiomCtxAsk_%s" label="Hỏi AI về đoạn này" tag="askAi" '
                 'onAction="ribbon.OnAction" getImage="ribbon.OnGetImage" insertBeforeMso="Cut"/>\n' % suffix)
    lines.append("    </contextMenu>\n")
    return "".join(lines)


def render_context_menus(actions: list[dict]) -> str:
    return (
        "  <contextMenus>\n"
        + render_menu(actions, "t", "ContextMenuText")
        + render_menu(actions, "l", "ContextMenuList")
        + "  </contextMenus>\n"
    )


def splice(path: str, text: str, block: str) -> str:
    if text.count(BEGIN) != 1:
        raise ValueError("khong tim thay moc BEGIN dung 1 lan trong %s (tim thay %d lan)" % (
            path, text.count(BEGIN)))
    if text.count(END) != 1:
        raise ValueError("khong tim thay moc END dung 1 lan trong %s (tim thay %d lan)" % (
            path, text.count(END)))
    begin_at = text.index(BEGIN) + len(BEGIN)
    end_at = text.index(END)
    if end_at < begin_at:
        raise ValueError("moc END nam truoc moc BEGIN trong %s" % path)
    return text[:begin_at] + "\n" + block + text[end_at:]


def read_text(path: str) -> str:
    with open(path, encoding="utf-8", newline="") as handle:
        return handle.read()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="chi kiem tra ribbon.xml co khop Core khong")
    parser.add_argument("--ribbon", default=RIBBON, help="duong dan ribbon.xml (mac dinh: %(default)s)")
    args = parser.parse_args()

    with open(GO, encoding="utf-8") as handle:
        actions = parse_quick_actions(handle.read())
    text = read_text(args.ribbon)
    try:
        expected = splice(args.ribbon, text, render_context_menus(actions))
    except ValueError as err:
        print("LOI: %s" % err, file=sys.stderr)
        return 2

    if args.check:
        if expected != text:
            print("%s lech, chay python3 scripts/generate_ribbon_menus.py" % os.path.basename(args.ribbon),
                  file=sys.stderr)
            return 1
        print("khop: %s" % args.ribbon)
        return 0

    if expected != text:
        with open(args.ribbon, "w", encoding="utf-8", newline="") as handle:
            handle.write(expected)
        print("da ghi %s" % args.ribbon)
    else:
        print("khop: %s" % args.ribbon)
    return 0


if __name__ == "__main__":
    sys.exit(main())

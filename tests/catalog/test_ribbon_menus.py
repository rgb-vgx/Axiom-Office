"""Spec: scripts/generate_ribbon_menus.py sinh khoi <contextMenus> cua ribbon.xml WPS tu bang QuickActions cua Core.

    python3 -m unittest tests/catalog/test_ribbon_menus.py -v

Truoc day hai menu chuot phai (ContextMenuText, ContextMenuList) chep tay giong het nhau, moi muc lap lai nhan cua
core-go/internal/agent/quickactions.go: them mot lenh nhanh phai sua 3 cho. Nay khoi nam giua hai moc
BEGIN/END SINH TU DONG va duoc sinh; `make check-gen` bao lech.

Quy tac dung (giong ban viet tay hien co, tests/wps/test_pane.mjs ghim):
  - Moi contextMenu (idMso ContextMenuText hau to "t", ContextMenuList hau to "l") co menu "Sua bang AI"
    (id axiomCtxQuick_<s>) roi nut "Hoi AI ve doan nay" (axiomCtxAsk_<s>).
  - Trong menu: cac muc theo DUNG thu tu cua Core; cac muc Group "tone" gom vao menu con "Doi giong"
    (axiomCtxTone_<s>) dat o vi tri muc tone dau tien; vach ngan axiomCtxQuickSep_<s> dat ngay truoc muc khong
    Edits dau tien. Group khac "" va "tone" -> ValueError.
"""
import importlib.util
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
SCRIPT = os.path.join(ROOT, "scripts", "generate_ribbon_menus.py")
RIBBON = os.path.join(ROOT, "src", "AxiomOffice.WPS", "ribbon.xml")
BEGIN = "<!-- BEGIN SINH TU DONG (scripts/generate_ribbon_menus.py) - KHONG SUA TAY -->"
END = "<!-- END SINH TU DONG -->"


def load():
    spec = importlib.util.spec_from_file_location("generate_ribbon_menus", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


GO = '''package agent

var QuickActions = []QuickAction{
	{ID: "polish", Label: "Trau chuốt", Edits: true, Prompt: "a " +
		"b" + quickEditTail},
	// Nhom "Doi giong"
	{ID: "formal", Label: "Trang trọng hơn", Edits: true, Group: "tone", Prompt: "c"},
	{ID: "plain", Label: "Dễ hiểu hơn", Edits: true, Group: "tone", Prompt: "d"},
	{ID: "continue", Label: "Viết tiếp", Edits: true, Prompt: "e"},
	{ID: "summarize", Label: "Tóm tắt", Prompt: "f"},
}

var DocumentActions = []QuickAction{
	{ID: "proofread", Label: "Soát chính tả cả bài", Edits: true, Document: true, Prompt: "g"},
}
'''

ACTIONS = [
    {"id": "polish", "label": "Trau chuốt", "edits": True, "group": ""},
    {"id": "formal", "label": "Trang trọng hơn", "edits": True, "group": "tone"},
    {"id": "plain", "label": "Dễ hiểu hơn", "edits": True, "group": "tone"},
    {"id": "continue", "label": "Viết tiếp", "edits": True, "group": ""},
    {"id": "summarize", "label": "Tóm tắt", "edits": False, "group": ""},
]


def expected_menu(suffix, idmso):
    return (
        '    <contextMenu idMso="%(m)s">\n'
        '      <menu id="axiomCtxQuick_%(s)s" label="Sửa bằng AI" tag="quickMenu" getImage="ribbon.OnGetImage" insertBeforeMso="Cut">\n'
        '        <button id="axiomQuick_polish_%(s)s" label="Trau chuốt" tag="quick:polish" onAction="ribbon.OnAction"/>\n'
        '        <menu id="axiomCtxTone_%(s)s" label="Đổi giọng">\n'
        '          <button id="axiomQuick_formal_%(s)s" label="Trang trọng hơn" tag="quick:formal" onAction="ribbon.OnAction"/>\n'
        '          <button id="axiomQuick_plain_%(s)s" label="Dễ hiểu hơn" tag="quick:plain" onAction="ribbon.OnAction"/>\n'
        '        </menu>\n'
        '        <button id="axiomQuick_continue_%(s)s" label="Viết tiếp" tag="quick:continue" onAction="ribbon.OnAction"/>\n'
        '        <menuSeparator id="axiomCtxQuickSep_%(s)s"/>\n'
        '        <button id="axiomQuick_summarize_%(s)s" label="Tóm tắt" tag="quick:summarize" onAction="ribbon.OnAction"/>\n'
        '      </menu>\n'
        '      <button id="axiomCtxAsk_%(s)s" label="Hỏi AI về đoạn này" tag="askAi" onAction="ribbon.OnAction" getImage="ribbon.OnGetImage" insertBeforeMso="Cut"/>\n'
        '    </contextMenu>\n'
    ) % {"s": suffix, "m": idmso}


class ParseTest(unittest.TestCase):
    def test_reads_only_the_quick_actions_table_in_order(self):
        self.assertEqual(load().parse_quick_actions(GO), ACTIONS)

    def test_reads_the_real_core_table(self):
        with open(os.path.join(ROOT, "core-go", "internal", "agent", "quickactions.go"), encoding="utf-8") as handle:
            actions = load().parse_quick_actions(handle.read())
        self.assertEqual(len(actions), 15)
        self.assertEqual(actions[0], {"id": "polish", "label": "Trau chuốt", "edits": True, "group": ""})
        self.assertEqual([a["id"] for a in actions if a["group"] == "tone"],
                         ["formal", "academic", "concise", "plain", "positive"])


class RenderTest(unittest.TestCase):
    def test_renders_both_context_menus(self):
        expected = "  <contextMenus>\n" + expected_menu("t", "ContextMenuText") + \
            expected_menu("l", "ContextMenuList") + "  </contextMenus>\n"
        self.assertEqual(load().render_context_menus(ACTIONS), expected)

    def test_unknown_group_is_an_error(self):
        bad = ACTIONS + [{"id": "x", "label": "X", "edits": True, "group": "mood"}]
        with self.assertRaises(ValueError):
            load().render_context_menus(bad)


class CheckTest(unittest.TestCase):
    def run_script(self, *args):
        return subprocess.run([sys.executable, SCRIPT, *args], capture_output=True, text=True, timeout=60)

    def test_repo_ribbon_has_markers_and_is_in_sync(self):
        with open(RIBBON, encoding="utf-8") as handle:
            text = handle.read()
        self.assertEqual(text.count(BEGIN), 1)
        self.assertEqual(text.count(END), 1)
        # Phan tu that (thut le 2, ca dong) - chu "<contextMenus>" con xuat hien trong chu thich dau tep.
        self.assertLess(text.index(BEGIN), text.index("\n  <contextMenus>\n"))
        self.assertGreater(text.index(END), text.index("\n  </contextMenus>\n"))
        result = self.run_script("--check")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_check_reports_drift_and_generate_fixes_it(self):
        with tempfile.TemporaryDirectory() as tmp:
            copy = os.path.join(tmp, "ribbon.xml")
            shutil.copy(RIBBON, copy)
            with open(copy, encoding="utf-8") as handle:
                text = handle.read()
            drifted = text.replace('id="axiomQuick_polish_l" label="Trau chuốt"', 'id="axiomQuick_polish_l" label="Trau chuot"')
            self.assertNotEqual(drifted, text)
            with open(copy, "w", encoding="utf-8") as handle:
                handle.write(drifted)
            result = self.run_script("--check", "--ribbon", copy)
            self.assertNotEqual(result.returncode, 0, "lech nhan phai bi --check bao")
            self.assertIn("ribbon.xml", result.stdout + result.stderr)
            result = self.run_script("--ribbon", copy)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            with open(copy, encoding="utf-8") as handle:
                self.assertEqual(handle.read(), text, "sinh lai phai ra dung ban dong bo, ngoai khoi khong doi gi")

    def test_check_gen_runs_it(self):
        with open(os.path.join(ROOT, "Makefile"), encoding="utf-8") as handle:
            makefile = handle.read()
        block = makefile[makefile.index("check-gen:"):]
        block = block[:block.index("\n\n")]
        self.assertIn("python3 scripts/generate_ribbon_menus.py --check", block)


if __name__ == "__main__":
    unittest.main()

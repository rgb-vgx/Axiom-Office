"""Unit test cho extension LibreOffice (src\\AxiomOffice.LibreOffice) - chạy được KHÔNG cần LibreOffice.

Hai phần, đều là thứ dễ lệch khi sửa tay:
1. Giải mã tham số của axiom.values (port từ CommandDispatcher.Params.cs): model hay gửi số dạng chuỗi,
   mảng bọc {"item": ...} nhiều lớp, values là chuỗi JSON... Bảng giá trị ở đây lấy đúng các ca trong
   tests\\live\\test_live_commands.py để hai làn xử lý giống nhau.
2. Registry lệnh (axiom.commands) phải khớp bản C#: so trực tiếp với `AxiomOffice.Host.exe commands --json`
   khi có EXE đã build (bỏ qua nếu chưa build).

    python tests\\lo\\test_extension.py
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import types
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
HOST_EXE = os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.Host.exe")

# Extension chay trong LibreOffice nen import UNO; o day thay bang module gia de test phan thuan Python.
if "uno" not in sys.modules:
    _uno = types.ModuleType("uno")
    _uno.createUnoStruct = lambda name: types.SimpleNamespace()
    _uno.Enum = lambda kind, name: name
    _uno.invoke = lambda obj, name, args: getattr(obj, name)(*args)
    _uno.Any = lambda kind, value: value
    _uno.fileUrlToSystemPath = lambda url: url
    _uno.systemPathToFileUrl = lambda path: path
    _uno.getConstantByName = lambda name: 0
    sys.modules["uno"] = _uno

sys.path.insert(0, PYTHONPATH)
from axiom import commands, values  # noqa: E402

commands.load_all()


class ParamTests(unittest.TestCase):
    """Cùng bảng ca với bản Windows (xem et.writeRange trong tests\\live\\test_live_commands.py)."""

    def test_matrix_plain_rows(self):
        rows = values.matrix({"values": [["Tên", "Điểm"], ["An", 9.5]]}, "values", True)
        self.assertEqual(rows, [["Tên", "Điểm"], ["An", 9.5]])

    def test_matrix_unwraps_item_wrappers(self):
        wrapped = {"item": [{"item": ["Tổng", "=SUM(B2:B3)"]}]}
        self.assertEqual(values.matrix({"values": wrapped}, "values", True), [["Tổng", "=SUM(B2:B3)"]])

    def test_matrix_double_wrapped_rows(self):
        double = {"item": [{"item": {"item": ["STT", "Tên"]}}, {"item": {"item": ["1", "An"]}}]}
        self.assertEqual(values.matrix({"values": double}, "values", True), [["STT", "Tên"], ["1", "An"]])

    def test_matrix_json_string(self):
        self.assertEqual(values.matrix({"values": "[[1,2],[3,4]]"}, "values", True), [[1, 2], [3, 4]])
        self.assertEqual(values.matrix({"values": '{"item": [["j", 1]]}'}, "values", True), [["j", 1]])

    def test_matrix_one_dimensional_is_one_column(self):
        self.assertEqual(values.matrix({"values": [1, 2, 3]}, "values", True), [[1], [2], [3]])

    def test_matrix_cell_wrapper(self):
        self.assertEqual(values.matrix({"values": [[{"item": "c1"}, "c2"]]}, "values", True), [["c1", "c2"]])

    def test_matrix_optional_empty(self):
        self.assertEqual(values.matrix({}, "values", False), None)
        self.assertEqual(values.matrix({"values": []}, "values", False), [])

    def test_matrix_unwraps_one_more_row_level(self):
        # Thêm một lớp bọc quanh cả DÒNG cũng được gỡ (giống ParamMatrix bản C#): [[["a","b"]]] -> [["a","b"]].
        self.assertEqual(values.matrix({"values": [[["a", "b"]]]}, "values", True), [["a", "b"]])

    def test_matrix_rejects_nested_cell_and_mix(self):
        with self.assertRaises(values.ParamError):
            values.matrix({"values": [["a", ["b", "c"]]]}, "values", True)
        with self.assertRaises(values.ParamError):
            values.matrix({"values": [{"a": 1}]}, "values", True)
        with self.assertRaises(values.ParamError):
            values.matrix({"values": [["a"], "b"]}, "values", True)

    def test_matrix_missing_required(self):
        with self.assertRaises(values.ParamError) as raised:
            values.matrix({}, "values", True)
        self.assertIn("must be a JSON 2D array", str(raised.exception))
        self.assertIn("missing", str(raised.exception))

    def test_scalars(self):
        self.assertEqual(values.string({"text": 12}, "text"), "12")
        self.assertEqual(values.string({"text": {"item": "x"}}, "text"), "x")
        self.assertEqual(values.string({}, "text", "d"), "d")
        self.assertEqual(values.integer({"maxChars": "12.0"}, "maxChars", 0), 12)
        self.assertEqual(values.integer({"rows": {"item": 2}}, "rows", 0), 2)
        self.assertEqual(values.integer({"cols": [3]}, "cols", 0), 3)
        self.assertEqual(values.number({"size": "11.5"}, "size", 0), 11.5)
        self.assertTrue(values.boolean({"bold": "true"}, "bold", False))
        self.assertFalse(values.boolean({"bold": 0}, "bold", True))

    def test_scalar_errors_name_the_parameter(self):
        with self.assertRaises(values.ParamError) as raised:
            values.boolean({"bold": "có"}, "bold", False)
        self.assertEqual(str(raised.exception), "'bold' must be true or false, got 'có'")
        with self.assertRaises(values.ParamError) as raised:
            values.integer({"count": "nhiều"}, "count", 1)
        self.assertIn("'count' must be a whole number", str(raised.exception))

    def test_has_ignores_none(self):
        self.assertFalse(values.has({"color": None}, "color"))
        self.assertTrue(values.has({"color": "#FF0000"}, "color"))

    def test_color_is_rgb_not_bgr(self):
        self.assertEqual(values.color("#C00000"), 0xC00000)
        self.assertEqual(values.color("0000FF"), 0x0000FF)
        self.assertIsNone(values.color("đỏ"))
        self.assertIsNone(values.color(None))


class CatalogTests(unittest.TestCase):
    def test_registry_shape(self):
        catalog = commands.catalog()
        self.assertEqual(len(catalog), len(commands.REGISTRY))
        for item in catalog:
            self.assertEqual(set(item), {"name", "kind", "agent", "summary", "params"})
            self.assertTrue(item["summary"])
            self.assertIn(item["kind"], (None, "wps", "et", "wpp"))
            for param in item["params"]:
                self.assertEqual(set(param), {"name", "required", "hint"})

    def test_mcp_live_commands_khop_registry(self):
        """`catalog/live-commands.json` (nguồn duy nhất) phải khớp registry này.

        Bản Windows lấy danh sách lệnh từ CommandCatalog của add-in; bản chạy độc lập (MCP server .NET 10
        cũ, MCP server Go) không có add-in nên nhúng sẵn file này - lệch nghĩa là mô tả tool `*_command`
        nói sai việc bridge làm được. Agent Core bản Go nhúng qua `livecommands_gen.go` (sinh từ file này).
        """
        path = os.path.join(ROOT, "catalog", "live-commands.json")
        with open(path, encoding="utf-8", newline="") as handle:
            text = handle.read().replace("\r\n", "\n")
        expected = [{"name": item["name"], "kind": item["kind"],
                     "params": [{"name": p["name"], "hint": p["hint"]} for p in item["params"]]}
                    for item in commands.catalog()]
        self.assertEqual(json.loads(text), expected, "chạy lại scripts/generate_mcp_commands.py")
        self.assertEqual(text, json.dumps(expected, ensure_ascii=False, indent=2) + "\n",
                         "file phải đúng định dạng của scripts/generate_mcp_commands.py")

    def test_every_kind_has_agent_actions(self):
        # Ten lenh mang tien to app (writer./et./wpp.), kind la ma port dung chung voi Office/WPS (wps/et/wpp).
        for kind, prefix in (("wps", "writer."), ("et", "et."), ("wpp", "wpp.")):
            agent = [c["name"] for c in commands.catalog() if c["kind"] == kind and c["agent"]]
            self.assertGreaterEqual(len(agent), 10, kind)
            self.assertTrue(all(name.startswith(prefix) for name in agent), agent)

    def test_names_use_kind_prefix(self):
        prefixes = {"wps": "writer.", "et": "et.", "wpp": "wpp."}
        for command in commands.REGISTRY.values():
            if command.kind is None:
                self.assertTrue(command.name.startswith(("app.", "ai.", "ui.")), command.name)
            else:
                self.assertTrue(command.name.startswith(prefixes[command.kind]), command.name)

    def test_catalog_matches_csharp(self):
        """Registry Python phải khớp bản C# (cùng tên, kind, cờ agent, tham số) - chỉ khác *closeAll của test."""
        if not os.path.exists(HOST_EXE):
            self.skipTest("chưa build AxiomOffice.Host.exe")
        raw = subprocess.run([HOST_EXE, "commands", "--json"], capture_output=True, text=True, encoding="utf-8", timeout=60).stdout
        csharp = {item["name"]: item for item in json.loads(raw)}
        python = {item["name"]: item for item in commands.catalog()}
        only_python = set(python) - set(csharp)
        self.assertEqual(only_python, {"et.closeAll", "wpp.closeAll", "ui.setup"},
                         "lệnh chỉ có ở làn LibreOffice (đóng tài liệu + mở wizard thiết lập, không ForAgent)")
        self.assertEqual(set(csharp) - set(python), set(), "lệnh C# bị thiếu trong extension")
        for name, theirs in csharp.items():
            mine = python[name]
            self.assertEqual((mine["kind"], mine["agent"]), (theirs["kind"], theirs["agent"]), name)
            self.assertEqual([(p["name"], p["required"]) for p in mine["params"]],
                             [(p["name"], p["required"]) for p in theirs["params"]], name)


class FakeUndoManager:
    """Ngan xep undo: phan tu cuoi la buoc tren cung (getCurrentUndoActionTitle)."""

    def __init__(self, titles):
        self.titles = list(titles)

    def isUndoPossible(self):
        return bool(self.titles)

    def getCurrentUndoActionTitle(self):
        return self.titles[-1]

    def undo(self):
        self.titles.pop()


class FakeDocument:
    def __init__(self, title, modified, store_fails=False):
        self.title, self.modified, self.store_fails = title, modified, store_fails
        self.closed, self.stored = False, []

    def supportsService(self, name):
        return name == "com.sun.star.text.TextDocument"

    def getTitle(self):
        return self.title

    def isModified(self):
        return self.modified

    def setModified(self, value):
        self.modified = value

    def storeToURL(self, url, args):
        if self.store_fails:
            raise RuntimeError("disk full")
        self.stored.append(url)
        with open(url, "w") as handle:  # to_url duoc thay bang ham dong nhat trong test
            handle.write("x")

    def close(self, deliver):
        self.closed = True


class SafetyTests(unittest.TestCase):
    """Bridge goi duoc tu /cmd ma khong qua policy cua Core: khong duoc xoa viec cua nguoi dung."""

    def setUp(self):
        from axiom import documents  # noqa: PLC0415

        self.documents = documents

    def test_undo_only_removes_ai_steps(self):
        manager = FakeUndoManager(["Gõ: chào", "AI: writer.typeText", "AI: writer.insertTable"])
        result = self.documents.undo(types.SimpleNamespace(getUndoManager=lambda: manager), 3)
        self.assertEqual(result, {"undone": 2, "stoppedAt": "Gõ: chào"})
        self.assertEqual(manager.titles, ["Gõ: chào"])

    def test_undo_stops_when_user_edited_after_ai(self):
        manager = FakeUndoManager(["AI: writer.typeText", "Gõ: sửa tay"])
        result = self.documents.undo(types.SimpleNamespace(getUndoManager=lambda: manager), 1)
        self.assertEqual(result, {"undone": 0, "stoppedAt": "Gõ: sửa tay"})
        self.assertEqual(len(manager.titles), 2, "chinh sua cua nguoi dung phai con nguyen")

    def test_undo_all_ai_steps_has_no_stop_marker(self):
        manager = FakeUndoManager(["AI: et.writeRange"])
        result = self.documents.undo(types.SimpleNamespace(getUndoManager=lambda: manager), 5)
        self.assertEqual(result, {"undone": 1})

    def test_close_all_rescues_unsaved_documents(self):
        import tempfile  # noqa: PLC0415

        with tempfile.TemporaryDirectory() as data:
            os.environ["XDG_DATA_HOME"] = data
            try:
                clean = FakeDocument("da-luu.odt", modified=False)
                dirty = FakeDocument("Báo cáo/tháng 9.docx", modified=True)
                broken = FakeDocument("hong.odt", modified=True, store_fails=True)
                self._with_components([clean, dirty, broken])
                result = self.documents.close_all(None, "wps")
            finally:
                del os.environ["XDG_DATA_HOME"]
                self._restore()

            self.assertEqual(result["closed"], 2)
            self.assertTrue(clean.closed and not clean.stored, "tai lieu da luu: dong, khong chep")
            self.assertTrue(dirty.closed and len(dirty.stored) == 1, "tai lieu chua luu: chep ban cuu ho roi dong")
            rescued = result["rescued"]
            self.assertEqual(len(rescued), 1)
            self.assertTrue(rescued[0].startswith(os.path.join(data, "axiom-office", "rescued")), rescued)
            self.assertTrue(rescued[0].endswith(".odt") and "/" not in os.path.basename(rescued[0]), rescued)
            self.assertTrue(os.path.exists(rescued[0]))
            self.assertFalse(broken.closed, "khong chep duoc ban cuu ho thi giu tai lieu mo")
            self.assertTrue(broken.modified)

    def _with_components(self, components):
        class Enumeration:
            def __init__(self, items):
                self.items = list(items)

            def hasMoreElements(self):
                return bool(self.items)

            def nextElement(self):
                return self.items.pop(0)

        desktop = types.SimpleNamespace(
            getComponents=lambda: types.SimpleNamespace(createEnumeration=lambda: Enumeration(components)))
        # Module uno gia cua test khac (test_calc/test_chat, nap truoc khi chay discover) la module rong:
        # khong de close_all cham toi uno o day.
        self._saved = (self.documents.desktop, self.documents.to_url, self.documents.props)
        self.documents.desktop = lambda ctx: desktop
        self.documents.to_url = lambda path: path
        self.documents.props = lambda **values: values

    def _restore(self):
        self.documents.desktop, self.documents.to_url, self.documents.props = self._saved


if __name__ == "__main__":
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    unittest.main(verbosity=2)

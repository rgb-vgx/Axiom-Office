"""Spec: dialog "Soat tai lieu" / "Ky nang" / "Nhat ky hoat dong" (khong can LibreOffice).

Vi sao can: khong test nao khac import `axiom/dialogs.py` (no can UNO that), nen mot loi import trong file
do chi lo ra khi nguoi dung mo LibreOffice. O day stub UNO vua du de import that, va kiem them rang:
- moi loai tai lieu (wps|et|wpp) co dung mot lenh soat,
- moi lenh do CO THAT trong registry (commands.REGISTRY) va dung `kind` - doi ten lenh trong checks.py ma
  quen sua bang nay thi menu se bao "unknown action" thay vi im lang,
- phan bien ket qua Core thanh chuoi hien cho nguoi dung (skill_lines, skill_status, audit_lines, _clock).

    python3 tests/lo/test_dialogs.py
"""
from __future__ import annotations

import datetime
import os
import sys
import types
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")


def _module_any(name: str) -> types.ModuleType:
    """Module gia tra ve mot lop moi cho MOI thuoc tinh duoc hoi (kieu `com.sun.star.awt.XItemListener`).

    Nho vay khong phai liet ke tung giao dien; them mot giao dien moi trong awt.py/widgets.py cung khong
    lam do file nay.
    """
    module = types.ModuleType(name)
    module.__getattr__ = lambda attr: type(attr, (), {})
    return module


def _install_stubs() -> None:
    if "uno" not in sys.modules:
        sys.modules["uno"] = types.ModuleType("uno")
    uno = sys.modules["uno"]
    for attr, value in (("createUnoStruct", lambda _name: types.SimpleNamespace()),
                        ("Enum", lambda kind, name: name),
                        ("Any", lambda kind, value: value),
                        ("invoke", lambda obj, name, args: getattr(obj, name)(*args)),
                        ("getConstantByName", lambda name: 0)):
        if not hasattr(uno, attr):
            setattr(uno, attr, value)
    if "unohelper" not in sys.modules:
        unohelper = _module_any("unohelper")
        # Base phai la mot LOP, khong phai `object`: cac lop trong awt.py ke thua Base cung vai giao dien.
        unohelper.Base = type("Base", (), {})
        sys.modules["unohelper"] = unohelper
    for name in ("com", "com.sun", "com.sun.star", "com.sun.star.awt", "com.sun.star.frame",
                 "com.sun.star.lang", "com.sun.star.ui"):
        if name not in sys.modules:
            module = _module_any(name)
            sys.modules[name] = module
            if "." in name:
                parent, leaf = name.rsplit(".", 1)
                setattr(sys.modules[parent], leaf, module)


_install_stubs()
sys.path.insert(0, PYTHONPATH)

from axiom import commands, dialogs, documents  # noqa: E402

commands.load_all()


class _Component:
    def __init__(self, service=None):
        self._service = service

    def supportsService(self, name):
        return name == self._service


class _Desktop:
    def __init__(self, component):
        self._component = component

    def getCurrentComponent(self):
        return self._component


class _Context:
    """ctx toi thieu: `documents.desktop(ctx)` chi can ServiceManager.createInstanceWithContext."""

    def __init__(self, component):
        self._desktop = _Desktop(component)
        self.ServiceManager = types.SimpleNamespace(
            createInstanceWithContext=lambda name, ctx: self._desktop)


class MappingTest(unittest.TestCase):
    def test_moi_loai_tai_lieu_co_dung_mot_lenh_soat(self):
        self.assertEqual({"wps", "et", "wpp"}, set(dialogs.CHECK_COMMANDS))

    def test_lenh_soat_co_that_va_dung_loai(self):
        for kind, (action, label) in dialogs.CHECK_COMMANDS.items():
            self.assertIn(action, commands.REGISTRY,
                          "lenh '%s' khong co trong registry - doi ten trong checks.py thi phai sua o day" % action)
            self.assertEqual(kind, commands.REGISTRY[action].kind,
                             "lenh '%s' khai bao kind khac voi bang CHECK_COMMANDS" % action)
            self.assertTrue(label, "lenh '%s' thieu nhan hien cho nguoi dung" % action)


class CurrentKindTest(unittest.TestCase):
    def test_nhan_dung_loai_dang_mo(self):
        for kind, service in documents.SERVICES.items():
            self.assertEqual(kind, dialogs._current_kind(_Context(_Component(service))))

    def test_khong_phai_tai_lieu_thi_tra_none(self):
        for component in (_Component("com.sun.star.text.TextDocumentX"), _Component(None)):
            self.assertIsNone(dialogs._current_kind(_Context(component)))

    def test_component_la_thi_khong_no(self):
        self.assertIsNone(dialogs._current_kind(_Context(object())))


class SkillLinesTest(unittest.TestCase):
    """Hien thi ky nang: mot dong moi ky nang, va KHONG duoc im lang bo qua thu muc nap loi."""

    def test_moi_ky_nang_mot_dong(self):
        listing = {"skills": [{"name": "bao-cao-thang", "description": "Báo cáo tháng", "apps": ["wps"]},
                              {"name": "trinh-bay", "description": "", "apps": []}]}
        self.assertEqual(["bao-cao-thang — Báo cáo tháng · wps", "trinh-bay"], dialogs.skill_lines(listing))

    def test_thu_muc_nap_loi_van_hien(self):
        listing = {"skills": [], "errors": [{"directory": "/tmp/skills", "error": "missing name"}]}
        self.assertEqual(["⚠ Không nạp được /tmp/skills: missing name"], dialogs.skill_lines(listing))

    def test_khong_co_gi_thi_noi_thang(self):
        for listing in ({}, None, {"skills": [], "errors": []}):
            self.assertEqual(["Chưa có kỹ năng nào."], dialogs.skill_lines(listing))


class SkillStatusTest(unittest.TestCase):
    def test_dem_va_loc_theo_app(self):
        listing = {"skills": [{"name": "x"}],
                   "sources": [{"name": "builtin", "directory": "/x", "exists": True},
                               {"name": "user", "directory": "/y", "exists": False}]}
        self.assertEqual("1 kỹ năng cho Writer · nguồn: sẵn có", dialogs.skill_status(listing, "wps"))
        self.assertEqual("1 kỹ năng · nguồn: sẵn có", dialogs.skill_status(listing))
        self.assertEqual("1 kỹ năng cho Impress · nguồn: cá nhân",
                         dialogs.skill_status({"skills": [{}], "sources": [{"name": "user", "exists": True}]}, "wpp"))

    def test_bao_so_thu_muc_nap_loi(self):
        self.assertEqual("0 kỹ năng · 2 thư mục nạp lỗi", dialogs.skill_status({"skills": [], "errors": [{}, {}]}))


class AuditLinesTest(unittest.TestCase):
    """Nhat ky hoat dong: gio · lenh · ok/loi · so ms. Gio doi theo mui gio may nen chi kiem phan duoi."""

    def test_moi_lenh_mot_dong(self):
        calls = [{"tool": "office_action", "action": "writer.insertText", "ok": True, "ms": 42,
                  "createdAt": "2026-10-13T00:00:00.000Z"},
                 {"tool": "office_action", "action": "wpp.addSlide", "ok": False, "ms": 3,
                  "error": "khong tim thay slide", "createdAt": "2026-10-13T00:00:01.000Z"}]
        lines = dialogs.audit_lines(calls)
        self.assertTrue(lines[0].endswith("· office_action.writer.insertText · ok · 42 ms"), lines[0])
        self.assertTrue(lines[1].endswith("· office_action.wpp.addSlide · LỖI · 3 ms — khong tim thay slide"),
                        lines[1])

    def test_tool_khong_co_action_thi_chi_in_ten_tool(self):
        self.assertTrue(dialogs.audit_lines([{"tool": "memory", "ok": True, "ms": 1}])[0]
                        .endswith("· memory · ok · 1 ms"))

    def test_chua_co_lenh_thi_noi_thang(self):
        for calls in ([], None):
            self.assertEqual(["Chưa có lượt gọi lệnh nào."], dialogs.audit_lines(calls))


class ClockTest(unittest.TestCase):
    def test_gio_utc_doi_sang_gio_may(self):
        expected = datetime.datetime(2026, 10, 13, 12, 34, 56, tzinfo=datetime.timezone.utc
                                     ).astimezone().strftime("%d/%m %H:%M")
        self.assertEqual(expected, dialogs._clock("2026-10-13T12:34:56.789Z"))

    def test_chuoi_khong_doc_duoc_thi_tra_nguyen(self):
        for stamp in ("", None, "khong-phai-gio", "2026-10-13"):
            self.assertEqual(str(stamp or ""), dialogs._clock(stamp))


class SurfaceTest(unittest.TestCase):
    def test_dialog_mo_duoc_voi_target_toi_thieu(self):
        for name in ("open_check", "open_memory", "open_skills", "open_audit"):
            self.assertTrue(callable(getattr(dialogs, name)), name)


if __name__ == "__main__":
    unittest.main()

"""Spec: menu Addons.xcu va ACTIONS cua dispatch.py phai khop nhau (khong can LibreOffice).

Boi canh: `ACTIONS` trong axiom/dispatch.py la danh sach TRANG. Them mot muc menu vao Addons.xcu ma quen
them ten vao ACTIONS thi `_action()` tra None, `dispatch()` thoat ngay - nguoi dung bam menu khong thay gi
xay ra, khong loi, khong thong bao, va cung khong co gi trong log. Do la kieu hong im lang, nen o day doi
chieu thang hai ben.

Cung kiem: URL la khong lam gi ca; mot action nem loi thi khong duoc lam sap LibreOffice (dispatch bat het).

    python3 tests/lo/test_dispatch.py
"""
from __future__ import annotations

import os
import sys
import types
import unittest
import xml.etree.ElementTree as ET

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
ADDONS_XCU = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "Addons.xcu")
URL_PREFIX = "org.axiomoffice.bridge:"


def _stub_uno():
    """dispatch.py import unohelper + com.sun.star.frame/lang; thay bang lop rong la du de import.

    Khong test nao trong tests/lo dung toi unohelper truoc day, nen day la cho dat stub do (dat truoc khi
    `from axiom import dispatch`).
    """
    if "uno" not in sys.modules:
        sys.modules["uno"] = types.ModuleType("uno")
    if "unohelper" not in sys.modules:
        unohelper = types.ModuleType("unohelper")
        # Base la mot LOP (khong phai `object`): Dispatcher ke thua Base cung ba giao dien cua com.sun.star,
        # de nguyen `object` trong danh sach base thi C3 bao "Cannot create a consistent method resolution order".
        unohelper.Base = type("Base", (), {})
        sys.modules["unohelper"] = unohelper
    com = sys.modules.setdefault("com", types.ModuleType("com"))
    for name, attrs in (("com.sun", ()),
                        ("com.sun.star", ()),
                        ("com.sun.star.frame", ("XDispatch", "XDispatchProvider")),
                        ("com.sun.star.lang", ("XInitialization",))):
        module = sys.modules.get(name)
        if module is None:
            module = types.ModuleType(name)
            sys.modules[name] = module
        for attr in attrs:
            if not hasattr(module, attr):
                setattr(module, attr, type(attr, (), {}))
        if "." in name:
            parent, leaf = name.rsplit(".", 1)
            setattr(sys.modules[parent], leaf, module)
    return com


_stub_uno()
sys.path.insert(0, PYTHONPATH)

import axiom  # noqa: E402
from axiom import dispatch  # noqa: E402


def menu_actions() -> set:
    """Cac action xuat hien trong menu Addons.xcu (phan sau "org.axiomoffice.bridge:")."""
    actions = set()
    for element in ET.parse(ADDONS_XCU).iter():
        text = (element.text or "").strip() if element.tag.endswith("value") else ""
        if text.startswith(URL_PREFIX):
            actions.add(text[len(URL_PREFIX):])
    return actions


class MenuParityTest(unittest.TestCase):
    def test_moi_muc_menu_deu_co_action(self):
        missing = menu_actions() - dispatch.ACTIONS
        self.assertEqual(set(), missing, "muc menu khong co trong ACTIONS (bam vao se khong lam gi): %s" % sorted(missing))

    def test_khong_co_action_thua(self):
        extra = dispatch.ACTIONS - menu_actions()
        self.assertEqual(set(), extra, "ACTIONS co ten khong muc menu nao dung: %s" % sorted(extra))

    def test_menu_co_muc_soat_tai_lieu(self):
        self.assertIn("check", menu_actions())


class ActionTest(unittest.TestCase):
    def _dispatcher(self, frame=None):
        item = dispatch.Dispatcher(ctx=None)
        item.initialize([frame])
        return item

    def test_nhan_url_cua_minh(self):
        item = self._dispatcher()
        for action in dispatch.ACTIONS:
            self.assertEqual(action, item._action(types.SimpleNamespace(Complete=URL_PREFIX + action)))

    def test_tu_choi_url_khac(self):
        item = self._dispatcher()
        for complete in (URL_PREFIX + "khong-co-that", "org.other.thing:setup", "", "askpane"):
            self.assertIsNone(item._action(types.SimpleNamespace(Complete=complete)))

    def test_url_hong_khong_no(self):
        item = self._dispatcher()
        self.assertIsNone(item._action(object()))


class RoutingTest(unittest.TestCase):
    """dispatch() goi dung ham cua tung module - va nuot loi cua chung."""

    def setUp(self):
        self.calls = []
        self.fakes = {}
        for name, entry, failure in (("panel", "show_pane", False),
                                     ("dialogs", None, False),
                                     ("setupwizard", "open_setup", True)):
            module = types.ModuleType("axiom." + name)
            if entry:
                def make(entry=entry, failure=failure, label=name):
                    def func(*args):
                        self.calls.append(label)
                        if failure:
                            raise RuntimeError("no")
                    return func
                setattr(module, entry, make())
            module.open_settings = lambda *a: self.calls.append("settings")
            module.open_check = lambda *a: self.calls.append("check")
            module.open_skills = lambda *a: self.calls.append("skills")
            module.open_audit = lambda *a: self.calls.append("audit")
            self.fakes[name] = module
            sys.modules["axiom." + name] = module
            setattr(axiom, name, module)
        self.addCleanup(self._restore)

    def _restore(self):
        for name, _module in self.fakes.items():
            sys.modules.pop("axiom." + name, None)
            if hasattr(axiom, name):
                delattr(axiom, name)

    def _dispatch(self, action):
        item = dispatch.Dispatcher(ctx=None)
        item.initialize(["cua-so-gia"])
        item.dispatch(types.SimpleNamespace(Complete=URL_PREFIX + action), ())

    def test_soat_tai_lieu_mo_dialog(self):
        self._dispatch("check")
        self.assertEqual(["check"], self.calls)

    def test_ky_nang_va_nhat_ky_mo_dialog(self):
        self._dispatch("skills")
        self._dispatch("audit")
        self.assertEqual(["skills", "audit"], self.calls)

    def test_askpane_va_settings_van_chay(self):
        self._dispatch("askpane")
        self._dispatch("settings")
        self.assertEqual(["panel", "settings"], self.calls)

    def test_loi_cua_mot_action_khong_lan_ra_ngoai(self):
        self._dispatch("setup")          # fake nem RuntimeError
        self.assertEqual(["setupwizard"], self.calls)


if __name__ == "__main__":
    unittest.main()

"""Spec: dialog "Soat tai lieu" - import duoc, va bang CHECK_COMMANDS khop registry (khong can LibreOffice).

Vi sao can: khong test nao khac import `axiom/dialogs.py` (no can UNO that), nen mot loi import trong file
do chi lo ra khi nguoi dung mo LibreOffice. O day stub UNO vua du de import that, va kiem them rang:
- moi loai tai lieu (wps|et|wpp) co dung mot lenh soat,
- moi lenh do CO THAT trong registry (commands.REGISTRY) va dung `kind` - doi ten lenh trong checks.py ma
  quen sua bang nay thi menu se bao "unknown action" thay vi im lang.

    python3 tests/lo/test_dialogs.py
"""
from __future__ import annotations

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


class SurfaceTest(unittest.TestCase):
    def test_dialog_mo_duoc_voi_target_toi_thieu(self):
        self.assertTrue(callable(dialogs.open_check))
        self.assertTrue(callable(dialogs.open_memory))


if __name__ == "__main__":
    unittest.main()

"""Spec: tham so `presentation` cua cac lenh wpp.* phai chon dung tai lieu dang mo (khong can LibreOffice).

Nhieu lenh Impress khai bao `presentation` ("full path of the presentation to work on; default: the active
one") nhung bridge.Env.document luon tra tai lieu dang active. Khi hai bai cung mo, lenh nham bai van
"thanh cong" - chi thay sai khi nhin vao bai kia.

    python3 tests/lo/test_bridge_env.py
"""
from __future__ import annotations

import os
import sys
import types
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
if "uno" not in sys.modules:
    sys.modules["uno"] = types.ModuleType("uno")
uno = sys.modules["uno"]
if not hasattr(uno, "systemPathToFileUrl"):
    uno.systemPathToFileUrl = lambda path: "file://" + os.path.abspath(path)
if not hasattr(uno, "fileUrlToSystemPath"):
    uno.fileUrlToSystemPath = lambda url: url[len("file://"):]
try:                                    # may CI khong co python3-uno
    import unohelper  # noqa: F401
except Exception:  # noqa: BLE001
    _helper = types.ModuleType("unohelper")
    _helper.Base = type("Base", (), {})
    sys.modules["unohelper"] = _helper
def _module_any(name: str) -> types.ModuleType:
    """Module gia tra ve mot lop moi cho moi thuoc tinh duoc hoi (XCallback, XActionListener...)."""
    module = types.ModuleType(name)
    module.__getattr__ = lambda attr: type(attr, (), {})
    return module


if "com" not in sys.modules:            # gate.py: from com.sun.star.awt import XCallback
    for _name in ("com", "com.sun", "com.sun.star", "com.sun.star.awt"):
        _module = _module_any(_name)
        sys.modules[_name] = _module
        if "." in _name:
            _parent, _leaf = _name.rsplit(".", 1)
            setattr(sys.modules[_parent], _leaf, _module)
sys.path.insert(0, PYTHONPATH)

from axiom import bridge, documents  # noqa: E402

WPP = "com.sun.star.presentation.PresentationDocument"


class Document:
    def __init__(self, url):
        self.url = url

    def getURL(self):
        return self.url

    def supportsService(self, service):
        return service == WPP

    def getTitle(self):
        return os.path.basename(self.url)

    def isModified(self):
        return False


class Enumeration:
    def __init__(self, items):
        self.items = list(items)

    def hasMoreElements(self):
        return bool(self.items)

    def nextElement(self):
        return self.items.pop(0)


class Components:
    def __init__(self, items):
        self.items = items

    def createEnumeration(self):
        return Enumeration(self.items)


class Desktop:
    def __init__(self, items):
        self.items = items

    def getCurrentComponent(self):
        return self.items[0] if self.items else None

    def getComponents(self):
        return Components(self.items)


class ServiceManager:
    def __init__(self, desktop):
        self.desktop = desktop

    def createInstanceWithContext(self, name, ctx):
        return self.desktop


class Ctx:
    def __init__(self, items):
        self.ServiceManager = ServiceManager(Desktop(items))


def _path(name: str) -> str:
    """Duong dan tuyet doi chay duoc ca Windows lan Linux (CI chay ca hai)."""
    return os.path.join(os.path.abspath(os.sep), name)


def _url(path: str) -> str:
    """URL dung CUNG ham voi production, neu khong tren Windows se lech o dia/thu muc goc."""
    return documents.to_url(os.path.abspath(path))


class BridgeEnvTest(unittest.TestCase):
    def test_presentation_chon_dung_tai_lieu_dang_mo(self):
        first_path, second_path = _path("mot.odp"), _path("hai.odp")
        first, second = Document(_url(first_path)), Document(_url(second_path))
        ctx = Ctx([first, second])          # bai dang active la bai DAU
        env = bridge.Env(ctx, "wpp", None, {"presentation": second_path})
        self.assertIs(second, env.document)

    def test_khong_co_presentation_van_dung_tai_lieu_active(self):
        first = Document(_url(_path("mot.odp")))
        ctx = Ctx([first])
        self.assertIs(first, bridge.Env(ctx, "wpp", None, {}).document)

    def test_presentation_khong_mo_thi_bao_loi_ro_rang(self):
        ctx = Ctx([Document(_url(_path("mot.odp")))])
        env = bridge.Env(ctx, "wpp", None, {"presentation": _path("khong-mo.odp")})
        with self.assertRaises(documents.NoDocumentError) as caught:
            _ = env.document
        self.assertIn("khong-mo.odp", str(caught.exception))


if __name__ == "__main__":
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    unittest.main()

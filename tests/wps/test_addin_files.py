"""Tep add-in WPS phai la XML/HTML hop le - WPS 12.1.2 bo qua ca ribbon khi XML hong, khong bao loi.

    python3 tests/wps/test_addin_files.py

Trieu chung da gap tren WPS that (28/10/2026): `ribbon.xml` co mot dau `&` tran trong label
("Bộ nhớ & kỹ năng"). WPS van GET ribbon.xml 200, nhung parser XML that bai -> KHONG co tab
"Axiom Office", `OnAddinLoad` khong duoc goi, add-in khong poll -> bridge bao "not connected".
Khong co thong bao loi nao trong UI hay log Core, nen chi test nay bat duoc.
"""
from __future__ import annotations

import os
import unittest
import xml.dom.minidom

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
ADDIN = os.path.join(ROOT, "src", "AxiomOffice.WPS")


class AddinFileTests(unittest.TestCase):
    def test_ribbon_xml_is_well_formed(self):
        path = os.path.join(ADDIN, "ribbon.xml")
        try:
            document = xml.dom.minidom.parse(path)
        except Exception as error:  # noqa: BLE001 - thong bao ro duong dan + loi parser
            self.fail("ribbon.xml khong hop le (%s): WPS se khong nap add-in" % error)
        custom = document.documentElement
        self.assertEqual(custom.tagName, "customUI")
        # onLoad phai tro toi ham ma main.js dat tren window (window.ribbon.OnAddinLoad).
        self.assertEqual(custom.getAttribute("onLoad"), "ribbon.OnAddinLoad")
        tabs = document.getElementsByTagName("tab")
        labels = [tab.getAttribute("label") for tab in tabs]
        self.assertIn("Axiom Office", labels)

    def test_every_xml_file_under_addin_parses(self):
        found = 0
        for base, _dirs, names in os.walk(ADDIN):
            for name in names:
                if not name.endswith(".xml"):
                    continue
                found += 1
                path = os.path.join(base, name)
                try:
                    xml.dom.minidom.parse(path)
                except Exception as error:  # noqa: BLE001
                    self.fail("%s khong hop le: %s" % (path, error))
        self.assertGreaterEqual(found, 1)


if __name__ == "__main__":
    unittest.main(verbosity=2)

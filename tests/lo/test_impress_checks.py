"""Spec: wpp.checkLayout phai cho CUNG mot ket qua moi lan goi, va khong duoc sua slide khi soat.

Log 13/10/2026 (do tren LibreOffice that, xem /tmp/lo-wpp-probe/micro5.py): phep do chieu cao chu trong
checks._shape_text_height bat TextAutoGrowHeight len roi tra tham so do ve False - nhung autogrow chi LON
ra chu khong tu nho lai, nen chinh phep do keo khung cao vinh vien. Mot khung 20pt chua 300 ky tu bi keo
len 107.9pt ngay o lan soat dau tien; tu lan sau khung da vua chu, nen `overflow` khong bao nua:

    slide qua day:  lan goi 1 -> issueCount 3 (overflow, overflow, dense)
                    lan goi 2 -> issueCount 1 (dense)

Agent sua tran chu xong goi lai se thay bao cao sach va tuong da xong, trong khi chu van tran - dung kieu
loi im lang ma review 8 skill (docs/research/skills-review.md) dang di tim. Ban WPS cua cung lenh nay da dung hop
dong nay tu truoc (tests/wps/test_addin_wpp.mjs: goi checkLayout hai lan van ra issueCount 1), nen day la
sua ban LibreOffice cho khop, khong phai dat ra quy tac moi.

Fake o day mo phong dung ba hanh vi da do cua LibreOffice: autogrow chi lon, khong tu nho; bi chan o
900pt; va tat autogrow KHONG tra chieu cao ve gia tri cu.

    python3 tests/lo/test_impress_checks.py
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
if not hasattr(sys.modules["uno"], "createUnoStruct"):
    sys.modules["uno"].createUnoStruct = lambda _name: types.SimpleNamespace()
sys.path.insert(0, PYTHONPATH)

from axiom import commands  # noqa: E402

commands.load_all()
from axiom import checks, values  # noqa: E402

# Chieu cao toi da ma LibreOffice dan ra khi bat autogrow (do 13/10/2026: khung 1200pt va 2000pt deu tra 900.0).
MAX_AUTOGROW_PT = 900.0


def hmm(point: float) -> int:
    return int(round(point * values.POINT_TO_HMM))


class Portion:
    def __init__(self, char_height):
        self.CharHeight = char_height


class Paragraph:
    def __init__(self, char_height):
        self._portions = [Portion(char_height)]

    def createEnumeration(self):
        return iter(self._portions)


class Text:
    def __init__(self, shape):
        self._shape = shape
        self.CharHeight = shape.font_pt
        self.CharColor = 0
        self.ParaAdjust = None

    def getString(self):
        return self._shape.text

    def setString(self, value):
        self._shape.text = value

    def createEnumeration(self):
        return iter([Paragraph(self._shape.font_pt)])


class Shape:
    """Mot text shape cua Impress, du ba hanh vi autogrow da do (xem docstring dau file)."""

    def __init__(self, text="", left=60.0, top=60.0, width=540.0, height=120.0, font_pt=18.0, text_pt=0.0):
        self.text = text
        self.font_pt = font_pt
        self.text_pt = text_pt          # chieu cao chu THAT (do truoc bang resize, khong tinh lai o day)
        self.Name = "AxiomText"
        self.Position = types.SimpleNamespace(X=hmm(left), Y=hmm(top))
        self.Size = types.SimpleNamespace(Width=hmm(width), Height=hmm(height))
        self._grow = False
        self._text = Text(self)

    @property
    def TextAutoGrowHeight(self):
        return self._grow

    @TextAutoGrowHeight.setter
    def TextAutoGrowHeight(self, value):
        self._grow = bool(value)
        if self._grow:
            need = hmm(self.text_pt)
            self.Size.Height = min(max(self.Size.Height, need), hmm(MAX_AUTOGROW_PT))

    def getText(self):
        return self._text

    def supportsService(self, name):
        return name == "com.sun.star.drawing.Text"


class Page:
    def __init__(self, shapes=(), width_pt=793.7, height_pt=446.5):
        self.Width, self.Height = hmm(width_pt), hmm(height_pt)
        self._shapes = list(shapes)

    def getCount(self):
        return len(self._shapes)

    def getByIndex(self, index):
        return self._shapes[index]


class Pages:
    def __init__(self, *pages):
        self._pages = list(pages)

    def getCount(self):
        return len(self._pages)

    def getByIndex(self, index):
        return self._pages[index]


class Env:
    def __init__(self, *pages):
        self.document = types.SimpleNamespace(getDrawPages=lambda: Pages(*pages))


def kinds(report, slide=0):
    return [i["type"] for i in report["slides"][slide]["issues"]]


class LayoutReportTests(unittest.TestCase):
    def test_text_taller_than_box_is_reported_as_overflow(self):
        page = Page([Shape(text="a" * 300, width=700, height=20, font_pt=18, text_pt=107.9)])
        self.assertEqual(kinds(checks.layout_report(Env(page), {})), ["overflow"])

    def test_box_tall_enough_is_not_reported(self):
        page = Page([Shape(text="a" * 300, width=700, height=296.4, font_pt=18, text_pt=107.9)])
        self.assertEqual(kinds(checks.layout_report(Env(page), {})), [])

    def test_second_report_agrees_with_the_first(self):
        """Loi 13/10/2026: lan soat dau keo khung cao, nen lan sau khong con thay tran chu."""
        page = Page([Shape(text="a" * 300, width=700, height=20, font_pt=18, text_pt=107.9)])
        env = Env(page)
        first = checks.layout_report(env, {})
        second = checks.layout_report(env, {})
        third = checks.layout_report(env, {})
        self.assertEqual(kinds(first), ["overflow"])
        self.assertEqual(kinds(second), kinds(first), "lan soat thu hai phai thay dung nhu lan dau")
        self.assertEqual(kinds(third), kinds(first))
        self.assertEqual(first["issueCount"], second["issueCount"])

    def test_reporting_does_not_change_the_slide(self):
        shape = Shape(text="a" * 300, width=700, height=20, font_pt=18, text_pt=107.9)
        before = (shape.Size.Width, shape.Size.Height, shape.Position.X, shape.Position.Y, shape.TextAutoGrowHeight)
        checks.layout_report(Env(Page([shape])), {})
        after = (shape.Size.Width, shape.Size.Height, shape.Position.X, shape.Position.Y, shape.TextAutoGrowHeight)
        self.assertEqual(after, before, "checkLayout la lenh chi doc: khong duoc doi hinh hoc hay autogrow")

    def test_full_slide_reports_every_problem_on_every_call(self):
        """Dung ca do that 13/10/2026: tieu de 38 ky tu 28pt trong khung 30pt + 900 ky tu trong khung 100pt."""
        page = Page([Shape(text="Doanh thu tang 12% nho kenh truc tuyen", left=40, top=30,
                           width=713.7, height=30, font_pt=28, text_pt=38.5),
                     Shape(text="a" * 900, left=40, top=110, width=713.7, height=100, font_pt=18, text_pt=254.0)])
        env = Env(page)
        self.assertEqual(kinds(checks.layout_report(env, {})), ["overflow", "overflow", "dense"])
        self.assertEqual(kinds(checks.layout_report(env, {})), ["overflow", "overflow", "dense"])

    def test_text_height_is_measured_through_autogrow(self):
        shape = Shape(text="a" * 300, width=700, height=20, font_pt=18, text_pt=107.9)
        self.assertAlmostEqual(checks._shape_text_height(shape), 107.9, places=1)

    def test_tall_box_keeps_its_own_height_as_the_measurement(self):
        """Khung da cao hon chu thi autogrow khong nho lai -> so do la chinh khung, va tran chu khong bao."""
        shape = Shape(text="a" * 300, width=700, height=296.4, font_pt=18, text_pt=107.9)
        self.assertAlmostEqual(checks._shape_text_height(shape), 296.4, places=1)

    def test_measurement_is_capped_like_libreoffice(self):
        shape = Shape(text="a", width=700, height=2000, font_pt=18, text_pt=27.3)
        self.assertAlmostEqual(checks._shape_text_height(shape), MAX_AUTOGROW_PT, places=1)


if __name__ == "__main__":
    unittest.main(verbosity=2)

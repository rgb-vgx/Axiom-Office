"""Spec: bao cao "Soat tai lieu" cho nguoi dung (thuan - khong can LibreOffice).

`report_lines` bien ket qua cua ba lenh soat (writer.checkTables / et.checkRange / wpp.checkLayout) thanh
cac dong de doc. `detail` goc la tieng Anh (viet cho model, xem checks.py), nen o day kiem: moi dong co
nhan tieng Viet theo `type`, va KHONG loai nao bi thieu nhan.

Test cuoi quet ma nguon checks.py: them mot `issue("<loai moi>", ...)` ma quen them nhan vao ISSUE_LABELS
thi spec nay do - dung y do, vi nhan thieu se hien ra ten loai tho cho nguoi dung.

    python3 tests/lo/test_report.py
"""
from __future__ import annotations

import os
import re
import sys
import types
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PYTHONPATH = os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath")
CHECKS_PY = os.path.join(PYTHONPATH, "axiom", "checks.py")
if "uno" not in sys.modules:
    sys.modules["uno"] = types.ModuleType("uno")
if not hasattr(sys.modules["uno"], "createUnoStruct"):
    sys.modules["uno"].createUnoStruct = lambda _name: types.SimpleNamespace()
sys.path.insert(0, PYTHONPATH)

from axiom import checks  # noqa: E402


class ReportTest(unittest.TestCase):
    def test_khong_co_van_de_thi_noi_thang(self):
        lines = checks.report_lines({"issueCount": 0, "issues": []})
        self.assertEqual(["Không phát hiện vấn đề."], lines)

    def test_ket_qua_rong_van_doc_duoc(self):
        self.assertEqual(["Không phát hiện vấn đề."], checks.report_lines(None))

    def test_moi_van_de_mot_dong(self):
        result = {"slideWidth": 960, "slideHeight": 540, "slides": [{}, {}], "issueCount": 2,
                  "issues": [{"type": "overflow", "detail": "'T1' overflows by 12pt"},
                             {"type": "overlap", "detail": "'A' and 'B' overlap (30% of the smaller shape)"}]}
        lines = checks.report_lines(result)
        self.assertEqual("2 trang · khổ 960×540 pt", lines[0])
        self.assertEqual("2 vấn đề:", lines[1])
        self.assertEqual("• Chữ tràn khung — 'T1' overflows by 12pt", lines[2])
        self.assertEqual("• Chồng lên nhau — 'A' and 'B' overlap (30% of the smaller shape)", lines[3])
        self.assertEqual(4, len(lines))

    def test_boi_canh_theo_tung_lenh(self):
        self.assertEqual("Trang tính Sheet1 · vùng A1:D9 · 9 dòng × 4 cột",
                         checks.report_lines({"sheet": "Sheet1", "range": "A1:D9", "rows": 9, "cols": 4})[0])
        self.assertEqual("3 bảng trong tài liệu",
                         checks.report_lines({"tableCount": 3})[0])
        self.assertEqual("1 trang · khổ 960×540 pt",
                         checks.report_lines({"slideWidth": 960.0, "slideHeight": 540.0, "slides": [{}]})[0])

    def test_khong_co_boi_canh_thi_bo_qua(self):
        lines = checks.report_lines({"issueCount": 1, "issues": [{"type": "dense", "detail": "too much"}]})
        self.assertEqual("1 vấn đề:", lines[0])

    def test_loai_la_thi_hien_ten_loai(self):
        lines = checks.report_lines({"issueCount": 1, "issues": [{"type": "loai-moi", "detail": "x"}]})
        self.assertEqual("• loai-moi — x", lines[1])

    def test_van_de_khong_co_detail_van_in(self):
        lines = checks.report_lines({"issueCount": 1, "issues": [{"type": "dense"}]})
        self.assertEqual("• Quá nhiều chữ trên một trang", lines[1])

    def test_moi_loai_trong_checks_py_deu_co_nhan_tieng_viet(self):
        with open(CHECKS_PY, encoding="utf-8") as handle:
            source = handle.read()
        kinds = set(re.findall(r'issue\(\s*"([a-z][a-z-]*)"', source))
        self.assertTrue(kinds, "khong quet duoc loai van de nao trong checks.py")
        missing = sorted(kind for kind in kinds if kind not in checks.ISSUE_LABELS)
        self.assertEqual([], missing, "loai van de chua co nhan tieng Viet: %s" % missing)


if __name__ == "__main__":
    unittest.main()

"""Kiem bo cham soat chinh ta (cham.py) va bo de cases.json.

    python3 -m unittest discover -s tests/bench/chinh-ta -v
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import unicodedata
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import cham  # noqa: E402

CASES = os.path.join(HERE, "cases.json")


def case(text, *fixes, cid="c1"):
    return {"id": cid, "text": text, "fixes": [{"wrong": w, "right": r, "kind": k} for w, r, k in fixes]}


SUA = case("Đề nghị sữa chữa ngay, rồi sẳn sàng hổ trợ khách.",
           ("sữa chữa", "sửa chữa", "hoi-nga"), ("sẳn sàng", "sẵn sàng", "hoi-nga"), ("hổ trợ", "hỗ trợ", "hoi-nga"))


class Tokenize(unittest.TestCase):
    def test_runs_of_space_and_non_space(self):
        self.assertEqual(cham.tokenize("a  b,c ."), ["a", "  ", "b,c", " ", "."])

    def test_round_trip(self):
        for text in ("", " x ", "Hà Nội\tngày  1/1", "Một\nhai"):
            self.assertEqual("".join(cham.tokenize(text)), text)


class ExpectedText(unittest.TestCase):
    def test_applies_every_fix(self):
        self.assertEqual(cham.expected_text(SUA), "Đề nghị sửa chữa ngay, rồi sẵn sàng hỗ trợ khách.")

    def test_wrong_must_occur_exactly_once(self):
        with self.assertRaises(ValueError):
            cham.expected_text(case("của của và của của", ("của của", "của", "lap-tu")))
        with self.assertRaises(ValueError):
            cham.expected_text(case("không có gì", ("sữa", "sửa", "hoi-nga")))

    def test_clean_case_is_unchanged(self):
        self.assertEqual(cham.expected_text(case("Không có lỗi.")), "Không có lỗi.")


class ScoreCase(unittest.TestCase):
    def test_perfect_output(self):
        s = cham.score_case(SUA, cham.expected_text(SUA))
        self.assertEqual((s["correct"], s["wrong"], s["missed"], s["false"]), (3, 0, 0, 0))

    def test_unchanged_output_misses_everything(self):
        s = cham.score_case(SUA, SUA["text"])
        self.assertEqual((s["correct"], s["wrong"], s["missed"], s["false"]), (0, 0, 3, 0))

    def test_partial_fix_and_false_edit(self):
        out = "Đề nghị sửa chữa ngay, rồi sẳn sàng hỗ trợ quý khách."
        s = cham.score_case(SUA, out)
        self.assertEqual((s["correct"], s["wrong"], s["missed"], s["false"]), (2, 0, 1, 1))
        self.assertEqual([m["wrong"] for m in s["missed_fixes"]], ["sẳn sàng"])
        self.assertEqual(len(s["false_edits"]), 1)
        fe = s["false_edits"][0]
        # chen "quý " hay " quý" deu dung - tuy cach diff ghep khoang trang
        self.assertEqual((fe["before"], fe["after"].strip()), ("", "quý"))

    def test_fix_changed_the_wrong_way(self):
        s = cham.score_case(SUA, "Đề nghị sủa chữa ngay, rồi sẵn sàng hỗ trợ khách.")
        self.assertEqual((s["correct"], s["wrong"], s["missed"], s["false"]), (2, 1, 0, 0))
        self.assertEqual(s["wrong_fixes"][0]["wrong"], "sữa chữa")
        self.assertEqual(s["wrong_fixes"][0]["got"], "sủa")

    def test_multi_token_fix_counts_once_and_only_when_whole(self):
        c = case("Ủy Ban Nhân Dân phường thông báo.", ("Ủy Ban Nhân Dân", "Ủy ban nhân dân", "viet-hoa"))
        whole = cham.score_case(c, "Ủy ban nhân dân phường thông báo.")
        self.assertEqual((whole["correct"], whole["wrong"], whole["missed"]), (1, 0, 0))
        half = cham.score_case(c, "Ủy ban Nhân Dân phường thông báo.")
        self.assertEqual((half["correct"], half["wrong"], half["missed"], half["false"]), (0, 1, 0, 0))

    def test_spacing_and_repeated_word(self):
        c = case("Kết quả của của đợt  này , xong.", ("của của", "của", "lap-tu"),
                 ("đợt  này", "đợt này", "dau-cau"), ("này , xong", "này, xong", "dau-cau"))
        s = cham.score_case(c, "Kết quả của đợt này, xong.")
        self.assertEqual((s["correct"], s["wrong"], s["missed"], s["false"]), (3, 0, 0, 0))

    def test_repeated_word_still_correct_beside_a_false_edit(self):
        c = case("Kết quả của của đợt kiểm tra.", ("của của", "của", "lap-tu"))
        s = cham.score_case(c, "Kết quả của đợt kiểm tra lại.")
        self.assertEqual((s["correct"], s["false"]), (1, 1))

    def test_any_change_to_a_clean_case_is_false(self):
        c = case("Công ty chú trọng sức khoẻ và kĩ năng.")
        s = cham.score_case(c, "Công ty chú trọng sức khỏe và kỹ năng.")
        self.assertEqual((s["correct"], s["missed"], s["false"]), (0, 0, 2))

    def test_by_kind(self):
        s = cham.score_case(SUA, "Đề nghị sửa chữa ngay, rồi sẳn sàng hỗ trợ khách.")
        self.assertEqual(s["by_kind"], {"hoi-nga": {"expected": 3, "correct": 2}})


class Score(unittest.TestCase):
    def test_totals_and_rates(self):
        clean = case("Không có lỗi.", cid="c2")
        r = cham.score([SUA, clean], {"c1": "Đề nghị sửa chữa ngay, rồi sẳn sàng hỗ trợ quý khách.",
                                      "c2": "Không có lỗi gì."})
        t = r["totals"]
        self.assertEqual((t["expected"], t["correct"], t["wrong"], t["missed"], t["false"]), (3, 2, 0, 1, 2))
        self.assertAlmostEqual(r["recall"], 2 / 3)
        self.assertAlmostEqual(r["precision"], 2 / 4)
        self.assertEqual([c["id"] for c in r["cases"]], ["c1", "c2"])

    def test_missing_output_is_reported_not_scored(self):
        r = cham.score([SUA], {})
        self.assertEqual(r["missing_outputs"], ["c1"])
        self.assertEqual(r["totals"]["expected"], 0)
        self.assertIsNone(r["recall"])
        self.assertIsNone(r["precision"])

    def test_unknown_output_id_is_an_error(self):
        with self.assertRaises(ValueError):
            cham.score([SUA], {"c1": SUA["text"], "khong-co": "x"})


class Cli(unittest.TestCase):
    def test_cli_prints_summary_and_writes_json(self):
        with tempfile.TemporaryDirectory() as d:
            outputs = os.path.join(d, "outputs.json")
            report = os.path.join(d, "report.json")
            with open(CASES, encoding="utf-8") as f:
                data = json.load(f)
            perfect = {c["id"]: cham.expected_text(c) for c in data["cases"]}
            with open(outputs, "w", encoding="utf-8") as f:
                json.dump(perfect, f, ensure_ascii=False)
            p = subprocess.run([sys.executable, os.path.join(HERE, "cham.py"), CASES, outputs, "--json", report],
                               capture_output=True, text=True, encoding="utf-8", check=False)
            self.assertEqual(p.returncode, 0, p.stderr)
            self.assertIn("recall", p.stdout)
            self.assertIn("precision", p.stdout)
            with open(report, encoding="utf-8") as f:
                r = json.load(f)
            self.assertEqual(r["recall"], 1.0)
            self.assertEqual(r["precision"], 1.0)


class Dataset(unittest.TestCase):
    """Bo de phai cham duoc: moi loi tim thay dung mot lan, dung ma loai, chu Unicode dung san (NFC)."""

    @classmethod
    def setUpClass(cls):
        with open(CASES, encoding="utf-8") as f:
            cls.data = json.load(f)

    def test_ids_unique_and_kinds_known(self):
        ids = [c["id"] for c in self.data["cases"]]
        self.assertEqual(len(ids), len(set(ids)))
        for c in self.data["cases"]:
            for fx in c["fixes"]:
                self.assertIn(fx["kind"], self.data["kinds"], c["id"])
                self.assertNotEqual(fx["wrong"], fx["right"], c["id"])

    def test_every_case_scores_perfect_against_its_answer(self):
        for c in self.data["cases"]:
            s = cham.score_case(c, cham.expected_text(c))
            self.assertEqual((s["correct"], s["wrong"], s["missed"], s["false"]), (len(c["fixes"]), 0, 0, 0), c["id"])

    def test_nfc(self):
        for c in self.data["cases"]:
            for s in [c["text"]] + [fx[k] for fx in c["fixes"] for k in ("wrong", "right")]:
                self.assertEqual(s, unicodedata.normalize("NFC", s), c["id"])

    def test_has_clean_cases_and_every_kind(self):
        self.assertGreaterEqual(sum(1 for c in self.data["cases"] if not c["fixes"]), 5)
        used = {fx["kind"] for c in self.data["cases"] for fx in c["fixes"]}
        self.assertEqual(used, set(self.data["kinds"]))


if __name__ == "__main__":
    unittest.main()

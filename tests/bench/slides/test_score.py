"""Spec cho bo cham slide (thuan - khong can PowerPoint).

    python -m unittest discover -s tests/bench/slides -v
"""
from __future__ import annotations

import json
import os
import sys
import tempfile
import unittest
import zipfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import score  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))


def shape(name, left, top, width, height, chars=10):
    return {"name": name, "left": left, "top": top, "width": width, "height": height, "chars": chars}


def grid_slide(index, title_top=30.0, issues=None):
    """Slide dung dung luoi cua skill: tieu de 40,30,880,80 + noi dung 40,130,880,370."""
    return {"index": index, "shapes": [shape("Title", 40, title_top, 880, 80), shape("Body", 40, 130, 880, 370, 200)],
            "issues": issues or []}


BRIEF = {"id": "t1", "slides": [2, 3], "mustContain": ["98,5%", "Nguyễn Thị Lan"]}


def listing(*texts):
    return {"slideCount": len(texts), "slides": [{"index": i + 1, "shapeTexts": [t]} for i, t in enumerate(texts)]}


class ScoreTest(unittest.TestCase):
    def test_clean_deck_passes(self):
        check = {"slideWidth": 960, "slideHeight": 540, "issueCount": 0, "slides": [grid_slide(1), grid_slide(2)]}
        result = score.score_run(BRIEF, check, listing("Tỷ lệ đúng hạn 98,5%", "nguyễn  thị LAN"),
                                 {"outcome": "run.completed", "rounds": 6})
        self.assertTrue(result["pass"], result)
        self.assertEqual(result["missing"], [])
        self.assertEqual(result["titleDrift"], 0.0)
        self.assertEqual(result["marginViolations"], 0)
        self.assertEqual(result["cleanSlideRatio"], 1.0)

    def test_issue_missing_fact_or_count_fails(self):
        overflow = [{"type": "overflow", "detail": "x"}]
        check = {"slideWidth": 960, "slideHeight": 540, "slides": [grid_slide(1), grid_slide(2, issues=overflow)]}
        result = score.score_run(BRIEF, check, listing("98,5%", "Nguyễn Thị Lan"), {"outcome": "run.completed"})
        self.assertFalse(result["pass"])
        self.assertEqual(result["issuesByType"], {"overflow": 1})
        self.assertEqual(result["cleanSlideRatio"], 0.5)

        clean = {"slideWidth": 960, "slideHeight": 540, "slides": [grid_slide(1), grid_slide(2)]}
        result = score.score_run(BRIEF, clean, listing("98,5%", "khác"), {"outcome": "run.completed"})
        self.assertEqual(result["missing"], ["Nguyễn Thị Lan"])
        self.assertFalse(result["pass"])

        result = score.score_run(BRIEF, clean, listing("98,5% Nguyễn Thị Lan"), {"outcome": "run.completed"})
        self.assertFalse(result["slidesOk"])
        result = score.score_run(BRIEF, clean, listing("98,5%", "Nguyễn Thị Lan"), {"outcome": "run.failed"})
        self.assertFalse(result["pass"])

    def test_title_drift_and_margins(self):
        check = {"slideWidth": 960, "slideHeight": 540,
                 "slides": [grid_slide(1), grid_slide(2), grid_slide(3, title_top=52)]}
        self.assertEqual(score.title_drift(check), 22.0)
        # Bia chi mot hop chu: khong tinh vao do lech tieu de.
        cover = {"index": 4, "shapes": [shape("Hero", 40, 200, 880, 120)], "issues": []}
        check["slides"].append(cover)
        self.assertEqual(score.title_drift(check), 22.0)
        # Bia co tieu de + dong phu dat giua slide: cung khong tinh.
        cover = {"index": 5, "shapes": [shape("Hero", 40, 184, 880, 110), shape("Sub", 40, 306, 880, 70)], "issues": []}
        check["slides"].append(cover)
        self.assertEqual(score.title_drift(check), 22.0)

        check = {"slideWidth": 960, "slideHeight": 540, "slides": [
            {"index": 1, "shapes": [shape("Background", 0, 0, 960, 540, 0), shape("Edge", 10, 130, 300, 100),
                                    shape("Ok", 39, 130, 882, 100), shape("Low", 40, 480, 300, 60)], "issues": []}]}
        self.assertEqual(score.margin_violations(check), ["1:Edge", "1:Low"])

    def test_collect_and_summary_from_run_folders(self):
        with tempfile.TemporaryDirectory() as root:
            for n, outcome in (("1", "run.completed"), ("2", "run.failed")):
                folder = os.path.join(root, "t1", n)
                os.makedirs(folder)
                files = {"brief.json": BRIEF, "run.json": {"outcome": outcome, "rounds": 4 if n == "1" else 8},
                         "check.json": {"slideWidth": 960, "slideHeight": 540, "slides": [grid_slide(1), grid_slide(2)]},
                         "slides.json": listing("98,5%", "Nguyễn Thị Lan")}
                for name, data in files.items():
                    with open(os.path.join(folder, name), "w", encoding="utf-8") as handle:
                        json.dump(data, handle, ensure_ascii=False)
            rows = score.summary(score.collect(root))
            self.assertEqual(rows[0]["pass"], "1/2")
            self.assertEqual(rows[0]["rounds"], 6.0)
            self.assertIn("| t1 | 2 | 0 | 1/2 |", score.markdown(rows))

    def test_table_text_is_read_from_the_pptx(self):
        # wpp.listSlides khong doc chu trong bang: .pptx la nguon chu.
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "deck.pptx")
            table = ("<p:graphicFrame><a:tbl><a:tr><a:tc><a:txBody><a:p><a:r><a:t>98,5%</a:t></a:r></a:p></a:txBody>"
                     "</a:tc></a:tr></a:tbl></p:graphicFrame>")
            with zipfile.ZipFile(path, "w") as package:
                package.writestr("ppt/slides/slide10.xml", "<p:sld><a:t>sau</a:t></p:sld>")
                package.writestr("ppt/slides/slide2.xml", "<p:sld><a:t>Nguyễn Thị Lan &amp; cộng sự</a:t>" + table + "</p:sld>")
                package.writestr("ppt/slides/_rels/slide2.xml.rels", "<a:t>khong phai slide</a:t>")
            texts = score.deck_texts(path)
            self.assertEqual(texts, ["Nguyễn Thị Lan & cộng sự\n98,5%", "sau"])
            clean = {"slideWidth": 960, "slideHeight": 540, "slides": [grid_slide(1), grid_slide(2)]}
            result = score.score_run(BRIEF, clean, listing("", ""), {"outcome": "run.completed"}, texts)
            self.assertEqual(result["missing"], [])
            self.assertTrue(result["pass"], result)
            # listSlides khong thay chu trong bang: readGap.
            self.assertEqual(result["readGap"], ["98,5%", "Nguyễn Thị Lan"])
            # .pptx co so slide khac bai listSlides doc: luot bi lan bai, khong hop le.
            result = score.score_run(BRIEF, clean, listing("98,5%", "Nguyễn Thị Lan", "x"), {"outcome": "run.completed"}, texts)
            self.assertFalse(result["valid"])
            self.assertFalse(result["pass"])

    def test_chart_values_are_read_from_the_pptx(self):
        # Bieu do goc: so lieu nam o ppt/charts/chartN.xml (lien ket qua rels cua slide), nhan co don vi.
        chart = ('<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" '
                 'xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><c:chart><c:plotArea><c:pieChart><c:ser>'
                 '<c:tx><c:strRef><c:strCache><c:pt idx="0"><c:v>Chi phí</c:v></c:pt></c:strCache></c:strRef></c:tx>'
                 '<c:dLbls><c:numFmt formatCode="General&quot;%&quot;" sourceLinked="0"/></c:dLbls>'
                 '<c:cat><c:strRef><c:strCache><c:pt idx="0"><c:v>Nhân sự</c:v></c:pt><c:pt idx="1"><c:v>Khác</c:v></c:pt>'
                 '</c:strCache></c:strRef></c:cat><c:val><c:numRef><c:numCache><c:formatCode>General</c:formatCode>'
                 '<c:pt idx="0"><c:v>55</c:v></c:pt><c:pt idx="1"><c:v>12.5</c:v></c:pt></c:numCache></c:numRef></c:val>'
                 '</c:ser></c:pieChart></c:plotArea></c:chart></c:chartSpace>')
        with tempfile.TemporaryDirectory() as root:
            path = os.path.join(root, "deck.pptx")
            with zipfile.ZipFile(path, "w") as package:
                package.writestr("ppt/slides/slide1.xml", "<p:sld><a:t>Cơ cấu chi phí</a:t></p:sld>")
                package.writestr("ppt/slides/_rels/slide1.xml.rels",
                                 '<Relationships><Relationship Id="rId2" Target="../charts/chart3.xml"/></Relationships>')
                package.writestr("ppt/charts/chart3.xml", chart)
            self.assertEqual(score.deck_texts(path), ["Cơ cấu chi phí\nChi phí: Nhân sự 55%, Khác 12.5%"])
        self.assertEqual(score.chart_value("0.55", "0%"), "55%")
        self.assertEqual(score.chart_value("28", 'General" tỷ"'), "28 tỷ")

    def test_unsourced_numbers(self):
        prompt = "tiếp nhận 1.240 hồ sơ, đúng hạn 98,5%, ngày 15/10/2026"
        texts = ["1240 hồ sơ, 98.5% đúng hạn", "Ngày 15/10/2026", "Thêm 5 việc, tăng 37% so với 2025"]
        self.assertEqual(score.unsourced_numbers(prompt, texts), ["2025", "37"])

    def test_must_remove_counts_leftover_over_the_run_folder(self):
        # Loi cai san ("205") con nam tren slide phai BI SUA - hai lan chay: xong va khong xong viec sua.
        with tempfile.TemporaryDirectory() as root:
            outputs = {
                "1": {"run.json": {"outcome": "run.completed", "rounds": 4},
                      "slides.json": listing("Kết quả tuyển sinh 2026: Chỉ tiêu 600, nhập học 612, tỷ lệ 102%.")},
                "2": {"run.json": {"outcome": "run.completed", "rounds": 2},
                      "slides.json": listing("Kết quả tuyển sinh 2026: Chỉ tiêu 600, nhập học 205.")},
            }
            for n, files in outputs.items():
                folder = os.path.join(root, "s25-loi-cai-san", n)
                os.makedirs(folder)
                for name, data in files.items():
                    with open(os.path.join(folder, name), "w", encoding="utf-8") as handle:
                        json.dump(data, handle, ensure_ascii=False)
                with open(os.path.join(folder, "brief.json"), "w", encoding="utf-8") as handle:
                    json.dump({"id": "s25-loi-cai-san", "slides": [1, 2], "mustContain": ["612", "102%"],
                               "mustRemove": ["205"]}, handle, ensure_ascii=False)
            results = score.collect(root)
            self.assertTrue(results[0]["pass"], results[0])
            self.assertEqual(results[0]["leftover"], [])
            self.assertFalse(results[1]["pass"])
            # Luot chay xong nhung so "205" cai san van nam tren bai: chua sua duoc loi.
            self.assertEqual(results[1]["leftover"], ["205"])
            rows = score.summary(results)
            self.assertEqual(rows[0]["leftover"], 0.5)

    def test_briefs_file_is_well_formed(self):
        with open(os.path.join(HERE, "briefs.json"), encoding="utf-8") as handle:
            briefs = json.load(handle)["briefs"]
        ids = [b["id"] for b in briefs]
        self.assertEqual(len(ids), len(set(ids)))
        self.assertTrue(20 <= len(briefs) <= 30)
        self.assertTrue(any(b["smoke"] for b in briefs))
        for brief in briefs:
            low, high = brief["slides"]
            self.assertTrue(1 <= low <= high, brief["id"])
            self.assertTrue(brief["mustContain"], brief["id"])
            self.assertTrue(set(brief["needs"]) <= {"chart", "shape", "connector"}, brief["id"])
            # Bai co loi cai san: lenh seed chi cho phep bridge wpp doc/ghi bai; so sai cai san khong duoc
            # trung voi so dung cua de bai (dung lai duoc bang moi "sua ... cho dung").
            for item in (brief.get("seed") or []):
                self.assertEqual(set(item), {"action", "params"}, brief["id"])
                self.assertTrue(str(item["action"]).startswith("wpp."), brief["id"])
            for phrase in brief.get("mustRemove") or []:
                self.assertNotIn(score.normalize(phrase), score.normalize(brief["prompt"]),
                                 brief["id"] + ": " + phrase)
                self.assertTrue(any(score.normalize(phrase) in score.normalize(c.get("params", {}).get("text", ""))
                                    for c in (brief.get("seed") or []) if c.get("action") == "wpp.addText"),
                                brief["id"] + ": " + phrase)
            # So lieu phai cham duoc: moi cum mustContain co trong chinh de bai - hoac, voi bai loi cai san ma de KHONG noi
            # cho loi (s26), trong chu cua bai cai san hoac la so suy ra ("derived", vd tong = cong cac dong).
            seeded = " ".join(score.normalize(c.get("params", {}).get("text", "")) for c in (brief.get("seed") or []))
            for phrase in brief["mustContain"]:
                if brief.get("mustRemove") and (score.normalize(phrase) in seeded or phrase in (brief.get("derived") or [])):
                    continue
                self.assertIn(score.normalize(phrase), score.normalize(brief["prompt"]), brief["id"] + ": " + phrase)


if __name__ == "__main__":
    unittest.main()

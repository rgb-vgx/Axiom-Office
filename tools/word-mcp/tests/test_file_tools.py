import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from word_mcp import file_tools


class WordFileToolsTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp(prefix="wordtest_")
        self.docx = os.path.join(self.dir, "doc.docx")
        file_tools.create(self.docx, [
            {"text": "Bao cao", "style": "Heading 1"},
            "Doan van thuong.",
            {"table": [["Ten", "Diem"], ["An", "9.5"]]},
        ], title="Test Doc")

    def tearDown(self):
        for name in os.listdir(self.dir):
            try:
                os.remove(os.path.join(self.dir, name))
            except OSError:
                pass
        try:
            os.rmdir(self.dir)
        except OSError:
            pass

    def test_profile(self):
        prof = file_tools.profile(self.docx)
        self.assertEqual(prof["paragraphs"], 2)
        self.assertEqual(prof["tables"], 1)
        self.assertEqual(prof["core"]["title"], "Test Doc")
        self.assertIn("Heading 1", prof["style_counts"])

    def test_get_text(self):
        result = file_tools.get_text(self.docx)
        self.assertIn("Bao cao", result["text"])
        self.assertIn("An | 9.5", result["text"])
        truncated = file_tools.get_text(self.docx, max_chars=5)
        self.assertTrue(truncated["truncated"])

    def test_find_text(self):
        result = file_tools.find_text(self.docx, "bao cao")
        self.assertEqual(result["count"], 1)
        self.assertEqual(result["matches"][0]["style"], "Heading 1")

    def test_extract_table(self):
        result = file_tools.extract_table(self.docx, 0)
        self.assertEqual(result["rows"], [["Ten", "Diem"], ["An", "9.5"]])
        with self.assertRaises(ValueError):
            file_tools.extract_table(self.docx, 5)

    def test_create_overwrite_guard(self):
        with self.assertRaises(ValueError):
            file_tools.create(self.docx, ["x"])
        file_tools.create(self.docx, ["x"], overwrite=True)
        self.assertEqual(file_tools.profile(self.docx)["paragraphs"], 1)


if __name__ == "__main__":
    unittest.main()

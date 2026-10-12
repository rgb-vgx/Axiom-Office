import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from ppt_mcp import file_tools


class PptFileToolsTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp(prefix="ppttest_")
        self.pptx = os.path.join(self.dir, "deck.pptx")
        file_tools.create(self.pptx, [
            {"title": "Ke hoach 2026", "bullets": ["Muc tieu", "Ngan sach"]},
        ])

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
        prof = file_tools.profile(self.pptx)
        self.assertEqual(prof["slide_count"], 1)
        texts = prof["slides"][0]["texts"]
        self.assertTrue(any("Ke hoach 2026" in t for t in texts))
        self.assertTrue(any("Ngan sach" in t for t in texts))

    def test_get_text(self):
        result = file_tools.get_text(self.pptx)
        self.assertIn("[Slide 1]", result["text"])
        self.assertIn("Muc tieu", result["text"])

    def test_add_slide(self):
        result = file_tools.add_slide(self.pptx, title="Slide 2", bullets=["A", "B"])
        self.assertEqual(result["slide_count"], 2)
        prof = file_tools.profile(self.pptx)
        self.assertEqual(prof["slide_count"], 2)

    def test_create_overwrite_guard(self):
        with self.assertRaises(ValueError):
            file_tools.create(self.pptx, [{"title": "x"}])
        file_tools.create(self.pptx, [{"title": "x"}], overwrite=True)
        self.assertEqual(file_tools.profile(self.pptx)["slide_count"], 1)


if __name__ == "__main__":
    unittest.main()

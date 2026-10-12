"""Unit test cho scripts/check_version.py - tripwire giu 4 ban sao so phien ban khop VERSION goc.

    python3 -m unittest discover -s tests/lo -v
"""
from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
SCRIPT = os.path.join(ROOT, "scripts", "check_version.py")

COPIES = [
    os.path.join("src", "AxiomOffice.LibreOffice", "python", "pythonpath", "axiom", "__init__.py"),
    os.path.join("src", "AxiomOffice.LibreOffice", "description.xml"),
    os.path.join("src", "AxiomOffice.WPS", "main.js"),
    os.path.join("core-go", "cmd", "axiom-core", "main.go"),
]

JS = os.path.join("src", "AxiomOffice.WPS", "main.js")


def repo_version() -> str:
    """So phien ban hien tai, doc tu VERSION goc.

    Doc dong nay thay vi viet cung: cay gia phai doi tu so THAT sang so khac, neu viet cung
    thi moi lan bump VERSION test lai do vi ly do sai (ban sao da la so moi, khong con gi de doi).
    """
    with open(os.path.join(ROOT, "VERSION"), encoding="utf-8") as handle:
        return handle.read().strip()


def run_check(*args: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, SCRIPT, *args],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )


class VersionTests(unittest.TestCase):
    def test_repo_that_khop(self):
        proc = run_check()
        self.assertEqual(proc.returncode, 0, proc.stderr)

    def make_fake_tree(self, version: str) -> str:
        """Cay gia: VERSION=<version>, va ca 4 ban sao da doi sang <version>."""
        tmp = tempfile.mkdtemp(prefix="axiom-version-")
        self.addCleanup(shutil.rmtree, tmp, True)
        with open(os.path.join(tmp, "VERSION"), "w", encoding="utf-8", newline="\n") as handle:
            handle.write(version + "\n")
        current = repo_version()
        for rel in COPIES:
            src = os.path.join(ROOT, rel)
            dst = os.path.join(tmp, rel)
            os.makedirs(os.path.dirname(dst), exist_ok=True)
            with open(src, encoding="utf-8") as handle:
                text = handle.read()
            text = text.replace(current, version)
            with open(dst, "w", encoding="utf-8", newline="\n") as handle:
                handle.write(text)
        return tmp

    def test_cay_gia_khop(self):
        # Chung minh trinh kiem khong phai luc nao cung do: thay toan bo sang 9.9.9 van xanh.
        tmp = self.make_fake_tree("9.9.9")
        proc = run_check("--root", tmp)
        self.assertEqual(proc.returncode, 0, proc.stderr)

    def test_cay_gia_lech_bao_do(self):
        # Phep thu bat buoc: mot trinh kiem luon bao xanh la vo dung.
        tmp = self.make_fake_tree("9.9.9")
        target = os.path.join(tmp, JS)
        with open(target, encoding="utf-8") as handle:
            text = handle.read()
        text = text.replace("9.9.9", "9.9.8")
        with open(target, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(text)
        proc = run_check("--root", tmp)
        self.assertEqual(proc.returncode, 1)
        self.assertIn(JS, proc.stderr)


if __name__ == "__main__":
    unittest.main()

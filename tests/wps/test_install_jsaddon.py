"""Unit test cho scripts/wps/install_jsaddon.py (khong can WPS) - SPEC, khong sua de qua bai.

Trinh cai chi duoc ghi trong --home (HOME that cua nguoi dung khong bao gio bi dung toi), ghi report URL vao
ban cai, va chay lai thi cap nhat chu khong dang ky trung.

    python3 tests/wps/test_install_jsaddon.py
"""
from __future__ import annotations

import os
import re
import subprocess
import sys
import tempfile
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
INSTALLER = os.path.join(ROOT, "scripts", "wps", "install_jsaddon.py")


def files_under(folder: str) -> list:
    found = []
    for base, _dirs, names in os.walk(folder):
        found.extend(os.path.join(base, name) for name in names)
    return found


def text_of(path: str) -> str:
    try:
        with open(path, encoding="utf-8", errors="replace") as handle:
            return handle.read()
    except OSError:
        return ""


class InstallerTests(unittest.TestCase):
    def setUp(self):
        if not os.path.exists(INSTALLER):
            self.fail("chua co scripts/wps/install_jsaddon.py")
        self.tmp = tempfile.TemporaryDirectory()
        self.home = os.path.join(self.tmp.name, "home")
        # HOME cua tien trinh tro vao thu muc "canh gac": trinh cai ma dung ~ thay vi --home se de lai dau vet o day.
        self.sentinel = os.path.join(self.tmp.name, "real-home")
        os.makedirs(self.home)
        os.makedirs(self.sentinel)
        self.jsaddons = os.path.join(self.home, ".local", "share", "Kingsoft", "wps", "jsaddons")

    def tearDown(self):
        self.tmp.cleanup()

    def install(self, url: str) -> subprocess.CompletedProcess:
        env = {key: value for key, value in os.environ.items() if not key.startswith("XDG_")}
        env["HOME"] = self.sentinel
        result = subprocess.run([sys.executable, INSTALLER, "--home", self.home, "--report-url", url],
                                capture_output=True, text=True, timeout=60, env=env, cwd=self.tmp.name)
        self.assertEqual(result.returncode, 0, result.stderr)
        return result

    def test_installs_only_under_home(self):
        self.install("http://127.0.0.1:41001/report")
        self.assertTrue(os.path.isdir(self.jsaddons) and os.listdir(self.jsaddons), "jsaddons rong")
        self.assertEqual(files_under(self.sentinel), [], "trinh cai ghi ra ngoai --home (dung ~ / $HOME?)")
        outside = [path for path in files_under(self.tmp.name)
                   if not path.startswith(self.home + os.sep)]
        self.assertEqual(outside, [], "trinh cai ghi ra ngoai --home")

    def test_report_url_is_injected(self):
        url = "http://127.0.0.1:41002/report"
        self.install(url)
        holders = [path for path in files_under(self.jsaddons) if url in text_of(path)]
        self.assertTrue(holders, "khong file nao trong ban cai chua report URL %s" % url)

    def test_reinstall_updates_url_without_duplicate_registration(self):
        first, second = "http://127.0.0.1:41003/report", "http://127.0.0.1:41004/report"
        self.install(first)
        publish = os.path.join(self.jsaddons, "publish.xml")
        before = len(re.findall(r"<jsplugin\b", text_of(publish)))
        self.install(second)
        installed = "\n".join(text_of(path) for path in files_under(self.jsaddons))
        self.assertIn(second, installed)
        self.assertNotIn(first, installed, "cai lai phai thay report URL cu")
        if os.path.exists(publish):
            self.assertEqual(len(re.findall(r"<jsplugin\b", text_of(publish))), before,
                             "cai lai khong duoc dang ky trung trong publish.xml")


if __name__ == "__main__":
    unittest.main(verbosity=2)

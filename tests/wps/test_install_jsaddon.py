"""Spec trinh cai add-in WPS (docs/wps-bridge.md muc 4) - offline, khong can WPS. Khong sua de qua bai.

    python3 scripts/wps/install_jsaddon.py --home <HOME> --core-url http://127.0.0.1:47840 [--test-profile]

- Chi ghi trong --home (HOME that cua tien trinh khong bao gio bi dung toi).
- <HOME>/.local/share/Kingsoft/wps/jsaddons/publish.xml co DUNG MOT
  <jspluginonline name="axiomoffice" type="wps" url="<core-url>/wps/addin/" .../>, giu dang ky cua add-in khac,
  go dang ky offline cu (<jsplugin name="axiomoffice">) va thu muc add-in offline cu (axiomoffice_*).
- Khong chep file add-in (Core phuc vu chung).
- Office.conf chi duoc ghi khi co --test-profile.

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
OTHER = '<jsplugin name="other_addin" type="et" url="other_addin_1.0" version="1.0"/>'


def files_under(folder: str) -> list:
    found = []
    for base, _dirs, names in os.walk(folder):
        found.extend(os.path.join(base, name) for name in names)
    return found


def read(path: str) -> str:
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
        self.sentinel = os.path.join(self.tmp.name, "real-home")   # HOME cua tien trinh: khong duoc dung toi
        os.makedirs(self.home)
        os.makedirs(self.sentinel)
        self.jsaddons = os.path.join(self.home, ".local", "share", "Kingsoft", "wps", "jsaddons")
        self.publish = os.path.join(self.jsaddons, "publish.xml")
        self.conf = os.path.join(self.home, ".config", "Kingsoft", "Office.conf")

    def tearDown(self):
        self.tmp.cleanup()

    def install(self, url: str, *extra: str) -> None:
        env = {key: value for key, value in os.environ.items() if not key.startswith("XDG_")}
        env["HOME"] = self.sentinel
        result = subprocess.run([sys.executable, INSTALLER, "--home", self.home, "--core-url", url, *extra],
                                capture_output=True, text=True, timeout=60, env=env, cwd=self.tmp.name)
        self.assertEqual(result.returncode, 0, result.stderr)

    def online_entries(self) -> list:
        return re.findall(r"<jspluginonline\b[^>]*>", read(self.publish))

    def test_registers_core_url_only_under_home(self):
        self.install("http://127.0.0.1:47840")
        entries = self.online_entries()
        self.assertEqual(len(entries), 1, read(self.publish))
        self.assertIn('name="axiomoffice"', entries[0])
        self.assertIn('type="wps"', entries[0])
        self.assertIn('url="http://127.0.0.1:47840/wps/addin/"', entries[0])
        self.assertEqual(files_under(self.sentinel), [], "trinh cai ghi ra ngoai --home")
        outside = [p for p in files_under(self.tmp.name) if not p.startswith(self.home + os.sep)]
        self.assertEqual(outside, [], "trinh cai ghi ra ngoai --home")
        self.assertEqual([p for p in files_under(self.jsaddons) if not p.endswith("publish.xml")], [],
                         "khong chep file add-in (Core phuc vu)")

    def test_trailing_slash_and_reinstall_replace_without_duplicate(self):
        self.install("http://127.0.0.1:47840/")
        self.install("http://127.0.0.1:47999")
        entries = self.online_entries()
        self.assertEqual(len(entries), 1, read(self.publish))
        self.assertIn('url="http://127.0.0.1:47999/wps/addin/"', entries[0])
        self.assertNotIn("47840", read(self.publish))

    def test_keeps_other_addins_and_removes_old_offline_registration(self):
        os.makedirs(os.path.join(self.jsaddons, "axiomoffice_0.1.0"))
        with open(os.path.join(self.jsaddons, "axiomoffice_0.1.0", "main.js"), "w", encoding="utf-8") as handle:
            handle.write("// ban offline cu")
        with open(self.publish, "w", encoding="utf-8") as handle:
            handle.write('<?xml version="1.0" encoding="UTF-8"?>\n<jsplugins>\n' + OTHER + "\n"
                         '<jsplugin name="axiomoffice" type="wps" url="axiomoffice_0.1.0" version="0.1.0"/>\n'
                         "</jsplugins>\n")
        self.install("http://127.0.0.1:47840")
        text = read(self.publish)
        self.assertIn(OTHER, text, "phai giu dang ky cua add-in khac")
        self.assertIsNone(re.search(r'<jsplugin\b[^>]*name="axiomoffice"', text), "phai go dang ky offline cu")
        self.assertEqual(len(self.online_entries()), 1, text)
        self.assertFalse(os.path.exists(os.path.join(self.jsaddons, "axiomoffice_0.1.0")),
                         "phai xoa thu muc add-in offline cu")

    def test_office_conf_only_with_test_profile(self):
        self.install("http://127.0.0.1:47840")
        self.assertFalse(os.path.exists(self.conf), "khong --test-profile thi khong dung Office.conf")
        self.install("http://127.0.0.1:47840", "--test-profile")
        text = read(self.conf)
        self.assertIn("[6.0]", text)
        self.assertIn("common\\AcceptedEULA=true", text)
        self.assertIn("AppComponentMode=prome_independ", text)


if __name__ == "__main__":
    unittest.main(verbosity=2)

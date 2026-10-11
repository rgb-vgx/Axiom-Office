"""Spec T3: goi Linux (scripts/linux/package.sh) mang add-in WPS - chay package.sh that (build Core Go + .oxt).

Tarball phai co: core/wps-addin/{index.html, ribbon.xml, main.js, commands.js, probe.js} (Core phuc vu tu
<thu muc binary>/wps-addin), wps/install_jsaddon.py (install.sh goi), va README.txt nhac WPS + buoc admin JsApiPlugin.

    python3 tests/linux/test_package.py
"""
from __future__ import annotations

import glob
import os
import subprocess
import tarfile
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


class PackageTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        built = subprocess.run(["bash", os.path.join(ROOT, "scripts", "linux", "package.sh")], capture_output=True,
                               text=True, timeout=900, cwd=ROOT)
        if built.returncode != 0:
            raise AssertionError("package.sh loi:\n" + (built.stdout + built.stderr)[-3000:])
        tarballs = sorted(glob.glob(os.path.join(ROOT, "dist", "axiom-office-linux-*.tar.gz")), key=os.path.getmtime)
        if not tarballs:
            raise AssertionError("khong thay dist/axiom-office-linux-*.tar.gz")
        with tarfile.open(tarballs[-1]) as archive:
            cls.names = archive.getnames()
            readme = [m for m in archive.getmembers() if m.name.endswith("/README.txt")]
            cls.readme = archive.extractfile(readme[0]).read().decode("utf-8") if readme else ""

    def has(self, suffix):
        return any(name.endswith(suffix) for name in self.names)

    def test_core_ships_wps_addin(self):
        for name in ("index.html", "ribbon.xml", "main.js", "commands.js", "probe.js"):
            self.assertTrue(self.has("/core/wps-addin/" + name), "thieu core/wps-addin/" + name)

    def test_installer_script_shipped(self):
        self.assertTrue(self.has("/wps/install_jsaddon.py"), "thieu wps/install_jsaddon.py")

    def test_readme_mentions_wps(self):
        self.assertIn("WPS", self.readme)
        self.assertIn("JsApiPlugin", self.readme)


if __name__ == "__main__":
    unittest.main(verbosity=2)

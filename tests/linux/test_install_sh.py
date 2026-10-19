"""Spec T3 (docs/wps-bridge.md muc 4): scripts/linux/install.sh cai them add-in WPS - khong can WPS/LibreOffice that.

Chay install.sh tu mot "goi" gia (core/ gia, .oxt gia, unopkg gia) voi HOME/XDG_* tam - khong bao gio dung toi
Core, LibreOffice hay WPS that cua nguoi dung. Bien moi truong cho test:
- AXIOM_INSTALL_SKIP_LO_CHECK=1 : bo kiem tra "LibreOffice dang mo" (nguoi dung co the dang dung LibreOffice).
- AXIOM_WPS_ROOT=<dir>          : thu muc cai WPS (that la /opt/kingsoft/wps-office); khong ton tai = khong co WPS.
Hanh vi mong doi:
- Co WPS: dang ky add-in vao <HOME>/.local/share/Kingsoft/wps/jsaddons/publish.xml
  (<jspluginonline name="axiomoffice" url="http://127.0.0.1:<cong Core>/wps/addin/">), bang wps/install_jsaddon.py
  trong goi; KHONG ghi Office.conf. oem.ini chua co JsApiPlugin=true -> in huong dan lenh sudo (khong tu chay sudo);
  da co -> khong in. Khong --systemd -> nhac nen dung --systemd (Core phai chay truoc khi mo WPS).
- Khong co WPS, hoac --no-wps: khong dong gi toi Kingsoft.
- Core cai kem add-in: <data>/axiom-office/core/wps-addin/index.html.
- --uninstall: go dang ky add-in khoi publish.xml (giu add-in khac).
- DU LIEU cua nguoi dung: Core luu core.db (+ -wal, -shm) ngay trong <data>/axiom-office/core/ - chinh thu muc
  install.sh thay khi cai. Cai lai / nang cap va --uninstall KHONG --purge phai giu nguyen ba tep do; chi --purge
  moi xoa (loi that 20/10/2026: ca ba duong deu rm -rf thu muc core/, mat het hoi thoai va Bo nho AI).

    python3 tests/linux/test_install_sh.py
"""
from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
OTHER = '<jsplugin name="other_addin" type="et" url="other_addin_1.0" version="1.0"/>'


class InstallShTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        base = self.tmp.name
        self.stage = os.path.join(base, "pkg")
        core = os.path.join(self.stage, "core")
        os.makedirs(os.path.join(core, "wps-addin"))
        with open(os.path.join(core, "AxiomOffice.Core"), "w") as handle:
            handle.write("#!/bin/sh\nexit 0\n")
        os.chmod(os.path.join(core, "AxiomOffice.Core"), 0o755)
        for name in ("index.html", "main.js", "ribbon.xml"):
            with open(os.path.join(core, "wps-addin", name), "w") as handle:
                handle.write("<!-- %s -->\n" % name)
        open(os.path.join(self.stage, "AxiomOffice-LibreOffice-0.1.0.oxt"), "w").close()
        os.makedirs(os.path.join(self.stage, "wps"))
        shutil.copy(os.path.join(ROOT, "scripts", "wps", "install_jsaddon.py"), os.path.join(self.stage, "wps"))
        shutil.copy(os.path.join(ROOT, "scripts", "linux", "install.sh"), self.stage)
        shutil.copy(os.path.join(ROOT, "scripts", "linux", "axiom-office-core.service.in"), self.stage)
        self.unopkg_log = os.path.join(base, "unopkg.log")
        unopkg = os.path.join(base, "unopkg")
        with open(unopkg, "w") as handle:
            handle.write('#!/bin/sh\necho "$@" >> "%s"\nexit 0\n' % self.unopkg_log)
        os.chmod(unopkg, 0o755)
        self.home = os.path.join(base, "home")
        os.makedirs(self.home)
        self.wps_root = os.path.join(base, "wps-office")
        os.makedirs(os.path.join(self.wps_root, "office6", "cfgs"))
        self.oem = os.path.join(self.wps_root, "office6", "cfgs", "oem.ini")
        self.set_oem(False)
        self.env = {
            "PATH": os.environ.get("PATH", ""), "HOME": self.home, "LANG": "C.UTF-8",
            "XDG_CONFIG_HOME": os.path.join(base, "config"), "XDG_DATA_HOME": os.path.join(base, "data"),
            "XDG_RUNTIME_DIR": os.path.join(base, "run"), "UNOPKG": unopkg,
            "AXIOM_INSTALL_SKIP_LO_CHECK": "1", "AXIOM_WPS_ROOT": self.wps_root,
            "DBUS_SESSION_BUS_ADDRESS": "disabled:",
        }
        os.makedirs(self.env["XDG_RUNTIME_DIR"])
        self.publish = os.path.join(self.home, ".local", "share", "Kingsoft", "wps", "jsaddons", "publish.xml")

    def tearDown(self):
        self.tmp.cleanup()

    def set_oem(self, enabled: bool) -> None:
        with open(self.oem, "w") as handle:
            handle.write("[Support]\n%sDocPermission=false\n" % ("JsApiPlugin=true\n" if enabled else ""))

    def run_install(self, *args):
        result = subprocess.run(["bash", os.path.join(self.stage, "install.sh"), *args], env=self.env,
                                capture_output=True, text=True, timeout=120, cwd=self.stage)
        return result.returncode, result.stdout + result.stderr

    def core_dir(self):
        return os.path.join(self.env["XDG_DATA_HOME"], "axiom-office", "core")

    def seed_data(self):
        # Gia lap Core da chay: co hoi thoai + Bo nho AI trong core.db, WAL chua checkpoint.
        data = {"core.db": "hoi thoai + bo nho", "core.db-wal": "wal", "core.db-shm": "shm"}
        for name, text in data.items():
            with open(os.path.join(self.core_dir(), name), "w") as handle:
                handle.write(text)
        return data

    def assert_data_kept(self, data, out):
        for name, text in data.items():
            path = os.path.join(self.core_dir(), name)
            self.assertTrue(os.path.exists(path), name + " bi xoa - mat du lieu nguoi dung\n" + out)
            with open(path) as handle:
                self.assertEqual(handle.read(), text, name + " bi doi noi dung")

    def read_publish(self):
        try:
            with open(self.publish, encoding="utf-8") as handle:
                return handle.read()
        except OSError:
            return ""

    def test_registers_wps_addin_and_explains_admin_step(self):
        code, out = self.run_install()
        self.assertEqual(code, 0, out)
        text = self.read_publish()
        self.assertIn('name="axiomoffice"', text, out)
        self.assertIn("http://127.0.0.1:47840/wps/addin/", text)
        self.assertIn("JsApiPlugin", out, "oem.ini chua bat -> phai huong dan")
        self.assertIn("sudo", out)
        self.assertIn("--systemd", out, "nhac chay Core thuong truc (Core phai chay truoc WPS)")
        self.assertFalse(os.path.exists(os.path.join(self.home, ".config", "Kingsoft", "Office.conf")),
                         "khong duoc dung Office.conf cua nguoi dung")
        installed = os.path.join(self.env["XDG_DATA_HOME"], "axiom-office", "core", "wps-addin", "index.html")
        self.assertTrue(os.path.exists(installed), "Core cai kem wps-addin/")

    def test_no_admin_hint_when_js_addins_enabled(self):
        self.set_oem(True)
        code, out = self.run_install()
        self.assertEqual(code, 0, out)
        self.assertIn('name="axiomoffice"', self.read_publish())
        self.assertNotIn("sudo", out, "JsApiPlugin da bat thi khong in lenh sudo")

    def test_without_wps_or_with_no_wps_flag_nothing_is_written(self):
        shutil.rmtree(self.wps_root)
        code, out = self.run_install()
        self.assertEqual(code, 0, out)
        self.assertFalse(os.path.exists(os.path.join(self.home, ".local", "share", "Kingsoft")), out)
        os.makedirs(os.path.join(self.wps_root, "office6", "cfgs"))
        self.set_oem(True)
        code, out = self.run_install("--no-wps")
        self.assertEqual(code, 0, out)
        self.assertFalse(os.path.exists(self.publish), "--no-wps: khong dang ky add-in")

    def test_uninstall_removes_only_our_registration(self):
        self.set_oem(True)
        self.assertEqual(self.run_install()[0], 0)
        text = self.read_publish().replace("</jsplugins>", OTHER + "\n</jsplugins>")
        with open(self.publish, "w", encoding="utf-8") as handle:
            handle.write(text)
        code, out = self.run_install("--uninstall")
        self.assertEqual(code, 0, out)
        text = self.read_publish()
        self.assertNotIn('name="axiomoffice"', text, out)
        self.assertIn(OTHER, text, "giu add-in khac")

    def test_reinstall_keeps_user_data(self):
        self.assertEqual(self.run_install()[0], 0)
        data = self.seed_data()
        # Goi moi co tep moi; tep cu khong con trong goi phai bien mat (khong giu ca thu muc cu).
        stale = os.path.join(self.core_dir(), "tep-cua-ban-cu.txt")
        open(stale, "w").close()
        with open(os.path.join(self.stage, "core", "wps-addin", "mini.html"), "w") as handle:
            handle.write("<!-- moi -->\n")
        code, out = self.run_install()
        self.assertEqual(code, 0, out)
        self.assert_data_kept(data, out)
        self.assertTrue(os.path.exists(os.path.join(self.core_dir(), "wps-addin", "mini.html")), "phai cai ban moi")
        self.assertFalse(os.path.exists(stale), "tep cua ban cu khong co trong goi moi thi bo")

    def test_uninstall_keeps_data_unless_purge(self):
        self.assertEqual(self.run_install()[0], 0)
        data = self.seed_data()
        code, out = self.run_install("--uninstall")
        self.assertEqual(code, 0, out)
        self.assert_data_kept(data, out)
        self.assertFalse(os.path.exists(os.path.join(self.core_dir(), "AxiomOffice.Core")), "go phai xoa Core")
        self.assertFalse(os.path.exists(os.path.join(self.core_dir(), "wps-addin")), "go phai xoa add-in")
        # Cai lai sau khi go: du lieu van con, dung nhu loi hua "Giu ... du lieu".
        self.assertEqual(self.run_install()[0], 0)
        self.assert_data_kept(data, out)
        code, out = self.run_install("--uninstall", "--purge")
        self.assertEqual(code, 0, out)
        self.assertFalse(os.path.exists(os.path.join(self.env["XDG_DATA_HOME"], "axiom-office")),
                         "--purge xoa ca du lieu\n" + out)


if __name__ == "__main__":
    unittest.main(verbosity=2)

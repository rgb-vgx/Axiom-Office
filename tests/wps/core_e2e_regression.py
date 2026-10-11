"""Chay lai e2e Agent Core (cac phan CI Linux) tren binary build tu cay hien tai - lenh kiem khong can shell.

    python3 tests/wps/core_e2e_regression.py
"""
from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
PARTS = "fake_bridge,guards,skills,confirm,memory,mcp,visual,setup,shutdown,anthropic,embeddings,summarize,cancel,mcp_http,verify"


def main() -> int:
    work = tempfile.mkdtemp(prefix="axiom-core-regression-")
    try:
        exe = os.path.join(work, "axiom-core")
        built = subprocess.run(["go", "-C", os.path.join(ROOT, "core-go"), "build", "-o", exe, "./cmd/axiom-core"],
                               capture_output=True, text=True, timeout=600)
        if built.returncode != 0:
            print(built.stderr[-3000:])
            return 1
        shutil.copytree(os.path.join(ROOT, "skills"), os.path.join(work, "skills"))   # skill dung san canh binary (nhu CI)
        env = dict(os.environ, AXIOM_E2E_CORE_EXE=exe)
        return subprocess.run([sys.executable, os.path.join(ROOT, "tests", "core", "test_core_e2e.py"), "--only", PARTS],
                              env=env, timeout=1800).returncode
    finally:
        shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())

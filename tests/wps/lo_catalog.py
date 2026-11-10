"""In catalog lenh bridge (catalog/live-commands.json) dang JSON - nguon doi chieu cho registry lenh cua add-in WPS
(tests/wps/test_addin.mjs).

Ten file giu nguyen cho cac test dang goi; tu 25/10/2026 nguon la catalog JSON chu khong con la registry Python cua
extension LibreOffice (LibreOffice dung ho tro - docs/plans/19-h0).

    python3 tests/wps/lo_catalog.py
"""
from __future__ import annotations

import json
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
with open(os.path.join(ROOT, "catalog", "live-commands.json"), encoding="utf-8") as handle:
    catalog = json.load(handle)
# Windows: stdout cua tien trinh con la cp1252, json.dump(ensure_ascii=False) se chet voi ten lenh tieng Viet.
# Tu chuyen sang UTF-8 de test_addin/test_pane (execFileSync) va test_chat doc duoc o moi noi.
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")
json.dump(catalog, sys.stdout, ensure_ascii=False)

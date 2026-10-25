"""Spec: catalog/live-commands.json la nguon chuan cua lenh bridge (tu 25/10/2026).

- File dung dinh dang chuan cua scripts/generate_mcp_commands.py.
- Moi lenh writer.*/et.*/wpp.* (tru lenh tao/mo/dong tai lieu) nhan tham so chon tai lieu (docs/plans/19-h0 H1):
  thieu la agent lai lam tren tai lieu active - benchmark slide 25/10/2026 da ghi vao nham bai.
- Ban C# (AxiomOffice.Host.exe commands --json) khop file: cung ten, kind, co agent, tham so theo thu tu.
  Chay khi da build (scripts/build.ps1); chua build thi bo qua.

    python -m unittest discover -s tests/catalog -v
"""
from __future__ import annotations

import json
import os
import subprocess
import unittest

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
CATALOG = os.path.join(ROOT, "catalog", "live-commands.json")
HOST_EXE = os.path.join(ROOT, "src", "AxiomOffice", "bin", "Release", "AxiomOffice.Host.exe")

TARGET = {"wpp": "presentation", "et": "workbook", "wps": "document"}
# Lenh chi con trong catalog vi extension LibreOffice (da dung ho tro) tung co; ban C# khong co.
ONLY_LIBREOFFICE = {"et.closeAll", "wpp.closeAll", "ui.setup"}


def creates_or_closes(name: str) -> bool:
    return name.endswith((".open", ".newPresentation", ".newDocument", ".newWorkbook", ".closeAll")) \
        or name == "wpp.newFromTemplate"


def load() -> tuple[str, list]:
    with open(CATALOG, encoding="utf-8", newline="") as handle:
        text = handle.read().replace("\r\n", "\n")
    return text, json.loads(text)


class CatalogTests(unittest.TestCase):
    def test_file_is_in_canonical_format(self):
        text, items = load()
        self.assertEqual(text, json.dumps(items, ensure_ascii=False, indent=2) + "\n",
                         "chay scripts/generate_mcp_commands.py de chuan hoa")
        names = [item["name"] for item in items]
        self.assertEqual(len(names), len(set(names)), "trung ten lenh")

    def test_every_document_command_takes_a_target(self):
        _, items = load()
        missing = [item["name"] for item in items
                   if item["kind"] in TARGET and not creates_or_closes(item["name"])
                   and TARGET[item["kind"]] not in {p["name"] for p in item["params"]}]
        self.assertEqual(missing, [], "lenh thieu tham so chon tai lieu (19-h0 H1)")

    def test_csharp_matches_catalog(self):
        if not os.path.exists(HOST_EXE):
            self.skipTest("chua build AxiomOffice.Host.exe")
        raw = subprocess.run([HOST_EXE, "commands", "--json"], capture_output=True, text=True, encoding="utf-8",
                             timeout=60).stdout
        csharp = {item["name"]: item for item in json.loads(raw.lstrip("﻿"))}
        catalog = {item["name"]: item for item in load()[1]}
        self.assertEqual(set(catalog) - set(csharp), ONLY_LIBREOFFICE, "lenh trong catalog ma C# khong co")
        self.assertEqual(set(csharp) - set(catalog), set(), "lenh C# chua co trong catalog")
        for name, theirs in csharp.items():
            mine = catalog[name]
            self.assertEqual((mine["kind"], mine["agent"]), (theirs["kind"], theirs["agent"]), name)
            self.assertEqual([(p["name"], p["required"]) for p in mine["params"]],
                             [(p["name"], p["required"]) for p in theirs["params"]], name)


if __name__ == "__main__":
    unittest.main()

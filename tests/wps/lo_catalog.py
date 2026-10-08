"""In registry lenh cua extension LibreOffice (axiom.commands.catalog()) dang JSON - nguon doi chieu cho
registry lenh cua add-in WPS (tests/wps/test_addin.mjs). Khong can LibreOffice: thay uno bang module gia.

    python3 tests/wps/lo_catalog.py
"""
from __future__ import annotations

import json
import os
import sys
import types

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
if "uno" not in sys.modules:
    fake = types.ModuleType("uno")
    fake.createUnoStruct = lambda name: types.SimpleNamespace()
    fake.Enum = lambda kind, name: name
    fake.invoke = lambda obj, name, args: getattr(obj, name)(*args)
    fake.Any = lambda kind, value: value
    fake.fileUrlToSystemPath = lambda url: url
    fake.systemPathToFileUrl = lambda path: path
    fake.getConstantByName = lambda name: 0
    sys.modules["uno"] = fake
sys.path.insert(0, os.path.join(ROOT, "src", "AxiomOffice.LibreOffice", "python", "pythonpath"))
from axiom import commands  # noqa: E402

commands.load_all()
json.dump(commands.catalog(), sys.stdout, ensure_ascii=False)

"""In he thiet ke cua pane LibreOffice (axiom.theme) dang JSON - nguon doi chieu cho pane WPS
(tests/wps/test_pane.mjs). theme.py la ban LibreOffice cua PaneTheme (src/AxiomOffice/Ai/PaneControls.cs)
nen day chinh la "mot san pham, mot he thiet ke".

Khong can LibreOffice (theme.py thuan Python, chi import uno gian tiep qua goi axiom).

    python3 tests/wps/theme_dump.py
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
from axiom import theme  # noqa: E402

# Mau: hang so nguyen 6 chu so hex (0xFFFFFF). SAMPLES = 4 va IS_WINDOWS = bool khong lot qua day.
COLORS = {name: value for name, value in vars(theme).items() if type(value) is int and value > 0xFFFF}
# Kich thuoc chu: tinh bang diem (point) trong theme.py.
SIZES = {name: value for name, value in vars(theme).items() if name.startswith("SIZE_") and type(value) is float}
# Kich thuoc bo cuc: pixel o 96 DPI, tru hai ti le be rong bong bong (0..1).
METRICS = ("PAD_X", "GAP_MESSAGE", "GAP_BEFORE_TOOLS", "GAP_TOOL_LINE", "GAP_CHIP", "RADIUS_BUBBLE",
           "RADIUS_CONTROL", "RADIUS_COMPOSER", "BUBBLE_PAD_X", "BUBBLE_PAD_Y", "BUBBLE_MAX_USER",
           "BUBBLE_MAX_AI", "HEADER_H", "FOOTER_H", "COMPOSER_H", "SEND_W", "SEND_H", "CHIP_H",
           "TOOL_ROW_H", "ICON", "SCROLL_W")

LABEL_PROBES = ("", "writer.khongCo", "khongdau", "writer.save", "writer.saveAs", "et.exportPdf",
                "wpp.undo", "writer.appendText", "load_skill")

payload = {
    "colors": {name: "#%06X" % value for name, value in COLORS.items()},
    "sizes": SIZES,
    "metrics": {name: getattr(theme, name) for name in METRICS},
    "strings": {"placeholder": theme.PROMPT_PLACEHOLDER, "hint": theme.PROMPT_HINT,
                "hint_short": theme.PROMPT_HINT_SHORT},
    "labels": {"action": theme.ACTION_LABELS, "suffix": theme.SUFFIX_LABELS, "tool": theme.TOOL_LABELS},
    "label_probes": {probe: theme.label_for(probe) for probe in LABEL_PROBES},
    "apps": theme.APP_NAMES,
    "nouns": theme.NOUNS,
    "empty": {kind: {"title": theme.empty_state(kind)[0], "description": theme.empty_state(kind)[1],
                     "suggestions": list(theme.empty_state(kind)[2])} for kind in ("wps", "et", "wpp")},
}
json.dump(payload, sys.stdout, ensure_ascii=False)

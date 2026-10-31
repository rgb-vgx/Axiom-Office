"""Cham ket qua soat chinh ta theo bo de cases.json.

    python3 tests/bench/chinh-ta/cham.py tests/bench/chinh-ta/cases.json outputs.json [--json report.json]

outputs.json: {"<id>": "<van ban sau khi soat>"}. Dac ta: test_cham.py.
"""
from __future__ import annotations

import sys


def tokenize(text):
    raise NotImplementedError


def expected_text(case):
    raise NotImplementedError


def score_case(case, output):
    raise NotImplementedError


def score(cases, outputs):
    raise NotImplementedError


def main(argv=None):
    raise NotImplementedError


if __name__ == "__main__":
    sys.exit(main())

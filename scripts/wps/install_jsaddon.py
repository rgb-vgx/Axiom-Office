#!/usr/bin/env python3
"""Cai Axiom Office JS add-in cho WPS Office for Linux (offline).

Chi ghi trong HOME duoc chi dinh qua --home (khong bao gio dung ~ hay $HOME
cua nguoi dung that):

- chep moi file cua src/AxiomOffice.WPS/ vao
  <home>/.local/share/Kingsoft/wps/jsaddons/axiomoffice_0.1.0/
  (cai lai thi thay toan bo noi dung thu muc nay);
- sinh config.js o do: window.AXIOM_REPORT_URL = "<report-url>";
- dang ky add-in trong <home>/.../jsaddons/publish.xml (giu cac add-in khac,
  khong bao gio dang ky trung);
- ghi cac khoa Office.conf can thiet vao <home>/.config/Kingsoft/Office.conf
  muc [6.0] (giu cac dong khac).

Buoc admin mot lan tren moi may (JsApiPlugin=true trong oem.ini cua he thong)
KHONG thuoc trach nhiem cua trinh cai nay (xem tests/wps/REFERENCE.md).

    python3 scripts/wps/install_jsaddon.py --home <HOME> --report-url <URL>
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import sys

ADDON_NAME = "axiomoffice"
ADDON_VERSION = "0.1.0"
ADDON_DIRNAME = "%s_%s" % (ADDON_NAME, ADDON_VERSION)

ENTRY = (
    '<jsplugin name="axiomoffice" type="wps" url="axiomoffice_0.1.0"'
    ' version="0.1.0" enable="enable_dev" install="null" customDomain=""/>'
)

# Khoa Office.conf can thiet cho HOME moi (xem tests/wps/REFERENCE.md).
OFFICE_CONF_KEYS = [
    "common\\AcceptedEULA=true",
    "wpsoffice\\Application%20Settings\\AppComponentMode=prome_independ",
    "wpsoffice\\Application%20Settings\\AppComponentModeInstall=prome_independ",
]

SECTION = "[6.0]"


def repo_src_dir() -> str:
    here = os.path.abspath(os.path.dirname(__file__))
    return os.path.join(os.path.abspath(os.path.join(here, "..", "..")), "src", "AxiomOffice.WPS")


def install_files(src: str, dest: str) -> list:
    if os.path.isdir(dest):
        shutil.rmtree(dest)
    os.makedirs(dest, exist_ok=True)
    copied = []
    for entry in sorted(os.listdir(src)):
        source = os.path.join(src, entry)
        target = os.path.join(dest, entry)
        if os.path.isdir(source):
            shutil.copytree(source, target)
        elif os.path.isfile(source):
            shutil.copy2(source, target)
        else:
            continue
        copied.append(entry)
    return copied


def write_config(dest: str, report_url: str) -> None:
    path = os.path.join(dest, "config.js")
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write("window.AXIOM_REPORT_URL = %s;\n" % json.dumps(report_url))


def update_publish_xml(jsaddons: str) -> str:
    """Dam bao publish.xml co dung mot entry cua add-in nay. Tra ve duong dan file."""
    path = os.path.join(jsaddons, "publish.xml")
    if os.path.isfile(path):
        with open(path, encoding="utf-8", errors="replace") as handle:
            text = handle.read()
    else:
        text = ""
    if not text.strip():
        text = '<?xml version="1.0" encoding="UTF-8"?>\n<jsplugins>\n</jsplugins>\n'
    # Xoa moi dang ky cu cua add-in nay (tu dong hoac cap mo/dong, moi thu tu thuoc tinh).
    text = re.sub(
        r'<jsplugin\b(?=[^>]*\bname\s*=\s*["\']axiomoffice["\'])[^>]*>.*?</jsplugin\s*>',
        "", text, flags=re.DOTALL)
    text = re.sub(
        r'<jsplugin\b(?=[^>]*\bname\s*=\s*["\']axiomoffice["\'])[^>]*/?>',
        "", text)
    if "</jsplugins>" in text:
        text = text.replace("</jsplugins>", ENTRY + "\n</jsplugins>", 1)
    elif re.search(r"<jsplugins\b[^>]*/>", text):
        text = re.sub(r"<jsplugins\b[^>]*/>",
                      "<jsplugins>\n" + ENTRY + "\n</jsplugins>", text, count=1)
    elif re.search(r"<jsplugins\b[^>]*>", text):
        text = re.sub(r"(<jsplugins\b[^>]*>)", r"\1\n" + ENTRY, text, count=1)
    else:
        if text and not text.endswith("\n"):
            text += "\n"
        text += "<jsplugins>\n" + ENTRY + "\n</jsplugins>\n"
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(text)
    return path


def update_office_conf(home: str) -> str:
    """Ghi cac khoa can thiet vao muc [6.0] cua Office.conf, giu cac dong khac."""
    path = os.path.join(home, ".config", "Kingsoft", "Office.conf")
    if os.path.isfile(path):
        with open(path, encoding="utf-8", errors="replace") as handle:
            content = handle.read()
        lines = content.splitlines(keepends=True)
    else:
        lines = []

    def is_section(line: str) -> str | None:
        stripped = line.strip()
        if len(stripped) >= 2 and stripped.startswith("[") and stripped.endswith("]"):
            return stripped
        return None

    start = None
    end = None
    for index, line in enumerate(lines):
        section = is_section(line)
        if section is None:
            continue
        if section == SECTION and start is None:
            start = index
            continue
        if start is not None:
            end = index
            break
    if start is None:
        if lines and not lines[-1].endswith("\n"):
            lines[-1] = lines[-1] + "\n"
        lines.append(SECTION + "\n")
        for key in OFFICE_CONF_KEYS:
            lines.append(key + "\n")
    else:
        if end is None:
            end = len(lines)
        have = {}
        for index in range(start + 1, end):
            line = lines[index]
            if "=" in line and not line.strip().startswith((";", "#")):
                have[line.split("=", 1)[0].strip()] = index
        insert_at = end
        for key in OFFICE_CONF_KEYS:
            name = key.split("=", 1)[0]
            if name in have:
                lines[have[name]] = key + "\n"
            else:
                lines.insert(insert_at, key + "\n")
                insert_at += 1
                end += 1
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8", newline="\n") as handle:
        handle.writelines(lines)
    return path


def main(argv: list | None = None) -> int:
    parser = argparse.ArgumentParser(description="Cai Axiom Office JS add-in cho WPS (offline).")
    parser.add_argument("--home", required=True, help="HOME dich (chi ghi trong nay)")
    parser.add_argument("--report-url", required=True, help="URL nhan bao cao JSON cua add-in")
    args = parser.parse_args(argv)

    home = args.home
    if not os.path.isdir(home):
        print("khong co thu muc home: %s" % home, file=sys.stderr)
        return 1
    src = repo_src_dir()
    if not os.path.isdir(src):
        print("khong co thu muc nguon: %s" % src, file=sys.stderr)
        return 1

    jsaddons = os.path.join(home, ".local", "share", "Kingsoft", "wps", "jsaddons")
    dest = os.path.join(jsaddons, ADDON_DIRNAME)
    copied = install_files(src, dest)
    write_config(dest, args.report_url)
    publish = update_publish_xml(jsaddons)
    conf = update_office_conf(home)
    print("da cai %d file vao %s; publish.xml: %s; Office.conf: %s"
          % (len(copied), dest, publish, conf))
    return 0


if __name__ == "__main__":
    sys.exit(main())

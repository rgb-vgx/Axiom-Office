#!/usr/bin/env python3
"""Cai Axiom Office JS add-in cho WPS Office for Linux (ban online do Agent Core phuc vu).

Chi ghi trong HOME duoc chi dinh qua --home (khong bao gio dung ~ hay $HOME
cua nguoi dung that):

- dang ky add-in online trong <home>/.local/share/Kingsoft/wps/jsaddons/publish.xml
  <jspluginonline name="axiomoffice" type="wps" url="<core-url>/wps/addin/" .../>
  (giu cac add-in khac, go dang ky cu cua axiomoffice, cai lai thi thay URL cu);
- xoa thu muc add-in offline cu <jsaddons>/axiomoffice_* (Core phuc vu file add-in,
  trinh cai khong chep file add-in);
- chi ghi cac khoa Office.conf can thiet vao <home>/.config/Kingsoft/Office.conf
  muc [6.0] khi co --test-profile (giu cac dong khac).

Buoc admin mot lan tren moi may (JsApiPlugin=true trong oem.ini cua he thong)
KHONG thuoc trach nhiem cua trinh cai nay (xem tests/wps/REFERENCE.md).

    python3 scripts/wps/install_jsaddon.py --home <HOME> --core-url <URL> [--test-profile]
"""
from __future__ import annotations

import argparse
import os
import re
import shutil
import sys

ADDON_NAME = "axiomoffice"

# Khoa Office.conf can thiet cho HOME moi (xem tests/wps/REFERENCE.md).
OFFICE_CONF_KEYS = [
    "common\\AcceptedEULA=true",
    "wpsoffice\\Application%20Settings\\AppComponentMode=prome_independ",
    "wpsoffice\\Application%20Settings\\AppComponentModeInstall=prome_independ",
]

SECTION = "[6.0]"


def build_entry(core_url: str) -> str:
    base = core_url.rstrip("/")
    return (
        '<jspluginonline name="axiomoffice" type="wps" url="%s/wps/addin/"'
        ' debug="" enable="enable_dev" install="null" customDomain=""/>'
        % base
    )


def remove_old_offline_dirs(jsaddons: str) -> None:
    try:
        names = os.listdir(jsaddons)
    except OSError:
        return
    for name in names:
        if not name.startswith(ADDON_NAME + "_"):
            continue
        path = os.path.join(jsaddons, name)
        try:
            if os.path.isdir(path) and not os.path.islink(path):
                shutil.rmtree(path)
            else:
                os.remove(path)
        except OSError:
            pass


def update_publish_xml(jsaddons: str, core_url: str) -> str:
    """Dam bao publish.xml co dung mot entry online cua add-in nay. Tra ve duong dan file."""
    entry = build_entry(core_url)
    path = os.path.join(jsaddons, "publish.xml")
    os.makedirs(jsaddons, exist_ok=True)
    if os.path.isfile(path):
        with open(path, encoding="utf-8", errors="replace") as handle:
            text = handle.read()
    else:
        text = ""
    if not text.strip():
        text = '<?xml version="1.0" encoding="UTF-8"?>\n<jsplugins>\n</jsplugins>\n'
    # Xoa moi dang ky cu cua add-in nay (online va offline, tu dong hoac cap mo/dong).
    for tag in ("jspluginonline", "jsplugin"):
        text = re.sub(
            r'<%s\b(?=[^>]*\bname\s*=\s*["\']axiomoffice["\'])[^>]*>.*?</%s\s*>' % (tag, tag),
            "", text, flags=re.DOTALL)
        text = re.sub(
            r'<%s\b(?=[^>]*\bname\s*=\s*["\']axiomoffice["\'])[^>]*/?>' % tag,
            "", text)
    if "</jsplugins>" in text:
        text = text.replace("</jsplugins>", entry + "\n</jsplugins>", 1)
    elif re.search(r"<jsplugins\b[^>]*/>", text):
        text = re.sub(r"<jsplugins\b[^>]*/>",
                      "<jsplugins>\n" + entry + "\n</jsplugins>", text, count=1)
    elif re.search(r"<jsplugins\b[^>]*>", text):
        text = re.sub(r"(<jsplugins\b[^>]*>)", r"\1\n" + entry, text, count=1)
    else:
        if text and not text.endswith("\n"):
            text += "\n"
        text += "<jsplugins>\n" + entry + "\n</jsplugins>\n"
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
    parser = argparse.ArgumentParser(description="Cai Axiom Office JS add-in cho WPS (ban online do Core phuc vu).")
    parser.add_argument("--home", required=True, help="HOME dich (chi ghi trong nay)")
    parser.add_argument("--core-url", required=True, help="URL goc cua Agent Core (vi du http://127.0.0.1:47840)")
    parser.add_argument("--test-profile", action="store_true",
                        help="ghi cac khoa Office.conf can thiet cho profile thu nghiem")
    args = parser.parse_args(argv)

    home = args.home
    if not os.path.isdir(home):
        print("khong co thu muc home: %s" % home, file=sys.stderr)
        return 1

    jsaddons = os.path.join(home, ".local", "share", "Kingsoft", "wps", "jsaddons")
    remove_old_offline_dirs(jsaddons)
    publish = update_publish_xml(jsaddons, args.core_url)
    if args.test_profile:
        conf = update_office_conf(home)
        print("da dang ky online %s; publish.xml: %s; Office.conf: %s"
              % (args.core_url.rstrip("/"), publish, conf))
    else:
        print("da dang ky online %s; publish.xml: %s"
              % (args.core_url.rstrip("/"), publish))
    return 0


if __name__ == "__main__":
    sys.exit(main())

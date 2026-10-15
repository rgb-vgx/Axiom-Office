"""Kiem tra ban dich giao dien add-in Windows (src/AxiomOffice/Localization/Ui.cs).

Gom moi chuoi goc tieng Viet can dich - Ui.T("..."), bang nhan buoc (PaneControls.Labels/SuffixLabels),
ten loi cua nut Soat (RibbonChecks.Labels, CheckInfo.Title), nhan ribbon (label/screentip/supertip trong
RibbonUi.RibbonXml) va chu hien thi trong catalog/setup.json (title, subtitle, label, description, help...) -
roi bao khoa con thieu / thua trong src/AxiomOffice/lang/*.json.

    python scripts/check_i18n.py            # bao thieu/thua, ma thoat 1 neu thieu
    python scripts/check_i18n.py --keys     # in danh sach khoa (JSON) de dich ngon ngu moi
"""
import json
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(ROOT, 'src', 'AxiomOffice')
LANG = os.path.join(SRC, 'lang')
CATALOG = os.path.join(ROOT, 'catalog', 'setup.json')

CALL = re.compile(r'Ui\.T\("((?:[^"\\]|\\.)*)"\)')
LABEL_ROW = re.compile(r'^\s*\{ "[^"]+", "((?:[^"\\]|\\.)*)" \},?')
TITLE = re.compile(r'Title = "((?:[^"\\]|\\.)*)"')
XML_TEXT = re.compile(r'\b(?:label|screentip|supertip)=\\"((?:[^"\\]|\\[^"])*)\\"')
ESCAPES = {'n': '\n', 'r': '\r', 't': '\t', '"': '"', '\\': '\\', '0': '\0'}


def unescape(text):
    return re.sub(r'\\(.)', lambda m: ESCAPES.get(m.group(1), m.group(1)), text)


def xml_unescape(text):
    return text.replace('&quot;', '"').replace('&lt;', '<').replace('&gt;', '>').replace('&amp;', '&')


def source_keys():
    keys = set()
    for folder, _, files in os.walk(SRC):
        if os.sep + 'bin' in folder or os.sep + 'obj' in folder:
            continue
        for name in files:
            if not name.endswith('.cs'):
                continue
            path = os.path.join(folder, name)
            text = open(path, encoding='utf-8-sig').read()
            keys.update(unescape(m) for m in CALL.findall(text))
            if name in ('PaneControls.cs', 'RibbonChecks.cs'):
                for line in text.splitlines():
                    match = LABEL_ROW.match(line)
                    if match:
                        keys.add(unescape(match.group(1)))
                keys.update(unescape(m) for m in TITLE.findall(text))
            if name == 'RibbonUi.cs':
                start = text.index('public const string RibbonXml')
                end = text.index('"</customUI>";', start)
                for value in XML_TEXT.findall(text[start:end]):
                    keys.add(xml_unescape(unescape(value)))
    # Tieng Viet ma khong co chu cai co dau (vd "Ask AI", "Axiom Office") thi khong can dich nhung van giu cho day du.
    keys.discard('')
    keys.update(catalog_keys())
    return keys


# Chuoi hien thi trong catalog/setup.json (title, subtitle, label, description, help...).
# Bo qua cac truong ky thuat (id, URL, ten model...). Wizard dich luc hien thi (Ui.T(...))
# chu khong sua setup.json hay cac tep sinh tu no.
CATALOG_KEYS = {'title', 'subtitle', 'label', 'description', 'help', 'fixLabel'}


def catalog_keys():
    keys = set()
    try:
        data = json.load(open(CATALOG, encoding='utf-8-sig'))
    except (OSError, ValueError):
        return keys

    def walk(node):
        if isinstance(node, dict):
            for key, value in node.items():
                if key in CATALOG_KEYS and isinstance(value, str) and value.strip():
                    keys.add(value)
                else:
                    walk(value)
        elif isinstance(node, list):
            for item in node:
                walk(item)

    walk(data)
    return keys


def main():
    keys = source_keys()
    if '--keys' in sys.argv:
        sys.stdout.reconfigure(encoding='utf-8')
        print(json.dumps(sorted(keys), ensure_ascii=False, indent=1))
        return 0
    failed = False
    for name in sorted(os.listdir(LANG)) if os.path.isdir(LANG) else []:
        if not name.endswith('.json'):
            continue
        table = json.load(open(os.path.join(LANG, name), encoding='utf-8-sig'))
        if not table.get('_name'):
            print('%s: thieu "_name" (ten ngon ngu hien trong menu)' % name)
            failed = True
        have = {key for key in table if not key.startswith('_')}
        missing = sorted(keys - have)
        extra = sorted(have - keys)
        sys.stdout.reconfigure(encoding='utf-8')
        print('%s: %d khoa, thieu %d, thua %d' % (name, len(keys), len(missing), len(extra)))
        for key in missing:
            print('  THIEU: %r' % key)
        for key in extra:
            print('  THUA:  %r' % key)
        failed = failed or bool(missing)
    return 1 if failed else 0


if __name__ == '__main__':
    sys.exit(main())

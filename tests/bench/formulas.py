"""Doc CONG THUC (khong phai gia tri) cua mot sheet trong file .ods.

    python formulas.py <file.ods> <ten sheet> [so dong toi da]

Can thiet vi et.readRange chi tra GIA TRI: mot o ghi cung 1.82868 va mot o cong thuc ra
1.82868 trong giong het nhau khi doc gia tri. De bai benchmark liet "hardcode ket qua thay vi
tao cong thuc" vao danh sach loi nghiem trong, nen phai doc duoc cong thuc.
"""
import re
import sys
import zipfile

sys.stdout.reconfigure(encoding="utf-8")


def col_index(letters: str) -> int:
    value = 0
    for ch in letters.upper():
        value = value * 26 + (ord(ch) - ord("A") + 1)
    return value - 1


def rows_of(xml: str, sheet: str):
    """Tra ve list cac dong; moi dong la dict {col_index: (formula, text)}."""
    # Cat dung bang can tim.
    start = xml.find('table:name="%s"' % sheet)
    if start < 0:
        return []
    end = xml.find("</table:table>", start)
    body = xml[start:end]
    result = []
    for row_xml in re.findall(r"<table:table-row.*?(?:/>|</table:table-row>)", body, re.S):
        repeat = re.search(r'table:number-rows-repeated="(\d+)"', row_xml)
        count = int(repeat.group(1)) if repeat else 1
        count = min(count, 20000)          # dong trong lap rat nhieu lan: khong can
        cells = {}
        column = 0
        for cell_xml in re.findall(r"<table:table-cell.*?(?:/>|</table:table-cell>)", row_xml, re.S):
            span = re.search(r'table:number-columns-repeated="(\d+)"', cell_xml)
            width = int(span.group(1)) if span else 1
            formula = re.search(r'table:formula="([^"]*)"', cell_xml)
            text = re.search(r"<text:p[^>]*>([^<]*)</text:p>", cell_xml)
            value = re.search(r'office:value="([^"]*)"', cell_xml)
            if formula or text or value:
                cells[column] = (formula.group(1) if formula else "",
                                 text.group(1) if text else (value.group(1) if value else ""))
            column += width
            if column > 60:
                break
        for _ in range(count):
            result.append(cells)
    return result


def cell_ref(col: int, row: int) -> str:
    letters = ""
    col += 1
    while col:
        col, rem = divmod(col - 1, 26)
        letters = chr(65 + rem) + letters
    return "%s%d" % (letters, row + 1)


def main() -> int:
    path, sheet = sys.argv[1], sys.argv[2]
    limit = int(sys.argv[3]) if len(sys.argv) > 3 else 40
    xml = zipfile.ZipFile(path).read("content.xml").decode("utf-8")
    rows = rows_of(xml, sheet)
    formulas = texts = 0
    for index, row in enumerate(rows, 1):
        if index > limit:
            break
        parts = []
        for col in sorted(row):
            formula, text = row[col]
            if formula:
                formulas += 1
                parts.append("%s: %s" % (cell_ref(col, index - 1), formula.replace("of:=", "=")))
            elif text:
                texts += 1
                parts.append("%s: [%s]" % (cell_ref(col, index - 1), text[:40]))
        if parts:
            print("%3d | %s" % (index, "  ".join(parts)))
    print("--- %d cong thuc, %d o chu ---" % (formulas, texts))
    return 0


if __name__ == "__main__":
    sys.exit(main())

"""Tom tat tung sheet cua mot file .ods tu goi ODF: so dong, so cong thuc, so o chu.

    python ods_summary.py <file.ods>
"""
import re
import sys
import zipfile

sys.stdout.reconfigure(encoding="utf-8")


def main() -> int:
    path = sys.argv[1]
    xml = zipfile.ZipFile(path).read("content.xml").decode("utf-8")
    parts = re.split(r'<table:table [^>]*table:name="', xml)[1:]
    print("%-22s %8s %10s %8s %8s" % ("sheet", "dong", "cong thuc", "o chu", "chart?"))
    for part in parts:
        name = part.split('"')[0]
        body = part.split("</table:table>")[0]
        rows = len(re.findall(r"<table:table-row", body))
        # dong lap: cong them phan duoc lap (tru dong trong lap vo han)
        for match in re.finditer(r'<table:table-row[^>]*table:number-rows-repeated="(\d+)"', body):
            rows += min(int(match.group(1)), 20000) - 1
        formulas = body.count("table:formula=")
        texts = len(re.findall(r"<text:p[^>]*>", body))
        charts = body.count("<draw:object") + body.count("<draw:frame")
        print("%-22s %8d %10d %8d %8d" % (name, rows, formulas, texts, charts))
    return 0


if __name__ == "__main__":
    sys.exit(main())

"""Probe dinh dang dieu kien (conditional format) tren LibreOffice that, qua UNO.

Hai probe truoc (tests/bench/results/probe-conditional-format*.txt) ket luan "khong co duong nao tao
duoc entry". Chung chi thu NHUNG CHO SAI:

  - doc.createInstance("com.sun.star.sheet.TableConditionalEntry")   -> None
  - ctx.ServiceManager.createInstanceWithContext(...)                -> None
  - uno.createUnoStruct("com.sun.star.sheet.TableConditionalEntry")  -> addNew nem "getTypes"
  - sheet.ConditionalFormats.createInstance()                        -> vat CHUA, khong phai doi tuong

Bai nay di dung duong cua API, va moi buoc deu ghi lai vi sao buoc truoc sai:

  1. formats = sheet.ConditionalFormats
  2. tap vung: doc.createInstance("com.sun.star.sheet.SheetCellRanges") + addRangeAddress
     (sheet.getCellRangesByName khong co trong pyuno; sheet.Ranges chi la tuple Python)
  3. formats.createByRange(tap vung) -> tra ve ID (long), khong phai doi tuong
  4. formats.getConditionalFormats() -> day cac doi tuong theo vung; moi cai co createEntry()
  5. entry = doi_tuong.createEntry(); dat Operator/Formula1/StyleName; entries.addNew(entry)

Kiem chung khong phai "goi API khong loi" ma la: luu ra .ods roi dem <style:map>/table:condition
trong content.xml, va doc lai bang chinh API.

    "C:\\Program Files\\LibreOffice\\program\\python.exe" tests/bench/probe_cf3.py
"""
from __future__ import annotations

import os
import re
import subprocess
import sys
import tempfile
import time
import zipfile

SOFFICE = os.environ.get("AXIOM_SOFFICE_EXE") or r"C:\Program Files\LibreOffice\program\soffice.exe"
PORT = int(os.environ.get("AXIOM_PROBE_PORT", "2003"))

# Loai entry cho createEntry(nIndex, nType). Lay tu chinh enum cua UNO, khong viet cung so 0.
try:
    from com.sun.star.sheet.ConditionEntryType import CONDITION as CONDITION_TYPE
except ImportError:      # ban LibreOffice cu chua co enum nay
    CONDITION_TYPE = 0

RESULT: list[tuple[bool, str, str]] = []


def log(ok: bool, name: str, detail: str = "") -> None:
    RESULT.append((bool(ok), name, detail))
    print(("OK   " if ok else "FAIL ") + name + ("" if not detail else "\n     " + str(detail)[:800]))


def start_soffice() -> subprocess.Popen:
    profile = tempfile.mkdtemp(prefix="lo-cf3-")
    url = "file:///" + profile.replace("\\", "/").replace(" ", "%20")
    return subprocess.Popen([
        SOFFICE, "-env:UserInstallation=" + url, "--headless", "--norestore", "--nologo",
        "--nofirststartwizard", "--nodefault",
        "--accept=socket,host=127.0.0.1,port=%d;urp;" % PORT,
    ], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def connect(timeout: float = 90.0):
    import uno
    local = uno.getComponentContext()
    resolver = local.ServiceManager.createInstanceWithContext("com.sun.star.bridge.UnoUrlResolver", local)
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        try:
            ctx = resolver.resolve("uno:socket,host=127.0.0.1,port=%d;urp;StarOffice.ComponentContext" % PORT)
            desktop = ctx.ServiceManager.createInstanceWithContext("com.sun.star.frame.Desktop", ctx)
            return ctx, desktop
        except Exception as failure:
            last = failure
            time.sleep(1.0)
    raise SystemExit("khong ket noi duoc LibreOffice: %s" % last)


def main() -> int:
    process = start_soffice()
    try:
        ctx, desktop = connect()
        doc = desktop.loadComponentFromURL("private:factory/scalc", "_blank", 0, ())
        sheet = doc.Sheets.getByIndex(0)
        sheet.Name = "CF"
        for row in range(1, 11):
            sheet.getCellByPosition(0, row - 1).setValue(float(row * 10))   # A1..A10 = 10..100

        cell_range = sheet.getCellRangeByName("A1:A10")
        formats = sheet.ConditionalFormats

        ranges = doc.createInstance("com.sun.star.sheet.SheetCellRanges")
        log(ranges is not None, "tao duoc tap vung qua doc.createInstance",
            "SheetCellRanges ty le: %s" % type(ranges))
        ranges.addRangeAddress(cell_range.RangeAddress, False)

        # createByRange tra ve ID (long). Do la ly do hai probe truoc tuong "khong co doi tuong".
        created = formats.createByRange(ranges)
        log(isinstance(created, int), "createByRange tra ve ID (khong phai doi tuong)",
            "gia tri=%r" % (created,))

        group = formats.getConditionalFormats()
        log(isinstance(group, tuple) and len(group) >= 1,
            "getConditionalFormats() tra ve day doi tuong theo vung",
            "type=%s len=%s" % (type(group), len(group) if isinstance(group, tuple) else "?"))
        if not isinstance(group, tuple) or not group:
            return finish()

        holder = group[-1]
        names = sorted(n for n in dir(holder) if not n.startswith("_"))
        log("createEntry" in names, "doi tuong theo vung CO createEntry", str(names))

        # createEntry(nIndex, nType): hai tham so LONG (doc tu CoreReflection, khong doan).
        # Chinh doi tuong theo vung LA day entry (getByIndex/getCount), khong co addNew - nen tao entry
        # la da nam trong day, khong phai addNew nua.
        #
        # pyuno tra ve None cho createEntry du entry DA duoc tao that: do Count truoc/sau. Vi vay phai
        # lay entry bang getByIndex, khong dung gia tri tra ve. Day dung la cho hai probe truoc ket luan
        # sai ("khong tao duoc entry") - chung khong kiem Count.
        before = holder.Count
        returned = holder.createEntry(0, CONDITION_TYPE)
        after_create = holder.Count
        log(after_create == before + 1, "createEntry(0, CONDITION) TAO entry that (Count tang)",
            "truoc=%s sau=%s (gia tri tra ve cua pyuno la %r)" % (before, after_create, returned))

        entry = holder.getByIndex(0)
        log(entry is not None, "getByIndex(0) tra ve entry that", "type=%s" % type(entry))
        if entry is None:
            return finish()
        print("     entry methods: %s" % sorted(n for n in dir(entry) if not n.startswith("_"))[:22])

        from com.sun.star.sheet.ConditionOperator import GREATER
        entry.Operator = GREATER
        entry.Formula1 = "50"
        entry.StyleName = "Good"
        print("     entry: Operator=%s Formula1=%s StyleName=%s"
              % (entry.Operator, entry.Formula1, entry.StyleName))

        after = holder.Count
        log(after >= 1, "entry nam trong day cua vung", "Count=%s" % after)

        # Kiem cai THAT SU vao file.
        out_dir = tempfile.mkdtemp(prefix="lo-cf3-out-")
        out = os.path.join(out_dir, "probe.ods")
        from com.sun.star.beans import PropertyValue

        def prop(name, value):
            item = PropertyValue()
            item.Name = name
            item.Value = value
            return item

        doc.storeToURL("file:///" + out.replace("\\", "/"), (prop("FilterName", "calc8"),))
        with zipfile.ZipFile(out) as archive:
            content = archive.read("content.xml").decode("utf-8", "replace")
        maps = len(re.findall(r"<style:map\b", content))
        conditions = len(re.findall(r'table:condition="', content))
        applies = len(re.findall(r'style:apply-style-name="', content))
        log(maps >= 1 or conditions >= 1,
            "file .ods luu ra CO dinh dang dieu kien",
            "<style:map>=%d | table:condition=%d | apply-style-name=%d" % (maps, conditions, applies))
        print("     file kiem chung: %s" % out)

        # Va doc lai bang API tren chinh tai lieu dang mo.
        readback = sheet.getCellRangeByName("A1:A10").ConditionalFormat.Count
        log(readback >= 1, "doc lai tu tai lieu dang mo", "Count=%s" % readback)

        # Doi chieu DOC LAP: xuat sang .xlsx (dinh dang khac han, khong phai ODF) roi tim
        # <conditionalFormatting> trong part trang tinh. Neu ca hai dinh dang deu ghi ra thi dinh dang
        # dieu kien la that, khong phai chi la trang thai tam trong phien.
        xlsx = os.path.join(out_dir, "probe.xlsx")
        doc.storeToURL("file:///" + xlsx.replace("\\", "/"), (prop("FilterName", "Calc MS Excel 2007 XML"),))
        with zipfile.ZipFile(xlsx) as archive:
            sheets = [n for n in archive.namelist() if n.startswith("xl/worksheets/sheet")]
            found = 0
            for name in sheets:
                body = archive.read(name).decode("utf-8", "replace")
                found += len(re.findall(r"<conditionalFormatting\b", body))
            styles = archive.read("xl/styles.xml").decode("utf-8", "replace")
            dxfs = len(re.findall(r"<dxf\b", styles))
        log(found >= 1, "xuat sang .xlsx cung co dinh dang dieu kien",
            "<conditionalFormatting>=%d | <dxf>=%d" % (found, dxfs))
        print("     file kiem chung .xlsx: %s" % xlsx)
        return finish()
    finally:
        try:
            desktop.closeDesktop()
        except Exception:
            pass
        process.terminate()
        try:
            process.wait(timeout=20)
        except subprocess.TimeoutExpired:
            process.kill()


def finish() -> int:
    failed = [name for ok, name, _ in RESULT if not ok]
    print("\n%d ok, %d failed" % (len(RESULT) - len(failed), len(failed)))
    for name in failed:
        print("  FAIL " + name)
    return 0


if __name__ == "__main__":
    sys.exit(main())

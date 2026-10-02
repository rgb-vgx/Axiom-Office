"""Nhung chi tiet API con thieu de viet lenh et.setConditionalFormat (do 02/10/2026):

  - module hang so ConditionOperator sinh theo kieu LUOI: dir() chi co ten da duoc yeu cau qua fromlist
  - lay so nguyen tu hang so do bang cach nao
  - xoa roi tao lai dinh dang tren CUNG vung thi co bi chong khong

    "C:\\Program Files\\LibreOffice\\program\\python.exe" tests/bench/probe_cf5.py
"""
from __future__ import annotations

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from probe_cf3 import connect, start_soffice

NAMES = ["EQUAL", "NOT_EQUAL", "GREATER", "GREATER_EQUAL", "LESS", "LESS_EQUAL",
         "BETWEEN", "NOT_BETWEEN", "FORMULA"]


def main() -> int:
    process = start_soffice()
    try:
        ctx, desktop = connect()
        module = __import__("com.sun.star.sheet.ConditionOperator", fromlist=NAMES)
        print("dir sau fromlist:", sorted(n for n in dir(module) if not n.startswith("_")))
        integers = {}
        for name in NAMES:
            value = getattr(module, name, None)
            try:
                as_int = int(value)
            except Exception as failure:
                as_int = "int() loi: %r" % failure
            integers[name] = as_int
            print("   %-14s type=%-10s int=%s" % (name, type(value).__name__, as_int))

        doc = desktop.loadComponentFromURL("private:factory/scalc", "_blank", 0, ())
        sheet = doc.Sheets.getByIndex(0)
        formats = sheet.ConditionalFormats
        wanted = doc.createInstance("com.sun.star.sheet.SheetCellRanges")
        wanted.addRangeAddress(sheet.getCellRangeByName("A1:A10").RangeAddress, False)
        formats.createByRange(wanted)
        holder = formats.getConditionalFormats()[-1]
        holder.createEntry(holder.Count, 0)
        entry = holder.getByIndex(holder.Count - 1)
        entry.Operator = getattr(module, "GREATER")
        entry.Formula1 = "80"
        entry.StyleName = "Good"
        print("doc lai Operator =", entry.Operator, "| kieu", type(entry.Operator).__name__)

        # Xoa bang removeByID roi tao lai tren cung vung: co con dung 1 dinh dang khong?
        before = formats.getLength()
        formats.removeByID(holder.ID)
        after_remove = formats.getLength()
        formats.createByRange(wanted)
        after_again = formats.getLength()
        again = formats.getConditionalFormats()[-1]
        print("getLength: %s -> removeByID -> %s -> createByRange lai -> %s" % (before, after_remove, after_again))
        print("dinh dang moi co entry cu khong? Count =", again.Count, "| ID moi =", again.ID)
        return 0
    finally:
        try:
            desktop.closeDesktop()
        except Exception:
            pass
        process.terminate()


if __name__ == "__main__":
    sys.exit(main())

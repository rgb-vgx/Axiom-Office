from __future__ import annotations

import json

from mcp.server.fastmcp import FastMCP

from excel_mcp import bridge, file_tools

mcp = FastMCP("excel-tools")


@mcp.tool()
def excel_profile(path: str, sheet: str | None = None) -> str:
    """Inspect a spreadsheet file (.xlsx .xlsm .xls .csv .tsv): sheet names, row/column counts, header row and a small sample per sheet."""
    return file_tools.to_json(file_tools.profile(path, sheet))


@mcp.tool()
def excel_read(path: str, cell_range: str | None = None, sheet: str | None = None,
               offset: int = 0, limit: int = 100) -> str:
    """Read a block of cells (values only) from a spreadsheet file (.xlsx .xlsm .xls .csv .tsv). cell_range like 'A1:C50'. Page large areas with offset/limit."""
    return file_tools.to_json(file_tools.read_range(path, sheet, cell_range, offset, limit))


@mcp.tool()
def excel_query(path: str, sql: str, sheet: str | None = None, limit: int = 1000) -> str:
    """Run DuckDB SQL over one sheet of a spreadsheet file (.xlsx .xlsm .xls .csv .tsv). The sheet is registered as table "data" (first row = header). Example: SELECT Ten, SUM(Diem) FROM data GROUP BY Ten."""
    return file_tools.to_json(file_tools.query(path, sql, sheet, limit))


@mcp.tool()
def excel_write(path: str, sheet: str, start_cell: str, values: list[list]) -> str:
    """Write a 2D block of values into a spreadsheet file at start_cell (e.g. 'B2'), preserving the rest of the file. Supports .xlsx .xlsm (macros kept) .csv .tsv; not .xls. Strings starting with '=' become formulas. Creates the file/sheet if missing."""
    return file_tools.to_json(file_tools.write_range(path, sheet, start_cell, values))


@mcp.tool()
def excel_create(path: str, sheets: list[dict]) -> str:
    """Create a new spreadsheet file (.xlsx .xlsm .csv .tsv). sheets example: [{"name": "Data", "values": [["Ten", "Diem"], ["An", 9.5]]}]."""
    return file_tools.to_json(file_tools.create_workbook(path, sheets))


@mcp.tool()
def excel_convert(path: str, sheet: str | None = None, to: str = "parquet") -> str:
    """Convert one sheet of a spreadsheet file (.xlsx .xlsm .xls .csv .tsv) to parquet or csv next to the source file. Use for very large sheets, then query the parquet for speed."""
    return file_tools.to_json(file_tools.convert(path, sheet, to))


def _safe(callable_):
    try:
        return json.dumps(callable_(), ensure_ascii=False, default=str)
    except Exception as ex:
        return json.dumps({"ok": False, "error": str(ex)}, ensure_ascii=False)


@mcp.tool()
def wps_health(app: str = "wps") -> str:
    """Check the live WPS bridge connection. app: wps (Writer), et (Spreadsheets), wpp (Presentation)."""
    return _safe(lambda: bridge.health(app))


@mcp.tool()
def wps_live_command(app: str, action: str, params: dict | None = None, port: int | None = None) -> str:
    """Send any command to the live WPS bridge (works on the document the user has open). Apps: wps=47821, et=47822, wpp=47823. Examples: action='app.info'; action='et.readRange' params={'range':'A1:C10'}; action='et.writeRange' params={'range':'A1','values':[[1,2],[3,4]]}; action='writer.typeText' params={'text':'hello'}; action='writer.saveAs' params={'path':'C:/tmp/out.docx'}."""
    return _safe(lambda: bridge.command(app, action, params, port))


@mcp.tool()
def wps_live_read_range(cell_range: str, app: str = "et", sheet: str | None = None) -> str:
    """Read a range from the spreadsheet currently open in WPS (live). cell_range like 'A1:F100'."""
    return _safe(lambda: bridge.command(app, "et.readRange", {"range": cell_range, "sheet": sheet}))


@mcp.tool()
def wps_live_write_range(cell_range: str, values: list[list], app: str = "et", sheet: str | None = None) -> str:
    """Write a 2D block into the spreadsheet currently open in WPS (live). cell_range is the top-left anchor like 'A1'."""
    return _safe(lambda: bridge.command(app, "et.writeRange", {"range": cell_range, "values": values, "sheet": sheet}))


def main() -> None:
    mcp.run()


if __name__ == "__main__":
    main()

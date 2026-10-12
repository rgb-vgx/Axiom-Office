from __future__ import annotations

import json

from mcp.server.mcpserver import MCPServer

from excel_mcp import bridge, file_tools

mcp = MCPServer("excel-tools")


@mcp.tool()
def excel_profile(path: str, sheet: str | None = None) -> str:
    """Inspect a spreadsheet file (.xlsx .xlsm .xls .csv .tsv): sheet names, row/column counts, header row and a small sample per sheet."""
    return file_tools.to_json(file_tools.profile(path, sheet))


@mcp.tool()
def excel_read(path: str, cell_range: str | None = None, sheet: str | None = None,
               offset: int = 0, limit: int = 100, show_formula: bool = False) -> str:
    """Read a block of cells from a spreadsheet file (.xlsx .xlsm .xls .csv .tsv). cell_range like 'A1:C50'. Page large areas with offset/limit. show_formula=True returns formulas instead of cached values (xlsx/xlsm only)."""
    return file_tools.to_json(file_tools.read_range(path, sheet, cell_range, offset, limit, show_formula))


@mcp.tool()
def excel_create_sheet(path: str, sheet: str, overwrite: bool = False) -> str:
    """Create a new worksheet in an existing workbook (.xlsx/.xlsm). Atomic save; openpyxl may drop charts/images/pivots on resave."""
    return file_tools.to_json(file_tools.create_sheet(path, sheet, overwrite))


@mcp.tool()
def excel_copy_sheet(path: str, src_sheet: str, dst_sheet: str) -> str:
    """Copy a worksheet inside the same workbook (values + styles; charts/images may not be preserved). Atomic save."""
    return file_tools.to_json(file_tools.copy_sheet(path, src_sheet, dst_sheet))


@mcp.tool()
def excel_rename_sheet(path: str, sheet: str, new_name: str) -> str:
    """Rename a worksheet. Atomic save; openpyxl may drop charts/images/pivots on resave."""
    return file_tools.to_json(file_tools.rename_sheet(path, sheet, new_name))


@mcp.tool()
def excel_delete_sheet(path: str, sheet: str) -> str:
    """Delete a worksheet (cannot delete the only sheet in the workbook). Atomic save; openpyxl may drop charts/images/pivots on resave."""
    return file_tools.to_json(file_tools.delete_sheet(path, sheet))


@mcp.tool()
def excel_format_range(path: str, sheet: str, cell_range: str, styles: dict | list) -> str:
    """Format cells in a range (.xlsx/.xlsm). styles = one style object applied to every cell OR a 2D array matching the range size (null entries skip that cell). Style object keys: font {bold, italic, underline, size, strike, color '#RRGGBB', name, vertAlign}, fill {pattern 'solid', color '#RRGGBB'}, border [{type: left|right|top|bottom|diagonalUp|diagonalDown, style: thin|medium|thick|double|dashed|dotted|hair|mediumDashed|dashDot|... , color}], alignment {horizontal, vertical, wrap, rotation}, numFmt (number format string), decimalPlaces (0-30). Atomic save; openpyxl may drop charts/images/pivots on resave."""
    return file_tools.to_json(file_tools.format_range(path, sheet, cell_range, styles))


@mcp.tool()
def excel_create_table(path: str, sheet: str, cell_range: str, table_name: str) -> str:
    """Create an Excel table (ListObject) over a range with a header row, e.g. cell_range 'A1:D10'. table_name: letters/digits/underscore. Atomic save; openpyxl may drop charts/images/pivots on resave."""
    return file_tools.to_json(file_tools.create_table(path, sheet, cell_range, table_name))


@mcp.tool()
def excel_query(path: str, sql: str, sheet: str | None = None, limit: int = 1000) -> str:
    """Run DuckDB SQL over one sheet of a spreadsheet file (.xlsx .xlsm .xls .csv .tsv). The sheet is registered as table "data" (first row = header). Example: SELECT Ten, SUM(Diem) FROM data GROUP BY Ten."""
    return file_tools.to_json(file_tools.query(path, sql, sheet, limit))


@mcp.tool()
def excel_write(path: str, sheet: str, start_cell: str, values: list[list]) -> str:
    """Write a 2D block of values into a spreadsheet file at start_cell (e.g. 'B2'), preserving the rest of the file. Supports .xlsx .xlsm (macros kept) .csv .tsv; not .xls. Strings starting with '=' become formulas. Creates the file/sheet if missing. Saves atomically (temp + replace). Note: openpyxl may drop charts/images/pivot tables — for files currently open in WPS/Office use wps_live_write_range."""
    return file_tools.to_json(file_tools.write_range(path, sheet, start_cell, values))


@mcp.tool()
def excel_create(path: str, sheets: list[dict]) -> str:
    """Create a new spreadsheet file (.xlsx .xlsm .csv .tsv). sheets example: [{"name": "Data", "values": [["Ten", "Diem"], ["An", 9.5]]}]. Saves atomically."""
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
def wps_health(app: str = "wps", port: int | None = None) -> str:
    """Check the live bridge. app: wps/et/wpp (WPS Office) or word/excel/ppt (Microsoft Office); or pass an explicit port."""
    return _safe(lambda: bridge.health(app, port))


@mcp.tool()
def office_sessions() -> str:
    """List all live Office/WPS bridge sessions: app, port, host, open document."""
    return _safe(bridge.sessions)


@mcp.tool()
def wps_live_command(app: str, action: str, params: dict | None = None, port: int | None = None) -> str:
    """Send any command to the live bridge (works on the document the user has open). Apps: wps=47821, et=47822, wpp=47823 (WPS); word=47831, excel=47832, ppt=47833 (Microsoft Office). Examples: action='app.info'; action='writer.insertStyledText' params={'text':'hello','bold':true,'color':'#FF0000'}; action='writer.heading' params={'level':1,'text':'Title'}; action='writer.insertTable' params={'rows':2,'cols':2,'values':[[1,2],[3,4]]}; action='writer.exportPdf' params={'path':'C:/tmp/out.pdf'}; action='et.formatRange' params={'range':'A1:B1','bold':true,'fillColor':'#FFFF00'}; action='et.readRange' params={'range':'A1:C10'}; action='et.writeRange' params={'range':'A1','values':[[1,2],[3,4]]}; action='wpp.addSlide' params={'layout':1}; action='wpp.addText' params={'text':'Hi','fontSize':28,'color':'#FF0000'}; action='wpp.addTable' params={'rows':2,'cols':3}; action='wpp.setNotes' params={'text':'notes'}; action='writer.undo' OR action='et.undo'. Tip: call office_sessions() to list every live bridge (Office + WPS) and pass its port here to target that exact instance."""
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

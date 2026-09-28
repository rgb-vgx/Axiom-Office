from __future__ import annotations

import json

from mcp.server.mcpserver import MCPServer

from word_mcp import bridge, file_tools

mcp = MCPServer("word-tools")


def _safe(callable_):
    try:
        return json.dumps(callable_(), ensure_ascii=False, default=str)
    except Exception as ex:
        return json.dumps({"ok": False, "error": str(ex)}, ensure_ascii=False)


def _clean(params: dict) -> dict:
    return {key: value for key, value in params.items() if value is not None}


def _live(action: str, params: dict | None = None, app: str = "word", port: int | None = None) -> str:
    return _safe(lambda: bridge.command(app, action, params, port))


@mcp.tool()
def doc_profile(path: str) -> str:
    """Inspect a .docx file: paragraph/table/section counts, style usage, core properties."""
    return file_tools.to_json(file_tools.profile(path))


@mcp.tool()
def doc_get_text(path: str, max_chars: int = 0, include_tables: bool = True) -> str:
    """Extract all text from a .docx file (paragraphs + tables), optionally truncated."""
    return file_tools.to_json(file_tools.get_text(path, max_chars, include_tables))


@mcp.tool()
def doc_find_text(path: str, query: str, max_results: int = 50) -> str:
    """Find paragraphs containing a query string in a .docx file."""
    return file_tools.to_json(file_tools.find_text(path, query, max_results))


@mcp.tool()
def doc_extract_table(path: str, index: int = 0) -> str:
    """Extract one table (by index) from a .docx file as a 2D array."""
    return file_tools.to_json(file_tools.extract_table(path, index))


@mcp.tool()
def doc_create(path: str, paragraphs: list | None = None, title: str | None = None,
               author: str | None = None, overwrite: bool = False) -> str:
    """Create a .docx file. paragraphs items: "text" | {"text": "...", "style": "Heading 1"} | {"table": [[...]]}. Atomic save."""
    return file_tools.to_json(file_tools.create(path, paragraphs, title, author, overwrite))


@mcp.tool()
def word_health(app: str = "word", port: int | None = None) -> str:
    """Check the live Word bridge. app: word (Microsoft Word, 47831) or wps (WPS Writer, 47821)."""
    return _safe(lambda: bridge.health(app, port))


@mcp.tool()
def office_sessions() -> str:
    """List all live Office/WPS bridge sessions: app, port, host, open document."""
    return _safe(bridge.sessions)


@mcp.tool()
def word_command(action: str, params: dict | None = None, app: str = "word", port: int | None = None) -> str:
    """Send any bridge command to the live Word/WPS Writer session. Examples: writer.getText; writer.undo; writer.replaceAll params={'find':'a','replace':'b'}; writer.exportPdf params={'path':'C:/tmp/out.pdf'}. Tip: call office_sessions() to list every live bridge (Office + WPS) and pass its port here to target that exact instance."""
    return _live(action, params, app, port)


@mcp.tool()
def word_read_text(app: str = "word", max_chars: int = 0) -> str:
    """Read the full text of the document currently open in Word/WPS."""
    return _live("writer.getText", _clean({"maxChars": max_chars or None}), app)


@mcp.tool()
def word_type_text(text: str, app: str = "word") -> str:
    """Type text at the current cursor position in the open document."""
    return _live("writer.typeText", {"text": text}, app)


@mcp.tool()
def word_insert_styled_text(text: str, bold: bool | None = None, italic: bool | None = None,
                            underline: bool | None = None, size: int | None = None,
                            color: str | None = None, font: str | None = None, app: str = "word") -> str:
    """Insert text with formatting (color as '#RRGGBB'). Each call is one Ctrl+Z step."""
    return _live("writer.insertStyledText", _clean({
        "text": text, "bold": bold, "italic": italic, "underline": underline,
        "size": size, "color": color, "font": font,
    }), app)


@mcp.tool()
def word_format_selection(bold: bool | None = None, italic: bool | None = None,
                          underline: bool | None = None, size: int | None = None,
                          color: str | None = None, font: str | None = None,
                          alignment: str | None = None, app: str = "word") -> str:
    """Format the currently selected text (alignment: left/center/right/justify)."""
    return _live("writer.formatSelection", _clean({
        "bold": bold, "italic": italic, "underline": underline, "size": size,
        "color": color, "font": font, "alignment": alignment,
    }), app)


@mcp.tool()
def word_heading(text: str | None = None, level: int = 1, app: str = "word") -> str:
    """Insert a heading (Heading 1-9 style) with an automatic paragraph break."""
    return _live("writer.heading", _clean({"text": text, "level": level}), app)


@mcp.tool()
def word_insert_table(rows: int, cols: int, values: list | None = None,
                      style: str | None = None, app: str = "word") -> str:
    """Insert a table at the cursor with optional 2D values and an optional table style (e.g. 'Table Grid')."""
    return _live("writer.insertTable", _clean({"rows": rows, "cols": cols, "values": values, "style": style}), app)


@mcp.tool()
def word_insert_image(path: str, width: int | None = None, height: int | None = None, app: str = "word") -> str:
    """Insert an image at the cursor (path to png/jpg; width/height in points)."""
    return _live("writer.insertImage", _clean({"path": path, "width": width, "height": height}), app)


@mcp.tool()
def word_insert_hyperlink(url: str, text: str | None = None, app: str = "word") -> str:
    """Insert a hyperlink at the cursor."""
    return _live("writer.insertHyperlink", _clean({"url": url, "text": text}), app)


@mcp.tool()
def word_replace_all(find: str, replace: str = "", app: str = "word") -> str:
    """Find and replace all occurrences in the open document (one undo step)."""
    return _live("writer.replaceAll", {"find": find, "replace": replace}, app)


@mcp.tool()
def word_export_pdf(path: str, app: str = "word") -> str:
    """Export the open document to PDF."""
    return _live("writer.exportPdf", {"path": path}, app)


@mcp.tool()
def word_undo(count: int = 1, app: str = "word") -> str:
    """Undo the last N AI actions (each AI action is a single undo record)."""
    return _live("writer.undo", {"count": count}, app)


@mcp.tool()
def word_save(path: str | None = None, app: str = "word") -> str:
    """Save the open document (optionally to a new path)."""
    if path:
        return _live("writer.saveAs", {"path": path}, app)
    return _live("writer.save", None, app)


def main() -> None:
    mcp.run()


if __name__ == "__main__":
    main()

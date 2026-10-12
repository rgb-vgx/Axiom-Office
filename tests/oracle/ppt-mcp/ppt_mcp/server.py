from __future__ import annotations

import json

from mcp.server.mcpserver import MCPServer

from ppt_mcp import bridge, file_tools

mcp = MCPServer("ppt-tools")


def _safe(callable_):
    try:
        return json.dumps(callable_(), ensure_ascii=False, default=str)
    except Exception as ex:
        return json.dumps({"ok": False, "error": str(ex)}, ensure_ascii=False)


def _clean(params: dict) -> dict:
    return {key: value for key, value in params.items() if value is not None}


def _live(action: str, params: dict | None = None, app: str = "ppt", port: int | None = None) -> str:
    return _safe(lambda: bridge.command(app, action, params, port))


@mcp.tool()
def ppt_profile(path: str) -> str:
    """Inspect a .pptx file: slide count, per-slide layout, shapes, texts and speaker notes."""
    return file_tools.to_json(file_tools.profile(path))


@mcp.tool()
def ppt_get_text(path: str, max_chars: int = 0) -> str:
    """Extract all text (slides + notes) from a .pptx file."""
    return file_tools.to_json(file_tools.get_text(path, max_chars))


@mcp.tool()
def ppt_create(path: str, slides: list | None = None, overwrite: bool = False) -> str:
    """Create a .pptx file. slides items: {"title": "...", "bullets": ["...", "..."], "layout": 1}. Atomic save."""
    return file_tools.to_json(file_tools.create(path, slides, overwrite))


@mcp.tool()
def ppt_add_slide_file(path: str, title: str | None = None, bullets: list | None = None, layout: int = 1) -> str:
    """Add a slide to a .pptx file on disk (offline mode)."""
    return file_tools.to_json(file_tools.add_slide(path, title, bullets, layout))


@mcp.tool()
def ppt_health(app: str = "ppt", port: int | None = None) -> str:
    """Check the live PowerPoint bridge. app: ppt (Microsoft PowerPoint, 47833) or wpp (WPS Presentation, 47823)."""
    return _safe(lambda: bridge.health(app, port))


@mcp.tool()
def office_sessions() -> str:
    """List all live Office/WPS bridge sessions: app, port, host, open document."""
    return _safe(bridge.sessions)


@mcp.tool()
def ppt_command(action: str, params: dict | None = None, app: str = "ppt", port: int | None = None) -> str:
    """Send any bridge command to the live PowerPoint/WPS session. Examples: wpp.listSlides; wpp.exportPdf params={'path':'C:/tmp/out.pdf'}; wpp.saveAs params={'path':'C:/tmp/out.pptx'}. Tip: call office_sessions() to list every live bridge (Office + WPS) and pass its port here to target that exact instance."""
    return _live(action, params, app, port)


@mcp.tool()
def ppt_list_slides(app: str = "ppt") -> str:
    """List slides (count + texts) of the presentation currently open in PowerPoint/WPS."""
    return _live("wpp.listSlides", None, app)


@mcp.tool()
def ppt_add_slide(layout: int = 12, app: str = "ppt") -> str:
    """Add a slide to the live presentation. layout: 1=title, 2=title+text, 12=blank (default)."""
    return _live("wpp.addSlide", {"layout": layout}, app)


@mcp.tool()
def ppt_add_text(text: str, slide: int | None = None, left: int | None = None, top: int | None = None,
                 width: int | None = None, height: int | None = None, font_size: int | None = None,
                 bold: bool | None = None, color: str | None = None, align: str | None = None,
                 app: str = "ppt") -> str:
    """Add a formatted text box to a live slide (color '#RRGGBB', align left/center/right)."""
    return _live("wpp.addText", _clean({
        "text": text, "slide": slide, "left": left, "top": top, "width": width,
        "height": height, "fontSize": font_size, "bold": bold, "color": color, "align": align,
    }), app)


@mcp.tool()
def ppt_add_image(path: str, slide: int | None = None, left: int | None = None, top: int | None = None,
                  width: int | None = None, height: int | None = None, app: str = "ppt") -> str:
    """Insert an image into a live slide (natural size when width/height omitted)."""
    return _live("wpp.addImage", _clean({
        "path": path, "slide": slide, "left": left, "top": top, "width": width, "height": height,
    }), app)


@mcp.tool()
def ppt_add_table(rows: int, cols: int, values: list | None = None, slide: int | None = None,
                  left: int | None = None, top: int | None = None, width: int | None = None,
                  height: int | None = None, app: str = "ppt") -> str:
    """Add a table with optional 2D values to a live slide."""
    return _live("wpp.addTable", _clean({
        "rows": rows, "cols": cols, "values": values, "slide": slide,
        "left": left, "top": top, "width": width, "height": height,
    }), app)


@mcp.tool()
def ppt_set_notes(text: str, slide: int | None = None, app: str = "ppt") -> str:
    """Set speaker notes of a live slide."""
    return _live("wpp.setNotes", _clean({"text": text, "slide": slide}), app)


@mcp.tool()
def ppt_delete_slide(slide: int | None = None, app: str = "ppt") -> str:
    """Delete a slide from the live presentation (default: last slide)."""
    return _live("wpp.deleteSlide", _clean({"slide": slide}), app)


@mcp.tool()
def ppt_export_pdf(path: str, app: str = "ppt") -> str:
    """Export the live presentation to PDF."""
    return _live("wpp.exportPdf", {"path": path}, app)


@mcp.tool()
def ppt_save(path: str | None = None, app: str = "ppt") -> str:
    """Save the live presentation (optionally to a new path)."""
    if path:
        return _live("wpp.saveAs", {"path": path}, app)
    return _live("wpp.save", None, app)


def main() -> None:
    mcp.run()


if __name__ == "__main__":
    main()

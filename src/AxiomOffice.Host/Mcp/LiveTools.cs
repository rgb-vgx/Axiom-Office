using System;
using System.Collections.Generic;

namespace AxiomOffice.Host.Mcp
{
    // Làn live: gửi lệnh tới bridge trong Word/Excel/PowerPoint hoặc WPS đang mở.
    // Lỗi kết nối trả {"ok":false,"error":...} như bản Python (không phải lỗi MCP).
    internal static class LiveTools
    {
        private const string PortTip = " Tip: call office_sessions() to list every live bridge (Office + WPS) and pass its port here to target that exact instance.";

        public static McpTool Sessions()
        {
            return new McpTool("office_sessions",
                "List all live Office/WPS bridge sessions: app, port, host, open document.",
                new Param[0],
                a => Safe(() => Json.Serialize(BridgeClient.Sessions())));
        }

        public static IEnumerable<McpTool> Word()
        {
            yield return new McpTool("word_health",
                "Check the live Word bridge. app: word (Microsoft Word, 47831) or wps (WPS Writer, 47821).",
                new[] { AppParam("word"), PortParam() },
                a => Safe(() => BridgeClient.Health(a.Str("app", "word"), a.IntOpt("port"))));
            yield return new McpTool("word_command",
                "Send any bridge command to the live Word/WPS Writer session. Examples: writer.getText; writer.undo; writer.replaceAll params={'find':'a','replace':'b'}; writer.exportPdf params={'path':'C:/tmp/out.pdf'}." + PortTip,
                new[] { Param.Str("action", null, true), Param.Obj("params"), AppParam("word"), PortParam() },
                a => Live(a, "word", a.Req("action"), a.Obj("params"), a.IntOpt("port")));
            yield return new McpTool("word_read_text",
                "Read the full text of the document currently open in Word/WPS.",
                new[] { AppParam("word"), Param.Int("max_chars", null, false, 0) },
                a => Live(a, "word", "writer.getText", Clean("maxChars", a.Int("max_chars", 0) > 0 ? (object)a.Int("max_chars", 0) : null)));
            yield return new McpTool("word_type_text",
                "Type text at the current cursor position in the open document.",
                new[] { Param.Str("text", null, true), AppParam("word") },
                a => Live(a, "word", "writer.typeText", Clean("text", a.Req("text"))));
            yield return new McpTool("word_insert_styled_text",
                "Insert text with formatting (color as '#RRGGBB'). Each call is one Ctrl+Z step.",
                new[] { Param.Str("text", null, true), Param.Bool("bold"), Param.Bool("italic"), Param.Bool("underline"), Param.Int("size"), Param.Str("color"), Param.Str("font"), AppParam("word") },
                a => Live(a, "word", "writer.insertStyledText", Clean("text", a.Req("text"), "bold", a.BoolOpt("bold"), "italic", a.BoolOpt("italic"),
                    "underline", a.BoolOpt("underline"), "size", a.IntOpt("size"), "color", a.Str("color"), "font", a.Str("font"))));
            yield return new McpTool("word_format_selection",
                "Format the currently selected text (alignment: left/center/right/justify).",
                new[] { Param.Bool("bold"), Param.Bool("italic"), Param.Bool("underline"), Param.Int("size"), Param.Str("color"), Param.Str("font"), Param.Str("alignment"), AppParam("word") },
                a => Live(a, "word", "writer.formatSelection", Clean("bold", a.BoolOpt("bold"), "italic", a.BoolOpt("italic"), "underline", a.BoolOpt("underline"),
                    "size", a.IntOpt("size"), "color", a.Str("color"), "font", a.Str("font"), "alignment", a.Str("alignment"))));
            yield return new McpTool("word_heading",
                "Insert a heading (Heading 1-9 style) with an automatic paragraph break.",
                new[] { Param.Str("text"), Param.Int("level", null, false, 1), AppParam("word") },
                a => Live(a, "word", "writer.heading", Clean("text", a.Str("text"), "level", a.Int("level", 1))));
            yield return new McpTool("word_insert_table",
                "Insert a table at the cursor with optional 2D values and an optional table style (e.g. 'Table Grid').",
                new[] { Param.Int("rows", null, true), Param.Int("cols", null, true), Param.Matrix("values"), Param.Str("style"), AppParam("word") },
                a => Live(a, "word", "writer.insertTable", Clean("rows", a.Int("rows", 0), "cols", a.Int("cols", 0), "values", a.Raw("values"), "style", a.Str("style"))));
            yield return new McpTool("word_insert_image",
                "Insert an image at the cursor (path to png/jpg; width/height in points).",
                new[] { Param.Str("path", null, true), Param.Int("width"), Param.Int("height"), AppParam("word") },
                a => Live(a, "word", "writer.insertImage", Clean("path", a.Req("path"), "width", a.IntOpt("width"), "height", a.IntOpt("height"))));
            yield return new McpTool("word_insert_hyperlink",
                "Insert a hyperlink at the cursor.",
                new[] { Param.Str("url", null, true), Param.Str("text"), AppParam("word") },
                a => Live(a, "word", "writer.insertHyperlink", Clean("url", a.Req("url"), "text", a.Str("text"))));
            yield return new McpTool("word_replace_all",
                "Find and replace all occurrences in the open document (one undo step).",
                new[] { Param.Str("find", null, true), Param.Str("replace", null, false, ""), AppParam("word") },
                a => Live(a, "word", "writer.replaceAll", Clean("find", a.Req("find"), "replace", a.Str("replace", ""))));
            yield return new McpTool("word_export_pdf",
                "Export the open document to PDF.",
                new[] { Param.Str("path", null, true), AppParam("word") },
                a => Live(a, "word", "writer.exportPdf", Clean("path", a.Req("path"))));
            yield return new McpTool("word_undo",
                "Undo the last N AI actions (each AI action is a single undo record).",
                new[] { Param.Int("count", null, false, 1), AppParam("word") },
                a => Live(a, "word", "writer.undo", Clean("count", a.Int("count", 1))));
            yield return new McpTool("word_save",
                "Save the open document (optionally to a new path).",
                new[] { Param.Str("path"), AppParam("word") },
                a => a.Has("path")
                    ? Live(a, "word", "writer.saveAs", Clean("path", a.Str("path")))
                    : Live(a, "word", "writer.save", null));
        }

        public static IEnumerable<McpTool> Ppt()
        {
            yield return new McpTool("ppt_health",
                "Check the live PowerPoint bridge. app: ppt (Microsoft PowerPoint, 47833) or wpp (WPS Presentation, 47823).",
                new[] { AppParam("ppt"), PortParam() },
                a => Safe(() => BridgeClient.Health(a.Str("app", "ppt"), a.IntOpt("port"))));
            yield return new McpTool("ppt_command",
                "Send any bridge command to the live PowerPoint/WPS session. Examples: wpp.listSlides; wpp.exportPdf params={'path':'C:/tmp/out.pdf'}; wpp.saveAs params={'path':'C:/tmp/out.pptx'}." + PortTip,
                new[] { Param.Str("action", null, true), Param.Obj("params"), AppParam("ppt"), PortParam() },
                a => Live(a, "ppt", a.Req("action"), a.Obj("params"), a.IntOpt("port")));
            yield return new McpTool("ppt_list_slides",
                "List slides (count + texts) of the presentation currently open in PowerPoint/WPS.",
                new[] { AppParam("ppt") },
                a => Live(a, "ppt", "wpp.listSlides", null));
            yield return new McpTool("ppt_add_slide",
                "Add a slide to the live presentation. layout: 1=title, 2=title+text, 12=blank (default).",
                new[] { Param.Int("layout", null, false, 12), AppParam("ppt") },
                a => Live(a, "ppt", "wpp.addSlide", Clean("layout", a.Int("layout", 12))));
            yield return new McpTool("ppt_add_text",
                "Add a formatted text box to a live slide (color '#RRGGBB', align left/center/right).",
                new[] { Param.Str("text", null, true), Param.Int("slide"), Param.Int("left"), Param.Int("top"), Param.Int("width"), Param.Int("height"),
                    Param.Int("font_size"), Param.Bool("bold"), Param.Str("color"), Param.Str("align"), AppParam("ppt") },
                a => Live(a, "ppt", "wpp.addText", Clean("text", a.Req("text"), "slide", a.IntOpt("slide"), "left", a.IntOpt("left"), "top", a.IntOpt("top"),
                    "width", a.IntOpt("width"), "height", a.IntOpt("height"), "fontSize", a.IntOpt("font_size"), "bold", a.BoolOpt("bold"),
                    "color", a.Str("color"), "align", a.Str("align"))));
            yield return new McpTool("ppt_add_image",
                "Insert an image into a live slide (natural size when width/height omitted).",
                new[] { Param.Str("path", null, true), Param.Int("slide"), Param.Int("left"), Param.Int("top"), Param.Int("width"), Param.Int("height"), AppParam("ppt") },
                a => Live(a, "ppt", "wpp.addImage", Clean("path", a.Req("path"), "slide", a.IntOpt("slide"), "left", a.IntOpt("left"), "top", a.IntOpt("top"),
                    "width", a.IntOpt("width"), "height", a.IntOpt("height"))));
            yield return new McpTool("ppt_add_table",
                "Add a table with optional 2D values to a live slide.",
                new[] { Param.Int("rows", null, true), Param.Int("cols", null, true), Param.Matrix("values"), Param.Int("slide"), Param.Int("left"), Param.Int("top"),
                    Param.Int("width"), Param.Int("height"), AppParam("ppt") },
                a => Live(a, "ppt", "wpp.addTable", Clean("rows", a.Int("rows", 0), "cols", a.Int("cols", 0), "values", a.Raw("values"), "slide", a.IntOpt("slide"),
                    "left", a.IntOpt("left"), "top", a.IntOpt("top"), "width", a.IntOpt("width"), "height", a.IntOpt("height"))));
            yield return new McpTool("ppt_set_notes",
                "Set speaker notes of a live slide.",
                new[] { Param.Str("text", null, true), Param.Int("slide"), AppParam("ppt") },
                a => Live(a, "ppt", "wpp.setNotes", Clean("text", a.Req("text"), "slide", a.IntOpt("slide"))));
            yield return new McpTool("ppt_delete_slide",
                "Delete a slide from the live presentation (default: last slide).",
                new[] { Param.Int("slide"), AppParam("ppt") },
                a => Live(a, "ppt", "wpp.deleteSlide", Clean("slide", a.IntOpt("slide"))));
            yield return new McpTool("ppt_export_pdf",
                "Export the live presentation to PDF.",
                new[] { Param.Str("path", null, true), AppParam("ppt") },
                a => Live(a, "ppt", "wpp.exportPdf", Clean("path", a.Req("path"))));
            yield return new McpTool("ppt_save",
                "Save the live presentation (optionally to a new path).",
                new[] { Param.Str("path"), AppParam("ppt") },
                a => a.Has("path")
                    ? Live(a, "ppt", "wpp.saveAs", Clean("path", a.Str("path")))
                    : Live(a, "ppt", "wpp.save", null));
        }

        public static IEnumerable<McpTool> Excel()
        {
            yield return new McpTool("wps_health",
                "Check the live bridge. app: wps/et/wpp (WPS Office) or word/excel/ppt (Microsoft Office); or pass an explicit port.",
                new[] { AppParam("wps"), PortParam() },
                a => Safe(() => BridgeClient.Health(a.Str("app", "wps"), a.IntOpt("port"))));
            yield return new McpTool("wps_live_command",
                "Send any command to the live bridge (works on the document the user has open). Apps: wps=47821, et=47822, wpp=47823 (WPS); word=47831, excel=47832, ppt=47833 (Microsoft Office). " +
                "Examples: action='app.info'; action='writer.insertStyledText' params={'text':'hello','bold':true,'color':'#FF0000'}; action='writer.heading' params={'level':1,'text':'Title'}; " +
                "action='writer.insertTable' params={'rows':2,'cols':2,'values':[[1,2],[3,4]]}; action='writer.exportPdf' params={'path':'C:/tmp/out.pdf'}; " +
                "action='et.formatRange' params={'range':'A1:B1','bold':true,'fillColor':'#FFFF00'}; action='et.readRange' params={'range':'A1:C10'}; " +
                "action='et.writeRange' params={'range':'A1','values':[[1,2],[3,4]]}; action='wpp.addSlide' params={'layout':1}; " +
                "action='wpp.addText' params={'text':'Hi','fontSize':28,'color':'#FF0000'}; action='wpp.addTable' params={'rows':2,'cols':3}; " +
                "action='wpp.setNotes' params={'text':'notes'}; action='writer.undo' OR action='et.undo'." + PortTip,
                new[] { Param.Str("app", "wps/et/wpp or word/excel/ppt", true), Param.Str("action", null, true), Param.Obj("params"), PortParam() },
                a => Live(a, a.Req("app"), a.Req("action"), a.Obj("params"), a.IntOpt("port")));
            yield return new McpTool("wps_live_read_range",
                "Read a range from the spreadsheet currently open in WPS (live). cell_range like 'A1:F100'.",
                new[] { Param.Str("cell_range", null, true), AppParam("et"), Param.Str("sheet") },
                a => Live(a, "et", "et.readRange", Clean("range", a.Req("cell_range"), "sheet", a.Str("sheet"))));
            yield return new McpTool("wps_live_write_range",
                "Write a 2D block into the spreadsheet currently open in WPS (live). cell_range is the top-left anchor like 'A1'.",
                new[] { Param.Str("cell_range", null, true), Param.Matrix("values", null, true), AppParam("et"), Param.Str("sheet") },
                a => Live(a, "et", "et.writeRange", Clean("range", a.Req("cell_range"), "values", a.Raw("values"), "sheet", a.Str("sheet"))));
        }

        private static Param AppParam(string def)
        {
            return Param.Str("app", "wps/et/wpp (WPS) or word/excel/ppt (Microsoft Office)", false, def);
        }

        private static Param PortParam()
        {
            return Param.Int("port", "explicit bridge port (from office_sessions)");
        }

        private static string Live(ToolArgs a, string defaultApp, string action, Dictionary<string, object> parameters, int? port = null)
        {
            return Safe(() => BridgeClient.Command(a.Str("app", defaultApp), action, parameters, port));
        }

        // Cặp key/value; bỏ các giá trị null (giống _clean của bản Python).
        private static Dictionary<string, object> Clean(params object[] pairs)
        {
            var result = new Dictionary<string, object>();
            for (int i = 0; i + 1 < pairs.Length; i += 2)
            {
                if (pairs[i + 1] != null)
                {
                    result[(string)pairs[i]] = pairs[i + 1];
                }
            }
            return result;
        }

        private static string Safe(Func<string> action)
        {
            try
            {
                return action();
            }
            catch (Exception ex)
            {
                return Json.Serialize(new Dictionary<string, object> { { "ok", false }, { "error", ex.Message } });
            }
        }
    }
}

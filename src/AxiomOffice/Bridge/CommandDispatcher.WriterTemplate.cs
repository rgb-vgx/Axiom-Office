using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;

namespace AxiomOffice.Bridge
{
    // Bảng dàn trang có định dạng từng dòng (docs/plans/21-v0 P3): phần đầu văn bản hành chính (cơ quan | quốc hiệu, tiêu
    // ngữ, ngày tháng) và phần ký (nơi nhận | chức vụ, họ tên) theo NĐ 30/2020. Trước đây chỉ có insertTable + formatTable
    // cả bảng: không đậm được riêng "CỘNG HÒA..." mà để "Số: ..." thường, hay nghiêng riêng dòng ngày tháng.
    internal static partial class CommandDispatcher
    {
        private static IEnumerable<CommandInfo> WriterTemplateCommands()
        {
            return new[]
            {
                Command("writer.insertLayoutTable", "wps", WriterInsertLayoutTable,
                    "Bảng dàn trang KHÔNG viền ở CUỐI bài, định dạng từng dòng trong ô - phần đầu văn bản hành chính (cơ quan | quốc hiệu, tiêu ngữ, ngày tháng) và phần ký (nơi nhận | chức vụ, họ tên)",
                    Req("rows", "2D array of cells; a cell is a string ('\\n' = new line) or {lines: [string | {t, bold, italic, underline, size, upper}], align: center|left|right}"),
                    Opt("widths", "column widths in percent, e.g. [40, 60]; default equal"),
                    Opt("font", "default Times New Roman"), Opt("size", "default 13")).ForAgent()
            };
        }

        private sealed class LayoutLine
        {
            public string Text;
            public bool Bold;
            public bool Italic;
            public bool Underline;
            public int Size;
        }

        private sealed class LayoutCell
        {
            public List<LayoutLine> Lines = new List<LayoutLine>();
            public int Align = 1;
        }

        private static LayoutLine ParseLayoutLine(object raw, int size)
        {
            raw = UnwrapScalar(raw);
            var entry = raw as Dictionary<string, object>;
            if (entry == null)
            {
                return new LayoutLine { Text = Convert.ToString(raw) ?? "", Size = size };
            }
            string text = ParamString(entry, "t", null) ?? ParamString(entry, "text", "") ?? "";
            if (ParamBool(entry, "upper", false))
            {
                text = text.ToUpper(new System.Globalization.CultureInfo("vi-VN"));
            }
            return new LayoutLine
            {
                Text = text,
                Bold = ParamBool(entry, "bold", false),
                Italic = ParamBool(entry, "italic", false),
                Underline = ParamBool(entry, "underline", false),
                Size = ParamInt(entry, "size", size)
            };
        }

        private static LayoutCell ParseLayoutCell(object raw, int size)
        {
            raw = UnwrapScalar(raw);
            var cell = new LayoutCell();
            var entry = raw as Dictionary<string, object>;
            IEnumerable lines;
            if (entry != null)
            {
                cell.Align = AlignmentOf(ParamString(entry, "align", "center"), 1);
                object value;
                entry.TryGetValue("lines", out value);
                value = UnwrapScalar(value);
                lines = value is IEnumerable && !(value is string) ? (IEnumerable)value
                    : (Convert.ToString(value) ?? "").Replace("\r\n", "\n").Split('\n');
            }
            else if (raw is IEnumerable && !(raw is string))
            {
                lines = (IEnumerable)raw;
            }
            else
            {
                lines = (Convert.ToString(raw) ?? "").Replace("\r\n", "\n").Split('\n');
            }
            foreach (object line in lines)
            {
                cell.Lines.Add(ParseLayoutLine(line, size));
            }
            if (cell.Lines.Count == 0)
            {
                cell.Lines.Add(new LayoutLine { Text = "", Size = size });
            }
            return cell;
        }

        private static Dictionary<string, object> WriterInsertLayoutTable(IAppHost host, Dictionary<string, object> p)
        {
            // Không UnwrapScalar mảng ngoài: bảng một hàng [[a, b]] sẽ bị gỡ thành [a, b] (ô thành hàng).
            object raw;
            if (p == null || !p.TryGetValue("rows", out raw) || !(raw is IEnumerable) || raw is string || raw is IDictionary)
            {
                throw new ArgumentException("'rows' must be a 2D array of cells, e.g. [[\"TRƯỜNG THPT A\", \"CỘNG HÒA XÃ HỘI CHỦ NGHĨA VIỆT NAM\"]]");
            }
            int size = ParamInt(p, "size", 13);
            string font = ParamString(p, "font", "Times New Roman");
            var grid = new List<List<LayoutCell>>();
            foreach (object row in (IEnumerable)raw)
            {
                object cells = row;
                if (!(cells is IEnumerable) || cells is string || cells is IDictionary)
                {
                    throw new ArgumentException("row " + (grid.Count + 1) + " must be an array of cells");
                }
                grid.Add(((IEnumerable)cells).Cast<object>().Select(cell => ParseLayoutCell(cell, size)).ToList());
            }
            int rows = grid.Count;
            int cols = rows == 0 ? 0 : grid.Max(r => r.Count);
            if (rows == 0 || cols == 0)
            {
                throw new ArgumentException("'rows' is empty");
            }
            List<int> widths = ParamIntList(p, "widths");
            if (widths.Count > 0 && (widths.Count != cols || widths.Any(w => w <= 0)))
            {
                throw new ArgumentException("'widths' needs " + cols + " positive percentages, got [" + string.Join(", ", widths) + "]");
            }
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            int index;
            using (new UndoRecordScope(app, "AI: layout table"))
            {
                app.Selection.EndKey(6);
                dynamic table = doc.Tables.Add(TableAnchor(app), rows, cols);
                table.Borders.Enable = 0;
                MarkLayoutTable(table, true);
                table.Range.Font.Name = font;
                table.Range.ParagraphFormat.SpaceBefore = 0;
                table.Range.ParagraphFormat.SpaceAfter = 0;
                // wdLineSpaceSingle = 0.
                table.Range.ParagraphFormat.LineSpacingRule = 0;
                if (widths.Count > 0)
                {
                    // wdPreferredWidthPercent = 2.
                    table.PreferredWidthType = 2;
                    table.PreferredWidth = 100;
                    double total = widths.Sum();
                    for (int c = 0; c < cols; c++)
                    {
                        dynamic column = table.Columns[c + 1];
                        column.PreferredWidthType = 2;
                        column.PreferredWidth = (float)(100.0 * widths[c] / total);
                    }
                }
                for (int r = 0; r < rows; r++)
                {
                    for (int c = 0; c < cols; c++)
                    {
                        LayoutCell cell = c < grid[r].Count ? grid[r][c] : new LayoutCell { Lines = { new LayoutLine { Text = "", Size = size } } };
                        dynamic range = table.Cell(r + 1, c + 1).Range;
                        range.Text = string.Join("\r", cell.Lines.Select(l => l.Text));
                        dynamic paragraphs = table.Cell(r + 1, c + 1).Range.Paragraphs;
                        for (int i = 0; i < cell.Lines.Count && i < Convert.ToInt32(paragraphs.Count); i++)
                        {
                            LayoutLine line = cell.Lines[i];
                            dynamic paragraph = paragraphs[i + 1];
                            paragraph.Alignment = cell.Align;
                            dynamic text = paragraph.Range;
                            text.Font.Bold = line.Bold ? -1 : 0;
                            text.Font.Italic = line.Italic ? -1 : 0;
                            text.Font.Underline = line.Underline ? 1 : 0;
                            text.Font.Size = line.Size;
                        }
                    }
                }
                index = Convert.ToInt32(doc.Range(0, Convert.ToInt32(table.Range.End)).Tables.Count);
                // Con trỏ ra sau bảng: phần viết tiếp (writeContent, appendText) nằm dưới bảng.
                dynamic after = table.Range;
                after.Collapse(0);
                after.Select();
            }
            return new Dictionary<string, object> { { "table", index }, { "rows", rows }, { "cols", cols }, { "layout", true } };
        }
    }
}

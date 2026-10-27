using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;

namespace AxiomOffice.Bridge
{
    internal static partial class CommandDispatcher
    {
        // Excel / WPS Spreadsheets (AppKind "et").
        private static IEnumerable<CommandInfo> SpreadsheetCommands()
        {
            return new[]
            {
                Command("et.newWorkbook", "et", EtNewWorkbook, "Tạo workbook mới"),
                Command("et.open", "et", EtOpen, "Mở .xlsx/.xls/.csv", Req("path")),
                Command("et.listSheets", "et", EtListSheets, "Danh sách sheet + sheet đang active").ForAgent(),
                Command("et.sheetInfo", "et", EtSheetInfo,
                    "Kích thước dùng thật của TỪNG sheet trong một lời gọi (`usedRange`/`rows`/`cols`/`empty`) — " +
                    "dùng để soát sheet nào thực sự có nội dung, không chỉ có tên").ForAgent(),
                Command("et.addSheet", "et", EtAddSheet, "Thêm sheet mới (bỏ trống `name` thì đặt tên Sheet1, Sheet2...)",
                    Opt("name"), Opt("index", "0-based, default: append")).ForAgent(),
                Command("et.renameSheet", "et", EtRenameSheet, "Đổi tên sheet", Req("sheet"), Req("name")).ForAgent(),
                Command("et.deleteSheet", "et", EtDeleteSheet,
                    "Xoá một sheet (sheet thừa như Sheet1 của sổ mới, sheet chính lượt này dựng sai); không xoá được sheet cuối cùng",
                    Req("sheet")).ForAgent(),
                Command("et.activateSheet", "et", EtActivateSheet, "Chuyển sheet", Req("sheet")).ForAgent(),
                Command("et.readRange", "et", EtReadRange,
                    "Đọc vùng, ví dụ `A1:C10`; `formulas=true` trả về công thức (ô không có công thức = null)",
                    Req("range"), Opt("sheet"), Opt("formulas", "true = trả mảng `formulas` thay cho `values`")).ForAgent(),
                Command("et.readRanges", "et", EtReadRanges,
                    "Đọc NHIỀU vùng (nhiều sheet) trong MỘT lời gọi: `reads` là mảng {range, sheet?, formulas?}; `range` nhận cả "
                    + "'Sheet!A1:B6'. Dùng thay cho nhiều lần et.readRange khi soát lại (tối đa 4000 ô mỗi lần)",
                    Req("reads", "array of {range, sheet?, formulas?}"), Opt("sheet", "default sheet for items without one"),
                    Opt("formulas", "true = formulas for every item")).ForAgent(),
                Command("et.whatIf", "et", EtWhatIf,
                    "Thử ô điều khiển SỐNG hay CHẾT trong MỘT lời gọi (chỉ đọc): đổi tạm ô giả định `cell` thành `value`, tính lại, "
                    + "đọc các vùng `watch` trước/sau, rồi TỰ TRẢ LẠI giá trị cũ. `unchanged` = vùng không đổi theo (công thức không "
                    + "đọc ô đó). Thay cho tự ghi giá trị thử -> đọc -> ghi lại",
                    Req("cell", "one input cell e.g. 'Config!B2'"), Req("value", "test value e.g. 0.95"),
                    Req("watch", "ranges that should follow e.g. ['Dashboard!B3:B6']"), Opt("sheet")).ForAgent(),
                Command("et.writeRange", "et", EtWriteRange, "Ghi vùng bắt đầu từ ô trên-trái `range`",
                    Req("range", "top-left cell e.g. 'A1'"), Req("values", "2D array of rows e.g. [[\"Tên\",\"Điểm\"],[\"An\",9.5]]"), Opt("sheet")).ForAgent(),
                Command("et.importCsv", "et", EtImportCsv,
                    "Nạp một file CSV vào sổ: không có `range` thì tạo sheet mới đặt tên theo tên file, có `range` thì ghi vào ô góc đó",
                    Req("path", "e.g. 'D:\\data\\raw.csv'"), Opt("range", "top-left cell; bỏ trống thì tạo sheet mới"),
                    Opt("sheet"), Opt("delimiter", "một ký tự, mặc định ','"), Opt("encoding", "mặc định utf-8-sig")).ForAgent(),
                Command("et.writeRanges", "et", EtWriteRanges,
                    "Ghi NHIỀU vùng trong MỘT lời gọi: `writes` là mảng {range, values, sheet?}. Dùng khi cần viết nhiều khối trong cùng một phản hồi (bớt vòng qua bridge). Kiểm hết tham số trước khi ghi: một vùng sai thì không vùng nào được ghi",
                    Req("writes", "array of {range, values, sheet?}"), Opt("sheet")).ForAgent(),
                Command("et.fillRange", "et", EtFillRange,
                    "Viết MỘT công thức vào ô góc trên-trái của `range` rồi điền ra cả vùng, tham chiếu tương đối tự dịch (dùng cho bảng nghìn dòng: đừng gửi từng ô)",
                    Req("range", "the whole area e.g. 'B2:H1000'"), Opt("formula", "written to the top-left cell first"), Opt("sheet")).ForAgent(),
                Command("et.formatRange", "et", EtFormatRange, "Định dạng MỘT vùng (màu dạng `#RRGGBB`, `horizontal` left/center/right); nhiều vùng thì dùng et.formatRanges",
                    Req("range"), Opt("bold"), Opt("italic"), Opt("fontSize"), Opt("fontColor"), Opt("fillColor"), Opt("numFmt"), Opt("horizontal"), Opt("wrap"),
                    Opt("columnWidth", "width of the range's columns in characters, e.g. 20"), Opt("sheet")).ForAgent(),
                Command("et.formatRanges", "et", EtFormatRanges,
                    "Định dạng NHIỀU vùng trong MỘT lời gọi: `formats` là mảng {range, sheet?, bold, italic, fontSize, fontColor, "
                    + "fillColor, numFmt, horizontal, wrap, columnWidth} (cùng thuộc tính như et.formatRange). Dùng thay cho nhiều lần "
                    + "et.formatRange (tiêu đề, ô nhập liệu, định dạng số của mọi sheet trong một lần). Kiểm hết trước khi định dạng: "
                    + "một mục sai thì không vùng nào bị đổi",
                    Req("formats", "array of {range, sheet?, ...format}"), Opt("sheet", "default sheet for items without one")).ForAgent(),
                Command("et.addChart", "et", EtAddChart,
                    "Chèn biểu đồ từ vùng dữ liệu; `type` column/bar/line/pie/area/scatter, `width`/`height` tính bằng cm",
                    Req("range", "source data e.g. 'A1:B13'"), Opt("type", "column (default)/bar/line/pie/area/scatter"),
                    Opt("title"), Opt("name"), Opt("anchor", "top-left cell, default 'A1'"),
                    Opt("width", "cm, default 12"), Opt("height", "cm, default 7"), Opt("sheet")).ForAgent(),
                Command("et.listCharts", "et", EtListCharts, "Danh sách biểu đồ trên sheet", Opt("sheet")).ForAgent(),
                Command("et.setConditionalFormat", "et", EtSetConditionalFormat,
                    "Đặt định dạng điều kiện cho vùng: `rules` là mảng rule, mỗi rule có operator "
                    + "(less/lessEqual/greater/greaterEqual/equal/notEqual/between/notBetween/formula), formula1, "
                    + "formula2 (chỉ between), và bold/italic/fontColor/fillColor; HOẶC `type` trực quan: colorScale "
                    + "{colors: 2-3 màu thấp→cao}, dataBar {color}, iconSet {icons: 3arrows/3traffic/3flags/3symbols/"
                    + "4arrows/5arrows}, top/bottom {rank, percent} + màu, aboveAverage/belowAverage + màu, duplicate/"
                    + "unique + màu. Gọi lại trên cùng vùng thì THAY rule cũ của vùng đó, nên nhiều rule trên một vùng "
                    + "phải để trong MỘT lần gọi",
                    Req("range"), Req("rules", "array of rule objects"), Opt("sheet")).ForAgent(),
                Command("et.listConditionalFormats", "et", EtListConditionalFormats,
                    "Liệt kê định dạng điều kiện đang có (đọc lại để tự kiểm); bỏ trống `range` thì soi vùng đang dùng",
                    Opt("range", "chỉ xem một vùng"), Opt("sheet")).ForAgent(),
                Command("et.undo", "et", EtUndo, "Hoàn tác", Opt("count")).ForAgent(),
                Command("et.exportPdf", "et", EtExportPdf, "Xuất PDF", Req("path")).ForAgent(),
                Command("et.save", "et", (host, p) => SaveDocument(host, "et", null), "Lưu").ForAgent(),
                Command("et.saveAs", "et", (host, p) => SaveDocument(host, "et", ParamString(p, "path", null)), "Lưu thành file mới", Req("path")).ForAgent()
            };
        }

        private static Dictionary<string, object> EtListSheets(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            int count = Convert.ToInt32(wb.Worksheets.Count);
            var sheets = new List<object>();
            for (int i = 1; i <= count; i++)
            {
                sheets.Add(Convert.ToString(wb.Worksheets[i].Name));
            }
            return new Dictionary<string, object>
            {
                { "workbook", Convert.ToString(wb.Name) },
                { "activeSheet", Convert.ToString(app.ActiveSheet.Name) },
                { "sheets", sheets }
            };
        }

        // Sổ đang mở; chưa có sổ nào (Excel trống) thì tạo một sổ. Agent không còn được gọi et.newWorkbook
        // (log 30/09 01:15: model tạo thêm Book2 dù Book1 đang mở, người dùng đóng Excel thì bị hỏi lưu một sổ
        // họ không biết) - giống Word/PowerPoint, agent làm việc trên tài liệu đang mở.
        private static dynamic EnsureWorkbook(dynamic app)
        {
            dynamic wb = app.ActiveWorkbook;
            if (wb == null)
            {
                wb = app.Workbooks.Add();
                Logger.Info("et: no workbook open, created " + Convert.ToString(wb.Name));
            }
            return wb;
        }

        private static Dictionary<string, object> EtNewWorkbook(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = app.Workbooks.Add();
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(wb.Name) }
            };
        }

        private static Dictionary<string, object> EtOpen(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.Workbooks.Open(path);
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(wb.Name) },
                { "fullName", Convert.ToString(wb.FullName) }
            };
        }

        private static Dictionary<string, object> EtReadRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", "A1");
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            // Nhận cả "Project_Data!A1:I4" (bộ đo 27/10/2026 ca B: lỗi COM 0x800A03EC khó hiểu khi có tên sheet trong range).
            dynamic sheet;
            dynamic range = SheetRange(wb, sheetName, address, "'range'", out sheet);
            var result = new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) },
                { "range", address }
            };
            if (ParamBool(p, "formulas", false))
            {
                // Chỉ CÔNG THỨC: ô không có công thức -> null (giống bản LibreOffice).
                result["formulas"] = ToFormulaMatrix(range.Formula);
                result["types"] = ToTypeMatrix(range);
                var links = LinkedSheets(range, sheet);
                if (links.Count > 0) result["linkedSheets"] = links;
                return result;
            }
            result["values"] = ToMatrix(range.Value2);
            return result;
        }

        // range.Formula: ô có công thức -> chuỗi bắt đầu '='; ô không có công thức -> giá trị của nó.
        // Cần để agent phân biệt số gõ tay với kết quả tính — values không phân biệt được (bench
        // 02/10/2026: sheet Checks của Test 2 toàn chữ PASS gõ tay mà đọc values ra vẫn "đúng").
        private static object ToFormulaMatrix(object value)
        {
            Func<object, object> cell = raw =>
            {
                string s = Convert.ToString(raw);
                return s != null && s.StartsWith("=") ? (object)s : null;
            };
            Array array = value as Array;
            if (array == null)
            {
                return new List<List<object>> { new List<object> { cell(value) } };
            }
            int r0 = array.GetLowerBound(0);
            int r1 = array.GetUpperBound(0);
            int c0 = array.GetLowerBound(1);
            int c1 = array.GetUpperBound(1);
            var rows = new List<List<object>>();
            for (int r = r0; r <= r1; r++)
            {
                var row = new List<object>();
                for (int c = c0; c <= c1; c++)
                {
                    row.Add(cell(array.GetValue(r, c)));
                }
                rows.Add(row);
            }
            return rows;
        }

        // P0-04 (16/10/2026): kieu o + sheet lien ket khi formulas=true.
        private static object ToTypeMatrix(dynamic range)
        {
            var rows = new System.Collections.Generic.List<System.Collections.Generic.List<object>>();
            try
            {
                object raw = range.Value2;
                object frm = range.Formula;
                System.Array vals = raw as System.Array;
                System.Array frms = frm as System.Array;
                if (vals == null)
                {
                    string f0 = System.Convert.ToString(frm) ?? "";
                    rows.Add(new System.Collections.Generic.List<object> { f0.StartsWith("=") ? "formula" : CellType(System.Convert.ToString(raw)) });
                    return rows;
                }
                int r0 = vals.GetLowerBound(0), r1 = vals.GetUpperBound(0);
                int c0 = vals.GetLowerBound(1), c1 = vals.GetUpperBound(1);
                for (int r = r0; r <= r1; r++)
                {
                    var row = new System.Collections.Generic.List<object>();
                    for (int c = c0; c <= c1; c++)
                    {
                        string f = null;
                        try { f = System.Convert.ToString(frms.GetValue(r, c)); } catch { }
                        if (f != null && f.StartsWith("=")) row.Add("formula");
                        else row.Add(CellType(System.Convert.ToString(vals.GetValue(r, c))));
                    }
                    rows.Add(row);
                }
            }
            catch { }
            return rows;
        }

        private static string CellType(string v)
        {
            if (string.IsNullOrEmpty(v)) return "empty";
            double d;
            if (double.TryParse(v, out d)) return "number";
            if (v == "True" || v == "False") return "bool";
            if (v.StartsWith("#")) return "error";
            return "text";
        }

        private static System.Collections.Generic.List<string> LinkedSheets(dynamic range, dynamic sheet)
        {
            var links = new System.Collections.Generic.List<string>();
            try
            {
                string self = System.Convert.ToString(sheet.Name);
                object frm = range.Formula;
                System.Array arr = frm as System.Array;
                var formulas = new System.Collections.Generic.List<string>();
                if (arr == null) formulas.Add(System.Convert.ToString(frm) ?? "");
                else foreach (object o in arr) formulas.Add(System.Convert.ToString(o) ?? "");
                var rx = new System.Text.RegularExpressions.Regex("([^A-Za-z0-9_.]+)([^!]+)!");
                foreach (string f in formulas)
                {
                    if (string.IsNullOrEmpty(f) || !f.StartsWith("=")) continue;
                    foreach (System.Text.RegularExpressions.Match m in rx.Matches(f))
                    {
                        string name = m.Groups[2].Value.Trim("'".ToCharArray());
                        if (name.Length > 0 && name != self && !links.Contains(name)) links.Add(name);
                    }
                }
            }
            catch { }
            return links;
        }

        private static Dictionary<string, object> EtSheetInfo(IAppHost host, Dictionary<string, object> p)
        {
            // Kích thước dùng thật từng sheet — cùng contract với calc.py sheet_info. Excel UsedRange
            // luôn gom ít nhất A1 nên sheet chưa chạm tới -> 1x1 và ô đó trống -> empty.
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            var sheets = new List<object>();
            for (int i = 1; i <= Convert.ToInt32(wb.Worksheets.Count); i++)
            {
                dynamic ws = wb.Worksheets[i];
                dynamic used = ws.UsedRange;
                int rows = Convert.ToInt32(used.Rows.Count);
                int cols = Convert.ToInt32(used.Columns.Count);
                bool empty;
                if (rows <= 1 && cols <= 1)
                {
                    dynamic one = used.Cells[1, 1];
                    string formula = Convert.ToString(one.Formula) ?? "";
                    string value = Convert.ToString(one.Value2) ?? "";
                    empty = !formula.StartsWith("=") && string.IsNullOrEmpty(value);
                }
                else
                {
                    empty = false;   // vùng dùng > 1x1: có dữ liệu hoặc có định dạng agent đã đặt
                }
                sheets.Add(new Dictionary<string, object>
                {
                    { "name", Convert.ToString(ws.Name) },
                    { "usedRange", empty ? null : Convert.ToString(used.Address).Replace("$", "") },
                    { "rows", empty ? 0 : rows },
                    { "cols", empty ? 0 : cols },
                    { "empty", empty }
                });
            }
            return new Dictionary<string, object>
            {
                { "activeSheet", Convert.ToString(app.ActiveSheet.Name) },
                { "sheets", sheets }
            };
        }

        private static Dictionary<string, object> EtWriteRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new ArgumentException("'range' is required: the top-left cell to write at, e.g. 'A1'");
            }
            string sheetName = ParamString(p, "sheet", null);
            List<IList> rowList = ParamMatrix(p, "values", true);

            int rowCount = rowList.Count;
            int colCount = Math.Max(1, rowList.Max(row => row.Count));
            object[,] matrix = new object[rowCount, colCount];
            for (int r = 0; r < rowCount; r++)
            {
                IList row = rowList[r];
                for (int c = 0; c < colCount; c++)
                {
                    matrix[r, c] = c < row.Count ? row[c] : null;
                }
            }

            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic target = sheet.Range[address];
            try
            {
                target = target.Resize[rowCount, colCount];
            }
            catch
            {
            }
            bool same = SameCells(target, matrix, rowCount, colCount);
            target.Value2 = matrix;

            var output = new Dictionary<string, object>
            {
                { "written", rowCount * colCount },
                { "sheet", Convert.ToString(sheet.Name) }
            };
            if (same)
            {
                // Ghi lại đúng thứ đang có (bộ đo 27/10/2026 ca B: ghi "P02" vào A12 rồi đọc lại, lặp 5 lần): báo rõ để
                // model thôi ghi lại; Core coi lệnh là không sửa tài liệu và chặn lần lặp y hệt sau đó.
                output["changed"] = false;
                output["note"] = "the cells already held exactly these values: nothing changed";
            }
            return output;
        }

        // Ô đích đã mang đúng các giá trị / công thức sắp ghi chưa (so công thức với chuỗi "=...", số với số, chữ với chữ).
        private static bool SameCells(dynamic target, object[,] matrix, int rows, int cols)
        {
            try
            {
                object[,] values = AsMatrix(target.Value2, rows, cols);
                object[,] formulas = AsMatrix(target.Formula, rows, cols);
                for (int r = 0; r < rows; r++)
                {
                    for (int c = 0; c < cols; c++)
                    {
                        object incoming = matrix[r, c];
                        string text = incoming as string;
                        if (text != null && text.StartsWith("="))
                        {
                            if (!string.Equals(text, Convert.ToString(formulas[r, c]), StringComparison.OrdinalIgnoreCase))
                            {
                                return false;
                            }
                            continue;
                        }
                        object existing = values[r, c];
                        if (incoming == null || (text != null && text.Length == 0))
                        {
                            if (existing != null && Convert.ToString(existing).Length > 0)
                            {
                                return false;
                            }
                            continue;
                        }
                        if (Convert.ToString(formulas[r, c]).StartsWith("="))
                        {
                            return false;
                        }
                        if (text != null)
                        {
                            if (!(existing is string) || (string)existing != text)
                            {
                                return false;
                            }
                            continue;
                        }
                        if (incoming is bool)
                        {
                            if (!(existing is bool) || (bool)existing != (bool)incoming)
                            {
                                return false;
                            }
                            continue;
                        }
                        if (!(existing is double) || Math.Abs((double)existing - Convert.ToDouble(incoming)) > 1e-9)
                        {
                            return false;
                        }
                    }
                }
                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        // Viết MỘT công thức vào ô góc rồi điền ra cả vùng (AutoFill của Excel tự dịch tham chiếu tương
        // đối, và làm được cả khối 2D trong một lần). Chi phí token của agent tính theo TỪNG Ô nó viết
        // ra, nên bảng nghìn dòng phải đi bằng đường này chứ không gửi từng ô.
        // Ghi NHIỀU vùng trong MỘT lời gọi: model hay gọi 5–10 et.writeRange liên tiếp trong cùng một
        // phản hồi, mỗi lời gọi là một vòng qua bridge. Gộp lại bớt từng ấy vòng; không đổi cách Excel
        // tính lại (đo trên LibreOffice: tắt tính tự động rồi tính một lần thì CHẬM hơn 2,3–4,0 lần).
        private static Dictionary<string, object> EtWriteRanges(IAppHost host, Dictionary<string, object> p)
        {
            List<Dictionary<string, object>> writes = ParamObjects(p, "writes");
            if (writes == null || writes.Count == 0)
            {
                throw new InvalidOperationException("'writes' is required: a list of {range, values, sheet?}, "
                    + "e.g. [{\"range\": \"A1\", \"values\": [[\"Tên\"]]}, {\"range\": \"B2\", \"values\": [[1, 2]]}]");
            }
            // Kiểm HẾT trước khi ghi: vùng thứ ba sai thì hai vùng đầu không được vào file.
            for (int i = 0; i < writes.Count; i++)
            {
                if (string.IsNullOrEmpty(ParamString(writes[i], "range", null)))
                {
                    throw new InvalidOperationException(string.Format(
                        "'writes[{0}]' needs 'range' (the top-left cell to write at)", i));
                }
                if (ParamMatrix(writes[i], "values", false) == null)
                {
                    throw new InvalidOperationException(string.Format("'writes[{0}]' needs 'values'", i));
                }
            }

            int written = 0;
            bool changed = false;
            var sheets = new List<string>();
            var errors = new List<object>();
            string defaultSheet = ParamString(p, "sheet", null);
            foreach (Dictionary<string, object> entry in writes)
            {
                // `sheet` ngoài là mặc định cho mục không ghi sheet (trước đây bị bỏ qua: ghi vào sheet đang mở).
                Dictionary<string, object> item = entry;
                if (!string.IsNullOrEmpty(defaultSheet) && string.IsNullOrEmpty(ParamString(entry, "sheet", null)))
                {
                    item = new Dictionary<string, object>(entry);
                    item["sheet"] = defaultSheet;
                }
                Dictionary<string, object> result = EtWriteRange(host, item);
                changed |= !result.ContainsKey("changed");
                object count;
                if (result.TryGetValue("written", out count) && count != null)
                {
                    written += Convert.ToInt32(count);
                }
                object name;
                if (result.TryGetValue("sheet", out name) && name != null)
                {
                    string text = Convert.ToString(name);
                    if (!sheets.Contains(text))
                    {
                        sheets.Add(text);
                    }
                }
                object problems;
                if (result.TryGetValue("formulaErrors", out problems))
                {
                    var list = problems as IEnumerable;
                    if (list != null)
                    {
                        foreach (object problem in list)
                        {
                            errors.Add(problem);
                        }
                    }
                }
            }
            sheets.Sort(StringComparer.Ordinal);

            var output = new Dictionary<string, object>
            {
                { "ranges", writes.Count }, { "written", written }, { "sheets", sheets }
            };
            if (!changed)
            {
                output["changed"] = false;
                output["note"] = "every range already held exactly these values: nothing changed";
            }
            if (errors.Count > 0)
            {
                output["formulaErrors"] = errors;
            }
            return output;
        }

        // Nạp CSV vào sổ: mở file bằng chính Excel (nó phân tích CSV tốt hơn ta tự viết), copy vùng đã
        // dùng sang đích, rồi đóng file nguồn KHÔNG lưu. Cách này giữ nguyên kiểu số/ngày như Excel đọc.
        private static Dictionary<string, object> EtImportCsv(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required: the .csv file to read");
            }
            if (!System.IO.File.Exists(path))
            {
                throw new InvalidOperationException(string.Format("no file at '{0}'", path));
            }
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            string sheetName = ParamString(p, "sheet", null);
            string address = ParamString(p, "range", null);

            dynamic source = null;
            try
            {
                source = app.Workbooks.Open(path, Type.Missing, true);   // ReadOnly
                dynamic sourceSheet = source.Worksheets[1];
                dynamic used = sourceSheet.UsedRange;
                int rows = Convert.ToInt32(used.Rows.Count);
                int columns = Convert.ToInt32(used.Columns.Count);

                dynamic sheet;
                if (!string.IsNullOrEmpty(address))
                {
                    sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
                }
                else
                {
                    wb.Worksheets.Add(Type.Missing, wb.Worksheets[wb.Worksheets.Count]);
                    sheet = wb.Worksheets[wb.Worksheets.Count];
                    sheet.Name = SheetNameFromPath(path, wb);
                }
                dynamic destination = string.IsNullOrEmpty(address) ? sheet.Cells[1, 1] : sheet.Range[address];
                used.Copy(destination);
                return new Dictionary<string, object>
                {
                    { "path", path }, { "sheet", Convert.ToString(sheet.Name) },
                    { "rows", rows }, { "columns", columns }, { "cells", rows * columns }
                };
            }
            finally
            {
                if (source != null)
                {
                    try
                    {
                        source.Close(false);
                    }
                    catch (Exception)
                    {
                    }
                }
            }
        }

        // Tên sheet tối đa 31 ký tự, không được trùng và không được chứa []*?/\:
        private static string SheetNameFromPath(string path, dynamic workbook)
        {
            string name = System.IO.Path.GetFileNameWithoutExtension(path);
            foreach (char bad in new[] { '[', ']', '*', '?', '/', '\\', ':' })
            {
                name = name.Replace(bad, '_');
            }
            if (name.Length > 31)
            {
                name = name.Substring(0, 31);
            }
            if (name.Length == 0)
            {
                name = "Import";
            }
            var taken = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
            foreach (dynamic existing in workbook.Worksheets)
            {
                taken.Add(Convert.ToString(existing.Name));
            }
            string candidate = name;
            int suffix = 2;
            while (taken.Contains(candidate))
            {
                string tail = " " + suffix;
                candidate = (name.Length + tail.Length > 31 ? name.Substring(0, 31 - tail.Length) : name) + tail;
                suffix++;
            }
            return candidate;
        }

        private static Dictionary<string, object> EtFillRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required: the whole area to fill, e.g. 'B2:H1000'");
            }
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic target = sheet.Range[address];
            dynamic corner = target.Cells[1, 1];

            string formula = ParamString(p, "formula", null);
            if (!string.IsNullOrEmpty(formula))
            {
                corner.Formula = formula;
            }

            int rows = Convert.ToInt32(target.Rows.Count);
            int cols = Convert.ToInt32(target.Columns.Count);
            if (rows > 1 || cols > 1)
            {
                corner.AutoFill(target, 0);   // xlFillDefault
            }

            var sample = new List<object>();
            for (int r = 1; r <= Math.Min(rows, 3); r++)
            {
                var line = new List<object>();
                for (int c = 1; c <= Math.Min(cols, 3); c++)
                {
                    line.Add(Convert.ToString(target.Cells[r, c].Formula));
                }
                sample.Add(line);
            }
            return new Dictionary<string, object>
            {
                { "filled", rows * cols }, { "range", address }, { "sheet", Convert.ToString(sheet.Name) },
                { "sample", sample }
            };
        }

        private static Dictionary<string, object> EtFormatRange(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required");
            }
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            ApplyRangeFormat(sheet.Range[address], p);
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) },
                { "range", address }
            };
        }

        private static readonly string[] RangeFormatKeys =
            { "bold", "italic", "fontSize", "fontColor", "fillColor", "numFmt", "horizontal", "wrap", "columnWidth" };

        // Định dạng NHIỀU vùng trong MỘT lời gọi (bộ đo 27/10/2026 ca B: 19 lần et.formatRange, mỗi lần một vùng,
        // hết 60 vòng). Kiểm HẾT trước khi định dạng (sheet có, địa chỉ đúng, màu #RRGGBB, có ít nhất một thuộc tính):
        // một mục sai thì không vùng nào bị đổi.
        private static Dictionary<string, object> EtFormatRanges(IAppHost host, Dictionary<string, object> p)
        {
            List<Dictionary<string, object>> formats = ParamObjects(p, "formats");
            if (formats == null || formats.Count == 0)
            {
                throw new InvalidOperationException("'formats' is required: a list of {range, sheet?, bold?, fillColor?, numFmt?, ...}, "
                    + "e.g. [{\"range\": \"A1:I1\", \"bold\": true, \"fillColor\": \"#DDEBF7\"}, {\"range\": \"D2:E4\", \"numFmt\": \"#,##0\"}]");
            }
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            string defaultSheet = ParamString(p, "sheet", null);
            var targets = new List<dynamic>();
            var sheets = new List<string>();
            for (int i = 0; i < formats.Count; i++)
            {
                Dictionary<string, object> item = formats[i];
                string address = ParamString(item, "range", null);
                if (string.IsNullOrEmpty(address))
                {
                    throw new InvalidOperationException(string.Format("'formats[{0}]' needs 'range', e.g. 'A1:C1'", i));
                }
                if (!RangeFormatKeys.Any(key => HasParam(item, key)))
                {
                    throw new InvalidOperationException(string.Format("'formats[{0}]' ({1}) has nothing to apply: give at least one of {2}",
                        i, address, string.Join("/", RangeFormatKeys)));
                }
                foreach (string key in new[] { "fontColor", "fillColor" })
                {
                    string color = ParamString(item, key, null);
                    if (!string.IsNullOrEmpty(color) && !ParseBgrColor(color).HasValue)
                    {
                        throw new InvalidOperationException(string.Format("'formats[{0}].{1}': '{2}' is not a #RRGGBB colour", i, key, color));
                    }
                }
                string horizontal = ParamString(item, "horizontal", "left").ToLowerInvariant();
                if (horizontal != "left" && horizontal != "center" && horizontal != "right")
                {
                    throw new InvalidOperationException(string.Format("'formats[{0}].horizontal' must be left/center/right, got '{1}'", i, horizontal));
                }
                string sheetName = ParamString(item, "sheet", defaultSheet);
                dynamic sheet;
                try
                {
                    sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
                }
                catch (Exception)
                {
                    throw new InvalidOperationException(string.Format("'formats[{0}].sheet': no sheet named '{1}' (sheets: {2})",
                        i, sheetName, string.Join(", ", SheetNames(wb))));
                }
                try
                {
                    targets.Add(sheet.Range[address]);
                }
                catch (Exception)
                {
                    throw new InvalidOperationException(string.Format("'formats[{0}].range': '{1}' is not a range address on sheet '{2}'",
                        i, address, Convert.ToString(sheet.Name)));
                }
                string name = Convert.ToString(sheet.Name);
                if (!sheets.Contains(name))
                {
                    sheets.Add(name);
                }
                // numFmt Excel không nhận ("#,##0 \"đ" thiếu nháy, "[Red") chỉ lộ khi gán: thử trên ô đầu của vùng rồi trả lại
                // định dạng cũ, để một mục sai thì vẫn không vùng nào bị đổi.
                string numFmt = ParamString(item, "numFmt", null);
                if (!string.IsNullOrEmpty(numFmt))
                {
                    dynamic probe = targets[i].Cells[1, 1];
                    object previous = probe.NumberFormat;
                    try
                    {
                        probe.NumberFormat = numFmt;
                    }
                    catch (Exception)
                    {
                        throw new InvalidOperationException(string.Format("'formats[{0}].numFmt': Excel does not accept '{1}' "
                            + "(use codes like '#,##0', '0.0%', 'dd/mm/yyyy', '#,##0 \"đ\"'); nothing was changed", i, numFmt));
                    }
                    probe.NumberFormat = previous;
                }
            }
            for (int i = 0; i < formats.Count; i++)
            {
                try
                {
                    ApplyRangeFormat(targets[i], formats[i]);
                }
                catch (Exception ex)
                {
                    // Excel từ chối thường là numFmt lạ (bộ đo 27/10/2026 ca B, p3k): nói rõ mục nào, mục nào đã áp.
                    string numFmt = ParamString(formats[i], "numFmt", null);
                    throw new InvalidOperationException(string.Format("'formats[{0}]' ({1}) was refused by Excel: {2}{3}. Items 0-{4} "
                        + "were applied; send only the remaining items again", i, ParamString(formats[i], "range", ""), ex.Message,
                        string.IsNullOrEmpty(numFmt) ? "" : " - check numFmt '" + numFmt + "' (use codes like '#,##0', '0.0%', "
                        + "'dd/mm/yyyy', '#,##0 \"đ\"')", i - 1));
                }
            }
            sheets.Sort(StringComparer.Ordinal);
            return new Dictionary<string, object> { { "ranges", formats.Count }, { "sheets", sheets } };
        }

        private static void ApplyRangeFormat(dynamic range, Dictionary<string, object> p)
        {
            dynamic font = range.Font;
            if (HasParam(p, "bold"))
            {
                font.Bold = ParamBool(p, "bold", false);
            }
            if (HasParam(p, "italic"))
            {
                font.Italic = ParamBool(p, "italic", false);
            }
            if (HasParam(p, "fontSize"))
            {
                font.Size = ParamInt(p, "fontSize", 11);
            }
            int? fontColor = ParseBgrColor(ParamString(p, "fontColor", null));
            if (fontColor.HasValue)
            {
                font.Color = fontColor.Value;
            }
            int? fillColor = ParseBgrColor(ParamString(p, "fillColor", null));
            if (fillColor.HasValue)
            {
                range.Interior.Color = fillColor.Value;
            }
            if (HasParam(p, "numFmt"))
            {
                range.NumberFormat = ParamString(p, "numFmt", "General");
            }
            if (HasParam(p, "horizontal"))
            {
                string horizontal = ParamString(p, "horizontal", "left").ToLowerInvariant();
                range.HorizontalAlignment = horizontal == "center" ? -4108 : (horizontal == "right" ? -4152 : -4131);
            }
            if (HasParam(p, "wrap"))
            {
                range.WrapText = ParamBool(p, "wrap", false);
            }
            // Ô số lớn cỡ chữ to hiện "####" khi cột hẹp (ô KPI của dashboard).
            if (HasParam(p, "columnWidth"))
            {
                range.EntireColumn.ColumnWidth = Math.Max(1.0, Math.Min(255.0, ParamDouble(p, "columnWidth", 12.0)));
            }
        }

        private static Dictionary<string, object> EtActivateSheet(IAppHost host, Dictionary<string, object> p)
        {
            string sheetName = ParamString(p, "sheet", null);
            if (string.IsNullOrEmpty(sheetName))
            {
                throw new InvalidOperationException("'sheet' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            wb.Worksheets[sheetName].Activate();
            return new Dictionary<string, object> { { "active", sheetName } };
        }

        // Thêm sheet mới. Không có lệnh này thì agent không dựng được sổ nhiều sheet trên tài liệu đang mở
        // (log 02/10: model thử ~30 tên lệnh tự nghĩ ra rồi bỏ cuộc). Tên bỏ trống -> Sheet1, Sheet2...
        // tránh trùng; `index` là vị trí chèn (1-based như Excel/WPS, mặc định là cuối).
        private static Dictionary<string, object> EtAddSheet(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            var existing = new List<string>();
            int count = Convert.ToInt32(wb.Worksheets.Count);
            for (int i = 1; i <= count; i++)
            {
                existing.Add(Convert.ToString(wb.Worksheets[i].Name));
            }

            string name = ParamString(p, "name", null);
            if (string.IsNullOrEmpty(name))
            {
                // So nho nhat con trong (Sheet1, Sheet2...), giong ban LibreOffice - hai lan chay cung mot thu.
                int suffix = 1;
                while (existing.Contains("Sheet" + suffix))
                {
                    suffix++;
                }
                name = "Sheet" + suffix;
            }
            else if (existing.Contains(name))
            {
                throw new InvalidOperationException("a sheet named '" + name + "' already exists (sheets: " +
                    string.Join(", ", existing) + ")");
            }

            // `index` 0-based giong ban LibreOffice (Excel/WPS dem sheet tu 1, nen phai doi o day).
            int insertAt = Math.Max(0, Math.Min(ParamInt(p, "index", count), count));
            dynamic sheet = insertAt >= count
                ? wb.Worksheets.Add(Type.Missing, wb.Worksheets[count])
                : wb.Worksheets.Add(Type.Missing, wb.Worksheets[insertAt + 1]);
            sheet.Name = name;
            sheet.Activate();
            return new Dictionary<string, object> { { "sheet", name }, { "sheets", SheetNames(wb) } };
        }

        private static Dictionary<string, object> EtRenameSheet(IAppHost host, Dictionary<string, object> p)
        {
            string oldName = ParamString(p, "sheet", null);
            string newName = ParamString(p, "name", null);
            if (string.IsNullOrEmpty(oldName))
            {
                throw new InvalidOperationException("'sheet' is required");
            }
            if (string.IsNullOrEmpty(newName))
            {
                throw new InvalidOperationException("'name' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            var names = SheetNames(wb);
            if (!names.Contains(oldName))
            {
                throw new InvalidOperationException("no sheet named '" + oldName + "' (sheets: " +
                    string.Join(", ", names) + ")");
            }
            if (newName != oldName && names.Contains(newName))
            {
                throw new InvalidOperationException("a sheet named '" + newName + "' already exists (sheets: " +
                    string.Join(", ", names) + ")");
            }
            wb.Worksheets[oldName].Name = newName;   // Excel tự cập nhật công thức trỏ tên cũ
            return new Dictionary<string, object> { { "sheet", newName }, { "sheets", SheetNames(wb) } };
        }

        // Bộ đo 27/10/2026 đề B ("đúng 4 sheet"): không có lệnh xoá sheet, model đổi tên Sheet1 qua lại hàng chục lần đến khi
        // bị chặn. Core hỏi người dùng trước khi xoá sheet có dữ liệu mà không do lượt tạo (policy.go).
        private static Dictionary<string, object> EtDeleteSheet(IAppHost host, Dictionary<string, object> p)
        {
            string name = ParamString(p, "sheet", null);
            if (string.IsNullOrEmpty(name))
            {
                throw new InvalidOperationException("'sheet' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            var names = SheetNames(wb);
            if (!names.Contains(name))
            {
                throw new InvalidOperationException("no sheet named '" + name + "' (sheets: " + string.Join(", ", names) + ")");
            }
            if (names.Count <= 1)
            {
                throw new InvalidOperationException("'" + name + "' is the only sheet - a workbook needs at least one");
            }
            // Excel hỏi "xoá vĩnh viễn?" bằng hộp thoại: tắt cảnh báo trong lúc xoá.
            bool alerts = Convert.ToBoolean(app.DisplayAlerts);
            try
            {
                app.DisplayAlerts = false;
                wb.Worksheets[name].Delete();
            }
            finally
            {
                app.DisplayAlerts = alerts;
            }
            return new Dictionary<string, object> { { "deleted", name }, { "sheets", SheetNames(wb) } };
        }

        // Kiểu biểu đồ theo tên người dùng gõ -> hằng số XlChartType của Excel.
        private static int ChartType(string kind)
        {
            switch (kind)
            {
                case "column": return 51;     // xlColumnClustered
                case "bar": return 57;        // xlBarClustered
                case "line": return 4;        // xlLine
                case "pie": return 5;         // xlPie
                case "area": return 1;        // xlArea
                case "scatter": return -4169; // xlXYScatter
                default:
                    throw new InvalidOperationException(
                        "'type' must be one of area/bar/column/line/pie/scatter, got '" + kind + "'");
            }
        }

        // Chèn biểu đồ từ một vùng dữ liệu. Không có lệnh này thì Dashboard chỉ là bảng số.
        private static Dictionary<string, object> EtAddChart(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required");
            }
            string kind = (ParamString(p, "type", "column") ?? "column").ToLowerInvariant();
            int xlType = ChartType(kind);
            string sheetName = ParamString(p, "sheet", null);
            string anchor = ParamString(p, "anchor", "A1") ?? "A1";
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];

            // cm -> point (Excel dùng point cho vị trí/kích thước shape).
            double width = Math.Max(30, ParamDouble(p, "width", 12.0) * 28.3465);
            double height = Math.Max(30, ParamDouble(p, "height", 7.0) * 28.3465);
            dynamic anchorCell = sheet.Range[anchor];

            // Tên phải tránh MỌI hình trên sheet, không chỉ hình là chart: một hình tên "Chart3" không phải
            // chart vẫn chiếm chỗ, mà "ChartObjects().Count + 1" thì có thể sinh đúng cái tên đó.
            // (Bản LibreOffice phải tránh tên chart của CẢ TÀI LIỆU - tên chart bên đó duy nhất theo tài
            // liệu chứ không theo sheet, và đó là một lỗi thật đã đo được; xem calc.py _taken_chart_names.)
            var taken = new HashSet<string>(StringComparer.Ordinal);
            try
            {
                foreach (dynamic existingShape in sheet.Shapes)
                {
                    taken.Add(Convert.ToString(existingShape.Name));
                }
            }
            catch (Exception)
            {
                // host thiếu Shapes: quay về chỉ kiểm chart
                foreach (dynamic existing in sheet.ChartObjects())
                {
                    taken.Add(Convert.ToString(existing.Name));
                }
            }

            string name = ParamString(p, "name", null);
            if (string.IsNullOrEmpty(name))
            {
                int index = 1;
                while (taken.Contains("Chart" + index))
                {
                    index++;
                }
                name = "Chart" + index;
            }
            else if (taken.Contains(name))
            {
                throw new InvalidOperationException("a chart named '" + name +
                    "' already exists on sheet '" + Convert.ToString(sheet.Name) + "'");
            }

            // Vùng nguồn ở sheet khác ("Data!A1:C7", "'Kế hoạch'!A1:C7"): biểu đồ đặt trên `sheet`, dữ liệu đọc thẳng từ sheet
            // nguồn để tự cập nhật (bộ đo 26/10/2026 ca x09: trước phải chép dữ liệu sang Dashboard làm bảng trung gian).
            dynamic source = sheet;
            // Vùng rời mỗi phần có tên sheet ("Data!A1:A13,Data!D1:D13" - bộ đo 27/10/2026 ca x11): bỏ tên sheet ở các phần
            // sau; mọi phần phải cùng một sheet.
            string[] areas = address.Split(',');
            string prefix = null;
            for (int i = 0; i < areas.Length; i++)
            {
                int mark = areas[i].LastIndexOf('!');
                if (mark <= 0)
                {
                    continue;
                }
                string areaSheet = areas[i].Substring(0, mark).Trim();
                if (prefix == null)
                {
                    prefix = areaSheet;
                }
                else if (!string.Equals(prefix.Trim('\''), areaSheet.Trim('\''), StringComparison.OrdinalIgnoreCase))
                {
                    throw new InvalidOperationException("all parts of 'range' must be on one sheet, got '" + address + "'");
                }
                areas[i] = (i == 0 ? areaSheet + "!" : "") + areas[i].Substring(mark + 1).Trim();
            }
            address = string.Join(",", areas);
            string local = address;
            int bang = address.IndexOf('!');
            if (bang > 0)
            {
                string sourceName = address.Substring(0, bang).Trim().Trim('\'').Replace("''", "'");
                local = address.Substring(bang + 1);
                try
                {
                    source = wb.Worksheets[sourceName];
                }
                catch (Exception)
                {
                    throw new InvalidOperationException("no sheet named '" + sourceName + "' for the chart data");
                }
            }
            dynamic data = source.Range[local];
            dynamic shape = sheet.Shapes.AddChart2(-1, xlType, anchorCell.Left, anchorCell.Top, width, height);
            string title = ParamString(p, "title", null);
            bool titleApplied = false;
            // Hỏng giữa chừng thì xoá hình biểu đồ vừa thêm: trước đây mỗi lần lỗi để lại một biểu đồ rỗng (bộ đo 27/10/2026
            // ca B, p3k: 6 lần "This object has no title" trên vùng chưa có số).
            try
            {
                shape.Name = name;
                shape.Chart.SetSourceData(data);
                if (Convert.ToInt32(shape.Chart.SeriesCollection().Count) == 0)
                {
                    throw new InvalidOperationException("the range '" + address + "' has no numbers to chart: write the data "
                        + "(or formulas) there first, or point 'range' at the table that holds them");
                }
                if (!string.IsNullOrEmpty(title))
                {
                    shape.Chart.HasTitle = true;
                    shape.Chart.ChartTitle.Text = title;
                    titleApplied = true;
                }
            }
            catch (Exception ex)
            {
                try
                {
                    shape.Delete();
                }
                catch (Exception)
                {
                }
                if (ex is InvalidOperationException)
                {
                    throw;
                }
                throw new InvalidOperationException("could not build the chart from '" + address + "' (" + ex.Message
                    + "): check that the range holds a header row and numbers; nothing was added");
            }
            // Đọc lại kiểu THẬT SỰ của biểu đồ thay vì lặp lại điều vừa xin (giống bản LibreOffice).
            string diagram = ChartKindName(shape.Chart);
            return new Dictionary<string, object>
            {
                { "chart", name }, { "sheet", Convert.ToString(sheet.Name) },
                { "type", kind }, { "diagram", diagram },
                { "typeApplied", diagram == kind }, { "titleApplied", titleApplied },
                { "range", address }, { "anchor", anchor }
            };
        }

        // Kiểu thật -> cùng bộ tên với bản LibreOffice (column/bar/line/pie/area/scatter).
        private static string ChartKindName(dynamic chart)
        {
            try
            {
                switch (Convert.ToInt32(chart.ChartType))
                {
                    case 51: return "column";
                    case 57: return "bar";
                    case 4: return "line";
                    case 5: return "pie";
                    case 1: return "area";
                    case -4169: return "scatter";
                    default: return "type" + Convert.ToInt32(chart.ChartType);
                }
            }
            catch (Exception)
            {
                return "";
            }
        }

        private static Dictionary<string, object> EtListCharts(IAppHost host, Dictionary<string, object> p)
        {
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            var charts = new List<object>();
            foreach (dynamic item in sheet.ChartObjects())
            {
                charts.Add(new Dictionary<string, object>
                {
                    { "name", Convert.ToString(item.Name) },
                    { "diagram", ChartKindName(item.Chart) }
                });
            }
            return new Dictionary<string, object> { { "sheet", Convert.ToString(sheet.Name) }, { "charts", charts } };
        }

        private static List<string> SheetNames(dynamic wb)
        {
            var names = new List<string>();
            int count = Convert.ToInt32(wb.Worksheets.Count);
            for (int i = 1; i <= count; i++)
            {
                names.Add(Convert.ToString(wb.Worksheets[i].Name));
            }
            return names;
        }

        private static Dictionary<string, object> EtExportPdf(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic wb = app.ActiveWorkbook;
            wb.ExportAsFixedFormat(0, path);
            return new Dictionary<string, object> { { "exported", path } };
        }

        // --- Định dạng điều kiện (Excel/WPS dùng Range.FormatConditions) ---------------------------
        //
        // Bản LibreOffice phải đi đường vòng qua UNO (xem calc.py: createByRange rồi createEntry);
        // bên này API phẳng hơn nên gọi thẳng. Tên lệnh/tham số hai bên giống nhau để agent thấy cùng
        // một hợp đồng, nhưng cách đặt style thì khác: LibreOffice gắn bằng TÊN CELL STYLE, Excel gắn
        // màu trực tiếp nên `styleName` chỉ có tác dụng bên LibreOffice.

        // XlFormatConditionOperator: xlBetween=1, xlNotBetween=2, xlEqual=3, xlNotEqual=4,
        // xlGreater=5, xlLess=6, xlGreaterEqual=7, xlLessEqual=8.
        private static readonly Dictionary<string, int> ConditionalOperators =
            new Dictionary<string, int>(StringComparer.Ordinal)
            {
                { "between", 1 }, { "notbetween", 2 }, { "equal", 3 }, { "notequal", 4 },
                { "greater", 5 }, { "less", 6 }, { "greaterequal", 7 }, { "lessequal", 8 }
            };

        private static readonly Dictionary<int, string> ConditionalOperatorNames =
            new Dictionary<int, string>
            {
                { 1, "between" }, { 2, "notBetween" }, { 3, "equal" }, { 4, "notEqual" },
                { 5, "greater" }, { 6, "less" }, { 7, "greaterEqual" }, { 8, "lessEqual" }
            };

        private const int XlCellValue = 1;      // XlFormatConditionType
        private const int XlExpression = 2;

        private static Dictionary<string, object> EtSetConditionalFormat(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required: the area the rules apply to, e.g. 'B2:B100'");
            }
            List<Dictionary<string, object>> rules = ParamObjects(p, "rules");
            if (rules == null || rules.Count == 0)
            {
                throw new InvalidOperationException("'rules' is required: a list of rules, e.g. "
                    + "[{\"operator\": \"less\", \"formula1\": \"50\", \"fillColor\": \"#FFCCCC\"}]");
            }
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            string sheetName = ParamString(p, "sheet", null);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            dynamic range = sheet.Range[address];

            // Kiểm hết rule trước khi ghi: rule thứ ba sai thì hai rule đầu cũng không được vào file.
            var plan = new List<Dictionary<string, object>>();
            for (int i = 0; i < rules.Count; i++)
            {
                string visual = NormalizeOperator(ParamString(rules[i], "type", ""));
                // type "cellIs" / "formula" (tên kiểu của OpenXML/openpyxl - bộ đo 27/10/2026 ca B, p3m: 3 lần gọi hỏng) là rule
                // so sánh / công thức thường, không phải kiểu trực quan.
                if (visual.Length > 0 && visual != "cellvalue" && visual != "expression" && visual != "cellis" && visual != "formula")
                {
                    CheckVisualRule(rules[i], visual, i);
                    plan.Add(new Dictionary<string, object>
                    {
                        { "key", visual }, { "input", ParamString(rules[i], "type", visual) }, { "formula1", null },
                        { "formula2", null }, { "rule", rules[i] }, { "visual", true }
                    });
                    continue;
                }
                string key = OperatorKey(ParamString(rules[i], "operator", visual == "formula" || visual == "expression" ? "formula" : "less"));
                bool isFormula = key == "formula";
                if (!isFormula && !ConditionalOperators.ContainsKey(key))
                {
                    throw new InvalidOperationException(string.Format(
                        "'rules[{0}].operator' must be one of less/lessEqual/greater/greaterEqual/equal/notEqual/"
                        + "between/notBetween/formula, got '{1}'", i, key));
                }
                // `value1`/`value` thay cho formula1 (bộ đo 27/10/2026 ca B: 4 lần gọi hỏng vì tên tham số).
                string formula1 = ParamString(rules[i], "formula1", ParamString(rules[i], "value1", ParamString(rules[i], "value", null)));
                if (string.IsNullOrEmpty(formula1))
                {
                    throw new InvalidOperationException(string.Format(
                        "'rules[{0}]' needs 'formula1' - the value to compare with, e.g. "
                        + "{{\"operator\": \"less\", \"formula1\": \"50\"}}", i));
                }
                string formula2 = ParamString(rules[i], "formula2", ParamString(rules[i], "value2", null));
                if ((key == "between" || key == "notbetween") && string.IsNullOrEmpty(formula2))
                {
                    throw new InvalidOperationException(string.Format(
                        "'rules[{0}]': operator '{1}' needs 'formula2' as well", i, key));
                }
                plan.Add(new Dictionary<string, object>
                {
                    { "key", key }, { "input", isFormula ? ParamString(rules[i], "operator", "formula") : ConditionalOperatorNames[ConditionalOperators[key]] }, { "formula1", formula1 },
                    { "formula2", formula2 }, { "rule", rules[i] }
                });
            }

            DeleteSameRangeConditions(range);       // thay rule cũ của ĐÚNG vùng này: gọi lại không chồng lên nhau

            var written = new List<object>();
            foreach (Dictionary<string, object> item in plan)
            {
                string key = Convert.ToString(item["key"]);
                string formula1 = Convert.ToString(item["formula1"]);
                string formula2 = Convert.ToString(item["formula2"]);
                Dictionary<string, object> rule = (Dictionary<string, object>)item["rule"];
                dynamic condition;
                if (item.ContainsKey("visual"))
                {
                    condition = AddVisualCondition(range, wb, key, rule);
                    if (VisualRuleTakesStyle(key))
                    {
                        ApplyConditionStyle(condition, rule);
                    }
                    written.Add(new Dictionary<string, object> { { "type", item["input"] } });
                    continue;
                }
                if (key == "formula")
                {
                    // Công thức trần: Excel nhận kèm dấu =, và không dùng toán tử.
                    condition = range.FormatConditions.Add(XlExpression, Type.Missing, "=" + formula1.TrimStart('='));
                }
                else if (!string.IsNullOrEmpty(formula2))
                {
                    condition = range.FormatConditions.Add(XlCellValue, ConditionalOperators[key], formula1, formula2);
                }
                else
                {
                    condition = range.FormatConditions.Add(XlCellValue, ConditionalOperators[key], formula1);
                }
                ApplyConditionStyle(condition, rule);
                // Trả tên toán tử chuẩn ("greaterEqual"), như bản LibreOffice/WPS - không phải khoá so khớp viết thường;
                // model gửi tên khác (greaterThanOrEqual, ">=") thì thấy tên đúng cho lần sau.
                written.Add(new Dictionary<string, object>
                {
                    { "operator", item["input"] }, { "formula1", formula1 },
                    { "formula2", string.IsNullOrEmpty(formula2) ? null : formula2 }
                });
            }

            // Handler trả kết quả trần: dispatcher tự bọc {ok, result} (trước đây Ok(...) ở đây làm kết quả bị bọc hai lớp).
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) }, { "range", Convert.ToString(range.Address(false, false)) }, { "rules", written }
            };
        }

        private static Dictionary<string, object> EtListConditionalFormats(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            string sheetName = ParamString(p, "sheet", null);
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            // Excel không có "mọi định dạng điều kiện của sheet" như LibreOffice: FormatConditions chỉ
            // đọc được từ một Range. Bỏ trống `range` thì soi vùng đang dùng thật.
            dynamic scope = string.IsNullOrEmpty(address) ? sheet.UsedRange : sheet.Range[address];

            // Cùng hình dạng với LibreOffice/WPS: {sheet, formats: [{range, rules: [...]}]}, nhóm theo vùng áp dụng
            // (AppliesTo). Trước đây Excel trả danh sách rule phẳng, không biết rule thuộc vùng nào.
            var formats = new List<object>();
            var byRange = new Dictionary<string, List<object>>();
            dynamic conditions = scope.FormatConditions;
            int count = Convert.ToInt32(conditions.Count);
            for (int i = 1; i <= count; i++)
            {
                dynamic condition = conditions[i];
                int type = 0;
                try
                {
                    type = Convert.ToInt32(condition.Type);
                }
                catch (Exception)
                {
                }
                string operatorName = null;
                if (VisualTypeNames.ContainsKey(type))
                {
                    operatorName = VisualTypeNames[type];
                }
                else if (type == XlExpression)
                {
                    // Điều kiện công thức không có Operator (đọc thuộc tính đó ném lỗi): trước đây trả null.
                    operatorName = "formula";
                }
                else
                {
                    try
                    {
                        int code = Convert.ToInt32(condition.Operator);
                        operatorName = ConditionalOperatorNames.ContainsKey(code) ? ConditionalOperatorNames[code] : "unknown(" + code + ")";
                    }
                    catch (Exception)
                    {
                    }
                }
                string applies = Convert.ToString(scope.Address(false, false));
                try
                {
                    applies = Convert.ToString(condition.AppliesTo.Address(false, false));
                }
                catch (Exception)
                {
                }
                List<object> rules;
                if (!byRange.TryGetValue(applies, out rules))
                {
                    rules = new List<object>();
                    byRange[applies] = rules;
                    formats.Add(new Dictionary<string, object> { { "range", applies }, { "rules", rules } });
                }
                // Excel lưu formula kèm "=" ("=50"); bỏ đi cho khớp giá trị đã ghi và bản LibreOffice. formula2 chỉ
                // có nghĩa với between/notBetween (giống LibreOffice/WPS).
                bool usesFormula2 = operatorName == "between" || operatorName == "notBetween";
                rules.Add(new Dictionary<string, object>
                {
                    { "operator", operatorName },
                    { "formula1", StripEquals(SafeFormula(condition, "Formula1")) },
                    { "formula2", usesFormula2 ? StripEquals(SafeFormula(condition, "Formula2")) : null },
                    { "type", type }
                });
            }
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) },
                { "formats", formats }
            };
        }

        private static string StripEquals(string formula)
        {
            return formula != null && formula.StartsWith("=") ? formula.Substring(1) : formula;
        }

        // Công thức của FormatCondition có thể ném COMException (vùng gộp, điều kiện kiểu thanh/màu) -
        // đọc hỏng một trường không được làm hỏng cả lệnh đọc.
        private static string SafeFormula(dynamic condition, string name)
        {
            try
            {
                return Convert.ToString(name == "Formula1" ? condition.Formula1 : condition.Formula2);
            }
            catch (Exception)
            {
                return null;
            }
        }

        // "lessEqual" / "less_equal" / "less equal" / "LESS" -> "lessequal"
        // Tên toán tử kiểu Excel/VBA model hay gửi: greaterThan, greaterThanOrEqual, lessThanOrEqualTo, equalTo, notEqualTo,
        // và ký hiệu > >= < <= = <> -> khoá của ConditionalOperators.
        private static string OperatorKey(string value)
        {
            string key = NormalizeOperator(value);
            switch (key)
            {
                case ">": return "greater";
                case ">=": return "greaterequal";
                case "<": return "less";
                case "<=": return "lessequal";
                case "=": case "==": return "equal";
                case "<>": case "!=": return "notequal";
            }
            if (key.StartsWith("xl"))
            {
                key = key.Substring(2);
            }
            key = key.Replace("orequalto", "equal").Replace("orequal", "equal").Replace("than", "");
            if (key.EndsWith("to"))
            {
                key = key.Substring(0, key.Length - 2);
            }
            return key;
        }

        private static string NormalizeOperator(string value)
        {
            return (value ?? "less").Replace("_", "").Replace(" ", "").ToLowerInvariant();
        }

        private static Dictionary<string, object> EtUndo(IAppHost host, Dictionary<string, object> p)
        {
            int count = Math.Max(1, ParamInt(p, "count", 1));
            dynamic app = host.Application;
            int undone = 0;
            for (int i = 0; i < count; i++)
            {
                try
                {
                    app.Undo();
                    undone++;
                }
                catch (COMException ex)
                {
                    if (IsRetryableComError(ex))
                    {
                        throw;
                    }
                    Logger.Info("et.undo stopped: " + ex.Message);
                    break;
                }
                catch (Exception ex)
                {
                    Logger.Info("et.undo stopped: " + ex.Message);
                    break;
                }
            }
            return new Dictionary<string, object> { { "undone", undone } };
        }
    }
}

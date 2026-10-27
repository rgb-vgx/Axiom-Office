using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;

namespace AxiomOffice.Bridge
{
    // Cấu trúc bảng tính dùng được thật (docs/plans/21-v0 P1): Excel Table (có bộ lọc), cố định hàng/cột tiêu đề, kiểm
    // tra dữ liệu nhập (danh sách chọn). Bộ đo Excel 26/10/2026: thiếu ba lệnh này nên model dặn người dùng tự bấm
    // (x05), dùng định dạng có điều kiện giả làm danh sách chọn (x06), workbook 4 sheet (đề B) không đạt.
    internal static partial class CommandDispatcher
    {
        private static IEnumerable<CommandInfo> SpreadsheetStructureCommands()
        {
            return new[]
            {
                Command("et.createTable", "et", EtCreateTable,
                    "Biến vùng dữ liệu (hàng đầu là tiêu đề) thành Excel Table: có nút lọc, tự mở rộng, công thức theo cột",
                    Req("range", "data incl. header row, e.g. 'A1:I4'"), Opt("sheet"),
                    Opt("name", "table name, e.g. 'tblProjects'"), Opt("style", "e.g. 'TableStyleMedium2' (default)")).ForAgent(),
                Command("et.freezePanes", "et", EtFreezePanes,
                    "Cố định hàng/cột tiêu đề khi cuộn (rows = số hàng trên cùng, cols = số cột bên trái; 0/0 = bỏ cố định)",
                    Opt("rows", "top rows to keep visible, default 1"), Opt("cols", "left columns to keep visible, default 0"),
                    Opt("sheet")).ForAgent(),
                Command("et.setValidation", "et", EtSetValidation,
                    "Kiểm tra dữ liệu nhập cho vùng: danh sách chọn (`list` hoặc `source` là vùng chứa danh sách) hoặc số/ngày trong khoảng (`type` + min/max); thay quy tắc cũ của vùng",
                    Req("range"), Opt("list", "array of allowed values"), Opt("source", "range holding the allowed values, e.g. 'Config!$B$2:$B$5'"),
                    Opt("type", "list (default when list/source) / whole / decimal / date"), Opt("min"), Opt("max"),
                    Opt("message", "error message shown on invalid input"), Opt("sheet")).ForAgent()
            };
        }

        private static dynamic SheetFor(dynamic wb, Dictionary<string, object> p)
        {
            string sheetName = ParamString(p, "sheet", null);
            if (string.IsNullOrEmpty(sheetName))
            {
                return wb.ActiveSheet;
            }
            try
            {
                return wb.Worksheets[sheetName];
            }
            catch (Exception)
            {
                var names = new List<string>();
                foreach (dynamic sheet in wb.Worksheets)
                {
                    names.Add(Convert.ToString(sheet.Name));
                }
                throw new InvalidOperationException("no sheet named '" + sheetName + "' (sheets: " + string.Join(", ", names) + ")");
            }
        }

        private static Dictionary<string, object> EtCreateTable(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required");
            }
            dynamic wb = EnsureWorkbook(host.Application);
            dynamic sheet = SheetFor(wb, p);
            dynamic range = sheet.Range[address];
            foreach (dynamic existing in sheet.ListObjects)
            {
                dynamic overlap = ((dynamic)host.Application).Intersect(existing.Range, range);
                if (overlap != null)
                {
                    throw new InvalidOperationException("the range overlaps table '" + Convert.ToString(existing.Name) + "' (" +
                        Convert.ToString(existing.Range.Address[false, false]) + ")");
                }
            }
            // xlSrcRange = 1, xlYes = 1 (hàng đầu là tiêu đề).
            dynamic table = sheet.ListObjects.Add(1, range, Type.Missing, 1);
            string name = ParamString(p, "name", null);
            if (!string.IsNullOrEmpty(name))
            {
                table.Name = name;
            }
            table.TableStyle = ParamString(p, "style", "TableStyleMedium2") ?? "TableStyleMedium2";
            return new Dictionary<string, object>
            {
                { "table", Convert.ToString(table.Name) }, { "sheet", Convert.ToString(sheet.Name) },
                { "range", Convert.ToString(table.Range.Address[false, false]) }, { "rows", Convert.ToInt32(table.ListRows.Count) }
            };
        }

        private static Dictionary<string, object> EtFreezePanes(IAppHost host, Dictionary<string, object> p)
        {
            int rows = Math.Max(0, ParamInt(p, "rows", 1));
            int cols = Math.Max(0, ParamInt(p, "cols", 0));
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = SheetFor(wb, p);
            dynamic previous = wb.ActiveSheet;
            // Cố định theo cửa sổ của sheet: phải kích hoạt sheet đó, xong trả lại sheet người dùng đang xem.
            sheet.Activate();
            dynamic window = app.ActiveWindow;
            window.FreezePanes = false;
            window.SplitColumn = 0;
            window.SplitRow = 0;
            if (rows > 0 || cols > 0)
            {
                window.ScrollRow = 1;
                window.ScrollColumn = 1;
                window.SplitColumn = cols;
                window.SplitRow = rows;
                window.FreezePanes = true;
            }
            try
            {
                if (Convert.ToString(previous.Name) != Convert.ToString(sheet.Name))
                {
                    previous.Activate();
                }
            }
            catch
            {
            }
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) }, { "rows", rows }, { "cols", cols }, { "frozen", rows > 0 || cols > 0 }
            };
        }

        private static Dictionary<string, object> EtSetValidation(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrEmpty(address))
            {
                throw new InvalidOperationException("'range' is required");
            }
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet = SheetFor(wb, p);
            dynamic range = sheet.Range[address];
            string source = ParamString(p, "source", null);
            List<string> values = null;
            object rawList;
            if (p != null && p.TryGetValue("list", out rawList) && rawList != null)
            {
                IList list = UnwrapScalar(rawList) as IList;
                values = list != null && !(list is string)
                    ? list.Cast<object>().Select(v => Convert.ToString(UnwrapScalar(v))).Where(v => !string.IsNullOrEmpty(v)).ToList()
                    : Convert.ToString(UnwrapScalar(rawList)).Split(',').Select(v => v.Trim()).Where(v => v.Length > 0).ToList();
            }
            string kind = (ParamString(p, "type", null) ?? (source != null || values != null ? "list" : "")).ToLowerInvariant();
            dynamic validation = range.Validation;
            validation.Delete();
            string rule;
            switch (kind)
            {
                case "list":
                {
                    string formula;
                    if (!string.IsNullOrEmpty(source))
                    {
                        formula = source.StartsWith("=") ? source : "=" + source;
                    }
                    else if (values != null && values.Count > 0)
                    {
                        // Danh sách gõ thẳng dùng dấu phân cách danh sách của máy (xlListSeparator = 5): máy Việt có thể là ';'.
                        string separator = ",";
                        try
                        {
                            separator = Convert.ToString(app.International[5]);
                        }
                        catch
                        {
                        }
                        formula = string.Join(separator, values);
                    }
                    else
                    {
                        throw new ArgumentException("a list rule needs 'list' (values) or 'source' (a range holding them)");
                    }
                    // xlValidateList = 3, xlValidAlertStop = 1, xlBetween = 1.
                    validation.Add(3, 1, 1, formula);
                    validation.InCellDropdown = true;
                    rule = "list " + formula;
                    break;
                }
                case "whole":
                case "decimal":
                case "date":
                {
                    string min = ParamString(p, "min", null);
                    string max = ParamString(p, "max", null);
                    if (min == null && max == null)
                    {
                        throw new ArgumentException("'" + kind + "' needs 'min' and/or 'max'");
                    }
                    int type = kind == "whole" ? 1 : kind == "decimal" ? 2 : 4;
                    // xlBetween = 1, xlGreaterEqual = 7, xlLessEqual = 8.
                    if (min != null && max != null)
                    {
                        validation.Add(type, 1, 1, min, max);
                    }
                    else if (min != null)
                    {
                        validation.Add(type, 1, 7, min);
                    }
                    else
                    {
                        validation.Add(type, 1, 8, max);
                    }
                    rule = kind + " " + (min ?? "") + ".." + (max ?? "");
                    break;
                }
                default:
                    throw new ArgumentException("'type' must be list, whole, decimal or date");
            }
            string message = ParamString(p, "message", null);
            if (!string.IsNullOrEmpty(message))
            {
                validation.ErrorMessage = message;
            }
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) }, { "range", Convert.ToString(range.Address[false, false]) }, { "rule", rule }
            };
        }
    }
}

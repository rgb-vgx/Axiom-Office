using System;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;

namespace AxiomOffice.Bridge
{
    // Sắp xếp và lọc vùng dữ liệu (docs/research/tinh-nang-con-thieu-so-voi-claude-m365.md X1). Sắp xếp đổi vị trí dữ liệu
    // và Excel không hoàn tác được thay đổi qua COM: Core hỏi xác nhận trước (tools/sort_guard.go), lệnh ở đây chỉ làm.
    internal static partial class CommandDispatcher
    {
        private static IEnumerable<CommandInfo> SpreadsheetSortCommands()
        {
            return new[]
            {
                Command("et.sort", "et", EtSort,
                    "Sắp xếp các hàng của một vùng theo 1-3 cột (`keys`: [{column: chữ tiêu đề hoặc chữ cột, order: asc|desc}]); mặc định hàng đầu là tiêu đề, không bị sắp xếp",
                    Req("range", "data incl. header row, e.g. 'A1:F200'"),
                    Req("keys", "[{column: 'Doanh thu' or 'C', order: 'asc'|'desc'}], 1-3 keys"),
                    Opt("header", "true (default): first row is the header"), Opt("sheet")).ForAgent(),
                Command("et.filter", "et", EtFilter,
                    "Lọc hàng (AutoFilter) theo một cột: `values` = danh sách giá trị giữ lại, hoặc `criteria` như '>100'; `clear: true` bỏ mọi bộ lọc của sheet. Chỉ ẩn hàng, không xoá dữ liệu",
                    Opt("range", "data incl. header row; default: the sheet's used range"),
                    Opt("column", "header text or column letter"), Opt("values", "array of values to keep"),
                    Opt("criteria", "e.g. '>100', '<>0', '=Hà Nội'"), Opt("clear", "true = remove all filters"), Opt("sheet")).ForAgent(),
            };
        }

        // Cột trong vùng: chữ tiêu đề (hàng đầu, không phân biệt hoa thường) hoặc chữ cột tuyệt đối ("C") -> số thứ tự 1..n
        // trong vùng.
        private static int ColumnInRange(dynamic range, string column, bool header)
        {
            string wanted = (column ?? "").Trim();
            if (wanted.Length == 0)
            {
                throw new ArgumentException("'column' is required: header text or column letter");
            }
            int first = Convert.ToInt32(range.Column);
            int cols = Convert.ToInt32(range.Columns.Count);
            if (header)
            {
                for (int c = 1; c <= cols; c++)
                {
                    string text = (Convert.ToString(range.Cells[1, c].Text) ?? "").Trim();
                    if (string.Equals(text, wanted, StringComparison.CurrentCultureIgnoreCase))
                    {
                        return c;
                    }
                }
            }
            if (wanted.Length <= 3 && wanted.All(char.IsLetter))
            {
                int index = ColumnIndex(wanted) - first + 1;
                if (index >= 1 && index <= cols)
                {
                    return index;
                }
            }
            var headers = new List<string>();
            if (header)
            {
                for (int c = 1; c <= cols; c++)
                {
                    headers.Add((Convert.ToString(range.Cells[1, c].Text) ?? "").Trim());
                }
            }
            throw new ArgumentException("column '" + wanted + "' is not in the range" +
                (headers.Count > 0 ? " (headers: " + string.Join(", ", headers) + ")" : ""));
        }

        private static Dictionary<string, object> EtSort(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrWhiteSpace(address))
            {
                throw new ArgumentException("'range' is required: the data incl. header row, e.g. 'A1:F200'");
            }
            List<Dictionary<string, object>> keys = ParamObjects(p, "keys");
            if (keys == null || keys.Count == 0 || keys.Count > 3)
            {
                throw new ArgumentException("'keys' must hold 1-3 items like [{\"column\": \"Doanh thu\", \"order\": \"desc\"}]");
            }
            bool header = ParamBool(p, "header", true);
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet;
            dynamic range = SheetRange(wb, ParamString(p, "sheet", null), address.Trim(), "'range'", out sheet);
            int rows = Convert.ToInt32(range.Rows.Count);
            if (rows < (header ? 3 : 2))
            {
                throw new ArgumentException("the range has too few rows to sort");
            }
            var columns = new List<int>();
            var orders = new List<int>();
            var report = new List<object>();
            foreach (Dictionary<string, object> key in keys)
            {
                int column = ColumnInRange(range, ParamString(key, "column", null), header);
                string order = (ParamString(key, "order", "asc") ?? "asc").Trim().ToLowerInvariant();
                bool descending = order.StartsWith("desc", StringComparison.Ordinal) || order == "z-a";
                columns.Add(column);
                orders.Add(descending ? 2 : 1); // xlDescending = 2, xlAscending = 1
                report.Add(new Dictionary<string, object> { { "column", column }, { "order", descending ? "desc" : "asc" } });
            }
            bool done = false;
            try
            {
                // Worksheet.Sort (Excel 2007+): nhiều khoá, không giới hạn 3.
                dynamic sort = sheet.Sort;
                sort.SortFields.Clear();
                for (int i = 0; i < columns.Count; i++)
                {
                    sort.SortFields.Add(range.Columns[columns[i]], 0, orders[i]); // xlSortOnValues = 0
                }
                sort.SetRange(range);
                sort.Header = header ? 1 : 2; // xlYes = 1, xlNo = 2
                sort.MatchCase = false;
                sort.Orientation = 1; // xlTopToBottom
                sort.Apply();
                done = true;
            }
            catch (Exception ex)
            {
                if (ex is System.Runtime.InteropServices.COMException && IsRetryableComError((System.Runtime.InteropServices.COMException)ex))
                {
                    throw;
                }
                Logger.Info("et.sort: Worksheet.Sort unavailable, falling back to Range.Sort: " + ex.Message);
            }
            if (!done)
            {
                // Range.Sort (Excel cũ, WPS): tối đa 3 khoá, tham số theo vị trí.
                object missing = Missing.Value;
                range.Sort(
                    range.Columns[columns[0]].Cells[1, 1], orders[0],
                    columns.Count > 1 ? (object)range.Columns[columns[1]].Cells[1, 1] : missing, missing,
                    columns.Count > 1 ? (object)orders[1] : missing,
                    columns.Count > 2 ? (object)range.Columns[columns[2]].Cells[1, 1] : missing,
                    columns.Count > 2 ? (object)orders[2] : missing,
                    header ? 1 : 2);
            }
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) }, { "range", Convert.ToString(range.Address(false, false)) },
                { "rows", header ? rows - 1 : rows }, { "keys", report }
            };
        }

        private static Dictionary<string, object> EtFilter(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            string address = ParamString(p, "range", null);
            dynamic sheet;
            dynamic range;
            if (string.IsNullOrWhiteSpace(address))
            {
                sheet = SheetFor(wb, p);
                range = sheet.UsedRange;
            }
            else
            {
                range = SheetRange(wb, ParamString(p, "sheet", null), address.Trim(), "'range'", out sheet);
            }
            if (ParamBool(p, "clear", false))
            {
                try
                {
                    if (Convert.ToBoolean(sheet.FilterMode))
                    {
                        sheet.ShowAllData();
                    }
                }
                catch
                {
                }
                try
                {
                    if (Convert.ToBoolean(sheet.AutoFilterMode))
                    {
                        sheet.AutoFilterMode = false;
                    }
                }
                catch
                {
                }
                return new Dictionary<string, object> { { "sheet", Convert.ToString(sheet.Name) }, { "cleared", true } };
            }
            int field = ColumnInRange(range, ParamString(p, "column", null), true);
            List<string> values = ParamStrings(p, "values");
            string criteria = ParamString(p, "criteria", null);
            if ((values == null || values.Count == 0) && string.IsNullOrWhiteSpace(criteria))
            {
                throw new ArgumentException("give 'values' (values to keep) or 'criteria' (e.g. '>100'), or 'clear': true");
            }
            if (values != null && values.Count > 0)
            {
                range.AutoFilter(field, values.Cast<object>().ToArray(), 7); // xlFilterValues = 7
            }
            else
            {
                range.AutoFilter(field, criteria.Trim());
            }
            int visible = -1;
            try
            {
                visible = Convert.ToInt32(range.Columns[1].SpecialCells(12).Count) - 1; // xlCellTypeVisible = 12, trừ tiêu đề
            }
            catch
            {
            }
            var result = new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) }, { "range", Convert.ToString(range.Address(false, false)) },
                { "column", field }
            };
            if (visible >= 0)
            {
                result["visibleRows"] = visible;
            }
            return result;
        }
    }
}

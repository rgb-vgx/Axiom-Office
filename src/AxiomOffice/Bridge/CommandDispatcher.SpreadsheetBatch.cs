using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;

namespace AxiomOffice.Bridge
{
    // Đọc nhiều vùng / thử đổi một giả định trong MỘT lời gọi (bộ đo 27/10/2026 ca B: 26/60 vòng cuối là đọc từng vùng
    // một và bước "đổi giả định -> đọc ô phụ thuộc -> trả lại" của skill mo-hinh-nhieu-sheet, mỗi vòng một lệnh).
    internal static partial class CommandDispatcher
    {
        private const int ReadRangesCellLimit = 4000;
        private const int WhatIfCellLimit = 400;

        // "Data!A1:C5", "'Kế hoạch'!B2" hoặc "A1:C5" (sheet = `fallback`, rỗng = sheet đang mở).
        private static dynamic SheetRange(dynamic wb, string fallback, string text, string label, out dynamic sheet)
        {
            string sheetName = fallback;
            string local = text.Trim();
            int bang = local.LastIndexOf('!');
            if (bang > 0)
            {
                sheetName = local.Substring(0, bang).Trim().Trim('\'').Replace("''", "'");
                local = local.Substring(bang + 1).Trim();
            }
            try
            {
                sheet = string.IsNullOrEmpty(sheetName) ? wb.ActiveSheet : wb.Worksheets[sheetName];
            }
            catch (Exception)
            {
                throw new InvalidOperationException(string.Format("{0}: no sheet named '{1}' (sheets: {2})", label, sheetName,
                    string.Join(", ", SheetNames(wb))));
            }
            try
            {
                return sheet.Range[local];
            }
            catch (Exception)
            {
                throw new InvalidOperationException(string.Format("{0}: '{1}' is not a range address", label, text));
            }
        }

        private static Dictionary<string, object> EtReadRanges(IAppHost host, Dictionary<string, object> p)
        {
            List<Dictionary<string, object>> reads = ParamObjects(p, "reads");
            if (reads == null || reads.Count == 0)
            {
                throw new InvalidOperationException("'reads' is required: a list of {range, sheet?, formulas?}, e.g. "
                    + "[{\"sheet\": \"Dashboard\", \"range\": \"A1:B6\"}, {\"range\": \"Config!B2:B5\"}]");
            }
            dynamic wb = EnsureWorkbook(host.Application);
            string defaultSheet = ParamString(p, "sheet", null);
            var targets = new List<dynamic>();
            var sheets = new List<dynamic>();
            int cells = 0;
            for (int i = 0; i < reads.Count; i++)
            {
                string address = ParamString(reads[i], "range", null);
                if (string.IsNullOrEmpty(address))
                {
                    throw new InvalidOperationException(string.Format("'reads[{0}]' needs 'range', e.g. 'A1:C10'", i));
                }
                dynamic sheet;
                dynamic range = SheetRange(wb, ParamString(reads[i], "sheet", defaultSheet), address, "'reads[" + i + "]'", out sheet);
                cells += Convert.ToInt32(range.Rows.Count) * Convert.ToInt32(range.Columns.Count);
                targets.Add(range);
                sheets.Add(sheet);
            }
            if (cells > ReadRangesCellLimit)
            {
                throw new InvalidOperationException(string.Format("the ranges hold {0} cells (limit {1} per call): read only the cells "
                    + "you need, or use et.readRange for one large area", cells, ReadRangesCellLimit));
            }
            var results = new List<Dictionary<string, object>>();
            for (int i = 0; i < reads.Count; i++)
            {
                var result = new Dictionary<string, object>
                {
                    { "sheet", Convert.ToString(sheets[i].Name) },
                    { "range", Convert.ToString(targets[i].Address(false, false)) }
                };
                if (ParamBool(reads[i], "formulas", ParamBool(p, "formulas", false)))
                {
                    result["formulas"] = ToFormulaMatrix(targets[i].Formula);
                }
                else
                {
                    result["values"] = ToMatrix(targets[i].Value2);
                }
                results.Add(result);
            }
            return new Dictionary<string, object> { { "reads", results }, { "cells", cells } };
        }

        // Đổi tạm MỘT ô giả định, tính lại, đọc các vùng theo dõi, rồi TRẢ LẠI giá trị / công thức cũ (kể cả khi lỗi giữa
        // chừng). Ô theo dõi không đổi = công thức không đọc ô giả định đó (ô điều khiển "chết").
        // Kết quả dùng `moved` (không phải `changed`): Core đọc "changed": false là "lệnh ghi không đổi gì" và chặn lần gọi
        // lặp lại - whatIf là phép thử chỉ đọc, gọi lại với vùng theo dõi khác là chuyện bình thường.
        private static Dictionary<string, object> EtWhatIf(IAppHost host, Dictionary<string, object> p)
        {
            string cellText = ParamString(p, "cell", null);
            if (string.IsNullOrEmpty(cellText))
            {
                throw new InvalidOperationException("'cell' is required: the input cell to change for a moment, e.g. 'Config!B2'");
            }
            if (!HasParam(p, "value"))
            {
                throw new InvalidOperationException("'value' is required: the test value, e.g. 0.95 (the old value is put back after)");
            }
            object rawWatch;
            var watchTexts = new List<string>();
            if (p.TryGetValue("watch", out rawWatch) && rawWatch != null)
            {
                if (rawWatch is string)
                {
                    watchTexts.AddRange(((string)rawWatch).Split(new[] { ';' }, StringSplitOptions.RemoveEmptyEntries));
                }
                else if (rawWatch is IEnumerable)
                {
                    foreach (object item in (IEnumerable)rawWatch)
                    {
                        var entry = UnwrapScalar(item) as Dictionary<string, object>;
                        if (entry != null)
                        {
                            string range = ParamString(entry, "range", "");
                            string sheet = ParamString(entry, "sheet", null);
                            watchTexts.Add(string.IsNullOrEmpty(sheet) || range.Contains("!") ? range : "'" + sheet.Replace("'", "''") + "'!" + range);
                        }
                        else
                        {
                            watchTexts.Add(Convert.ToString(UnwrapScalar(item)));
                        }
                    }
                }
            }
            watchTexts = watchTexts.Select(t => t.Trim()).Where(t => t.Length > 0).ToList();
            if (watchTexts.Count == 0)
            {
                throw new InvalidOperationException("'watch' is required: the cells that should follow the input, e.g. "
                    + "[\"Dashboard!B3:B6\", \"Project_Data!E2:E4\"]");
            }

            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            string defaultSheet = ParamString(p, "sheet", null);
            dynamic inputSheet;
            dynamic input = SheetRange(wb, defaultSheet, cellText, "'cell'", out inputSheet);
            if (Convert.ToInt32(input.Count) != 1)
            {
                throw new InvalidOperationException("'cell' must be ONE cell, got '" + cellText + "'");
            }
            if (Convert.ToBoolean(input.HasFormula))
            {
                throw new InvalidOperationException("'" + cellText + "' holds a formula: test an INPUT cell (a typed value) that "
                    + "the formulas read");
            }
            var watched = new List<dynamic>();
            var watchSheets = new List<dynamic>();
            int cells = 0;
            for (int i = 0; i < watchTexts.Count; i++)
            {
                dynamic sheet;
                dynamic range = SheetRange(wb, Convert.ToString(inputSheet.Name), watchTexts[i], "'watch[" + i + "]'", out sheet);
                cells += Convert.ToInt32(range.Count);
                watched.Add(range);
                watchSheets.Add(sheet);
            }
            if (cells > WhatIfCellLimit)
            {
                throw new InvalidOperationException(string.Format("'watch' covers {0} cells (limit {1}): watch the totals / KPI cells only",
                    cells, WhatIfCellLimit));
            }

            object original = input.Value2;
            object value = UnwrapScalar(p["value"]);
            var before = watched.Select(r => (object)ToMatrix(r.Value2)).ToList();
            var after = new List<object>();
            object tested;
            try
            {
                string text = value as string;
                if (text != null && text.StartsWith("="))
                {
                    throw new InvalidOperationException("'value' must be a value, not a formula");
                }
                input.Value2 = value;
                app.Calculate();
                tested = input.Value2;
                foreach (dynamic range in watched)
                {
                    after.Add(ToMatrix(range.Value2));
                }
            }
            finally
            {
                input.Value2 = original;
                app.Calculate();
            }

            var report = new List<Dictionary<string, object>>();
            var unchanged = new List<string>();
            for (int i = 0; i < watched.Count; i++)
            {
                string name = Convert.ToString(watchSheets[i].Name) + "!" + Convert.ToString(watched[i].Address(false, false));
                bool changed = MatrixText(before[i]) != MatrixText(after[i]);
                if (!changed)
                {
                    unchanged.Add(name);
                }
                report.Add(new Dictionary<string, object>
                {
                    { "range", name }, { "before", before[i] }, { "after", after[i] }, { "moved", changed }
                });
            }
            var output = new Dictionary<string, object>
            {
                { "cell", Convert.ToString(inputSheet.Name) + "!" + Convert.ToString(input.Address(false, false)) },
                { "original", original }, { "tested", tested }, { "restored", true }, { "watch", report },
                { "moved", watched.Count - unchanged.Count }, { "unchanged", unchanged }
            };
            if (unchanged.Count > 0)
            {
                // Ô ngưỡng chỉ định dạng có điều kiện / validation đọc (vd "cảnh báo vượt 90%") thì giá trị không đổi theo là
                // ĐÚNG - báo "dead control" ở đây làm model sửa định dạng có điều kiện mãi (bộ đo 27/10/2026 ca B, p3l).
                List<string> rules = RulesReading(wb, inputSheet, input);
                if (rules.Count > 0)
                {
                    output["usedBy"] = rules;
                    output["hint"] = "no watched value moved, but " + output["cell"] + " is read by " + string.Join("; ", rules)
                        + ": it is a threshold for formatting/validation, not a dead control. Nothing to fix; to test that "
                        + "values follow the inputs, try an input that formulas read (a budget or cost cell)";
                }
                else
                {
                    output["hint"] = "these cells did not move when the input changed: their formulas do not read "
                        + output["cell"] + " (dead control) - point them at the input cell, or drop them from 'watch' if they "
                        + "should not depend on it";
                }
            }
            return output;
        }

        // Định dạng có điều kiện / validation có công thức đọc ô `input` (theo địa chỉ, có hoặc không tên sheet).
        private static List<string> RulesReading(dynamic wb, dynamic inputSheet, dynamic input)
        {
            var found = new List<string>();
            string inputName = Convert.ToString(inputSheet.Name);
            string plain = Convert.ToString(input.Address(false, false)).ToUpperInvariant();
            foreach (dynamic ws in wb.Worksheets)
            {
                string wsName = Convert.ToString(ws.Name);
                var texts = new List<KeyValuePair<string, string>>();
                try
                {
                    foreach (dynamic condition in ws.Cells.FormatConditions)
                    {
                        string where = SafeAddress(() => condition.AppliesTo.Address(false, false));
                        texts.Add(new KeyValuePair<string, string>("conditional format " + wsName + "!" + where,
                            SafeAddress(() => condition.Formula1) + " " + SafeAddress(() => condition.Formula2)));
                    }
                }
                catch (Exception)
                {
                }
                try
                {
                    foreach (dynamic area in ws.Cells.SpecialCells(-4174).Areas)
                    {
                        texts.Add(new KeyValuePair<string, string>("validation " + wsName + "!" + Convert.ToString(area.Address(false, false)),
                            SafeAddress(() => area.Validation.Formula1) + " " + SafeAddress(() => area.Validation.Formula2)));
                    }
                }
                catch (Exception)
                {
                    // SpecialCells ném lỗi khi sheet không có validation.
                }
                foreach (KeyValuePair<string, string> entry in texts)
                {
                    string formula = entry.Value.Replace("$", "").Replace("'", "").ToUpperInvariant();
                    bool sameSheet = string.Equals(wsName, inputName, StringComparison.OrdinalIgnoreCase);
                    bool reads = formula.Contains(inputName.ToUpperInvariant() + "!" + plain)
                        || (sameSheet && System.Text.RegularExpressions.Regex.IsMatch(formula, @"(?<![A-Z0-9!])" + plain + @"(?![0-9])"));
                    if (reads && !found.Contains(entry.Key))
                    {
                        found.Add(entry.Key);
                    }
                }
                if (found.Count >= 6)
                {
                    break;
                }
            }
            return found;
        }

        private static string SafeAddress(Func<object> read)
        {
            try
            {
                return Convert.ToString(read());
            }
            catch (Exception)
            {
                return "";
            }
        }

        private static string MatrixText(object matrix)
        {
            var rows = matrix as List<List<object>>;
            if (rows == null)
            {
                return Convert.ToString(matrix);
            }
            return string.Join("\n", rows.Select(row => string.Join("\t", row.Select(cell => Convert.ToString(cell)))));
        }
    }
}

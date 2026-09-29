using System;
using System.Collections;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Runtime.InteropServices;

namespace AxiomOffice.Bridge
{
    internal static partial class CommandDispatcher
    {
        // Đọc tham số, chuyển giá trị COM sang JSON, và các tiện ích dùng chung của handler.

        private static Dictionary<string, object> Ok(object result)
        {
            return new Dictionary<string, object> { { "ok", true }, { "result", result } };
        }

        private static Dictionary<string, object> Err(string message)
        {
            return new Dictionary<string, object> { { "ok", false }, { "error", message } };
        }

        private static string ParamString(Dictionary<string, object> p, string name, string fallback)
        {
            object v;
            if (p != null && p.TryGetValue(name, out v) && v != null)
            {
                return Convert.ToString(v);
            }
            return fallback;
        }

        // Sai kiểu thì báo tên tham số + giá trị nhận được (thay cho "FormatException: Input string was
        // not in a correct format.") để model tự sửa ở vòng sau.
        private static int ParamInt(Dictionary<string, object> p, string name, int fallback)
        {
            object v;
            if (p == null || !p.TryGetValue(name, out v) || v == null)
            {
                return fallback;
            }
            try
            {
                // Model hay gửi số dạng chuỗi ("44", "12.0").
                var text = v as string;
                double number;
                if (text != null && double.TryParse(text.Trim(), NumberStyles.Float, CultureInfo.InvariantCulture, out number))
                {
                    return Convert.ToInt32(number);
                }
                return Convert.ToInt32(v);
            }
            catch (Exception ex) when (ex is FormatException || ex is InvalidCastException || ex is OverflowException)
            {
                throw new ArgumentException("'" + name + "' must be a whole number, got " + DescribeValue(v));
            }
        }

        private static bool ParamBool(Dictionary<string, object> p, string name, bool fallback)
        {
            object v;
            if (p == null || !p.TryGetValue(name, out v) || v == null)
            {
                return fallback;
            }
            if (v is bool)
            {
                return (bool)v;
            }
            try
            {
                return Convert.ToBoolean(v);
            }
            catch (Exception ex) when (ex is FormatException || ex is InvalidCastException)
            {
                throw new ArgumentException("'" + name + "' must be true or false, got " + DescribeValue(v));
            }
        }

        private static string DescribeValue(object value)
        {
            var text = value as string;
            if (text != null)
            {
                return "'" + (text.Length > 40 ? text.Substring(0, 40) + "..." : text) + "'";
            }
            return value is IList ? "an array" : DescribeShape(value);
        }

        private static void RequireKind(IAppHost host, string kind)
        {
            if (host.AppKind != kind)
            {
                throw new InvalidOperationException(
                    "command requires '" + kind + "' but the connected application is '" + host.AppKind + "'");
            }
        }

        private static string ExcelErrorName(int code)
        {
            switch (code)
            {
                case -2146826288:
                    return "#NULL!";
                case -2146826281:
                    return "#DIV/0!";
                case -2146826273:
                    return "#VALUE!";
                case -2146826265:
                    return "#REF!";
                case -2146826259:
                    return "#NAME?";
                case -2146826252:
                    return "#NUM!";
                default:
                    return null;
            }
        }

        private static object Safe(Func<object> getter)
        {
            try
            {
                return getter();
            }
            catch
            {
                return null;
            }
        }

        private static object ToPlain(object value)
        {
            if (value == null)
            {
                return null;
            }
            if (value is int)
            {
                int intValue = (int)value;
                if (intValue == -2146826246 || intValue == -2147352572)
                {
                    return null;
                }
                string excelError = ExcelErrorName(intValue);
                if (excelError != null)
                {
                    return excelError;
                }
                return intValue;
            }
            if (value is string || value is bool || value is long || value is double || value is decimal)
            {
                return value;
            }
            if (value is float)
            {
                return (double)(float)value;
            }
            if (value is short)
            {
                return (int)(short)value;
            }
            if (value is byte)
            {
                return (int)(byte)value;
            }
            if (value is DateTime)
            {
                return ((DateTime)value).ToString("o");
            }
            if (value is Array)
            {
                return ToMatrix(value);
            }
            return Convert.ToString(value);
        }

        // Đọc tham số mảng 2 chiều (values của writeRange/insertTable/addTable). Model đôi khi gửi mảng
        // bọc trong {"item": ...} hoặc dạng chuỗi JSON: gỡ ra được thì dùng, sai dạng thì báo lỗi kèm ví dụ
        // để model tự sửa ở vòng sau (trước đây bị bỏ qua âm thầm mà vẫn trả ok).
        private static List<IList> ParamMatrix(Dictionary<string, object> p, string name, bool required)
        {
            object raw;
            if (p == null || !p.TryGetValue(name, out raw) || raw == null)
            {
                if (required)
                {
                    throw new ArgumentException(MatrixHelp(name, "missing"));
                }
                return null;
            }
            var text = raw as string;
            string trimmed = text == null ? null : text.TrimStart();
            if (trimmed != null && (trimmed.StartsWith("[", StringComparison.Ordinal) || trimmed.StartsWith("{", StringComparison.Ordinal)))
            {
                try
                {
                    raw = new System.Web.Script.Serialization.JavaScriptSerializer().DeserializeObject(text);
                }
                catch (Exception)
                {
                }
            }
            object decoded = DecodeArrays(raw);
            var list = decoded as IList;
            if (list == null)
            {
                throw new ArgumentException(MatrixHelp(name, DescribeShape(decoded)));
            }
            // Lớp bọc thừa ngoài cùng: [[["a","b"],["c","d"]]] → [["a","b"],["c","d"]].
            while (list.Count == 1)
            {
                var only = list[0] as IList;
                if (only == null || only.Count == 0 || !only.Cast<object>().All(i => i is IList))
                {
                    break;
                }
                list = only;
            }
            if (list.Count == 0)
            {
                if (required)
                {
                    throw new ArgumentException(MatrixHelp(name, "an empty array"));
                }
                return new List<IList>();
            }
            var items = list.Cast<object>().ToList();
            bool allRows = items.All(i => i is IList);
            bool allScalars = items.All(i => !(i is IList) && !(i is IDictionary));
            if (!allRows && !allScalars)
            {
                throw new ArgumentException(MatrixHelp(name, "a mix of rows and single values"));
            }
            var rows = new List<IList>();
            for (int r = 0; r < items.Count; r++)
            {
                // Mảng 1 chiều: mỗi phần tử là một dòng 1 ô (giữ hành vi cũ của et.writeRange).
                IList source = allRows ? (IList)items[r] : new object[] { items[r] };
                var row = new List<object>();
                for (int c = 0; c < source.Count; c++)
                {
                    // Ô bị bọc thành mảng 1 phần tử ({"item":"a"} → ["a"]): lấy giá trị bên trong.
                    object cell = source[c];
                    IList wrapped;
                    while ((wrapped = cell as IList) != null && wrapped.Count == 1)
                    {
                        cell = wrapped[0];
                    }
                    if (cell is IList || cell is IDictionary)
                    {
                        throw new ArgumentException(MatrixHelp(name, "a nested array/object at row " + (r + 1) + ", column " + (c + 1)));
                    }
                    row.Add(cell);
                }
                rows.Add(row);
            }
            return rows;
        }

        // Model đôi khi mã hoá mảng kiểu XML: mỗi lớp {"item": x} là MỘT cấp mảng - x là danh sách phần tử,
        // hoặc là phần tử duy nhất khi x không phải mảng. Vd {"item":{"item":["Tổng","=SUM(B2:B3)"]}} =
        // [["Tổng","=SUM(B2:B3)"]] (một dòng); trước đây gỡ hết các lớp một lúc thành mảng 1 chiều nên bị
        // ghi thành cột. Kết quả chỉ còn List và giá trị đơn; object khác giữ nguyên để báo lỗi.
        private static object DecodeArrays(object value)
        {
            var dict = value as IDictionary<string, object>;
            if (dict != null && dict.Count == 1)
            {
                string key = dict.Keys.First();
                if (key == "item" || key == "items" || key == "row" || key == "rows" || key == "values")
                {
                    object inner = dict[key];
                    var innerList = inner as IList;
                    return innerList != null
                        ? innerList.Cast<object>().Select(DecodeArrays).ToList()
                        : new List<object> { DecodeArrays(inner) };
                }
            }
            var list = value as IList;
            return list != null ? list.Cast<object>().Select(DecodeArrays).ToList() : value;
        }

        private static string DescribeShape(object value)
        {
            var dict = value as IDictionary<string, object>;
            if (dict != null)
            {
                return "an object with keys [" + string.Join(", ", dict.Keys.Take(5).ToArray()) + "]";
            }
            return value == null ? "null" : "a " + value.GetType().Name + " value";
        }

        private static string MatrixHelp(string name, string got)
        {
            return "'" + name + "' must be a JSON 2D array (a list of rows), e.g. [[\"Họ tên\",\"Điểm\"],[\"An\",9.5],[\"Bình\",8]] - got " + got;
        }

        private static List<List<object>> ToMatrix(object value)
        {
            var rows = new List<List<object>>();
            Array array = value as Array;
            if (array == null)
            {
                rows.Add(new List<object> { ToPlain(value) });
                return rows;
            }

            int r0 = array.GetLowerBound(0);
            int r1 = array.GetUpperBound(0);
            int c0 = array.GetLowerBound(1);
            int c1 = array.GetUpperBound(1);
            for (int r = r0; r <= r1; r++)
            {
                var row = new List<object>();
                for (int c = c0; c <= c1; c++)
                {
                    row.Add(ToPlain(array.GetValue(r, c)));
                }
                rows.Add(row);
            }
            return rows;
        }

        private sealed class UndoRecordScope : IDisposable
        {
            private readonly object _undoRecord;

            public UndoRecordScope(object app, string label)
            {
                try
                {
                    dynamic dynamicApp = app;
                    object record = dynamicApp.UndoRecord;
                    if (record != null)
                    {
                        dynamic dynamicRecord = record;
                        dynamicRecord.StartCustomRecord(label);
                        _undoRecord = record;
                    }
                }
                catch
                {
                    _undoRecord = null;
                }
            }

            public void Dispose()
            {
                if (_undoRecord != null)
                {
                    try
                    {
                        dynamic dynamicRecord = _undoRecord;
                        dynamicRecord.EndCustomRecord();
                    }
                    catch
                    {
                    }
                }
            }
        }

        private static bool HasParam(Dictionary<string, object> p, string name)
        {
            object v;
            return p != null && p.TryGetValue(name, out v) && v != null;
        }

        private static int? ParseBgrColor(string hex)
        {
            if (string.IsNullOrEmpty(hex))
            {
                return null;
            }
            string text = hex.Trim().TrimStart('#');
            if (text.Length != 6)
            {
                return null;
            }
            try
            {
                int r = Convert.ToInt32(text.Substring(0, 2), 16);
                int g = Convert.ToInt32(text.Substring(2, 2), 16);
                int b = Convert.ToInt32(text.Substring(4, 2), 16);
                return r + (g << 8) + (b << 16);
            }
            catch
            {
                return null;
            }
        }

        private static int ParseAlignment(string alignment)
        {
            switch ((alignment ?? "left").ToLowerInvariant())
            {
                case "center":
                    return 1;
                case "right":
                    return 2;
                case "justify":
                    return 3;
                default:
                    return 0;
            }
        }

        private static Dictionary<string, object> SaveDocument(IAppHost host, string kind, string path)
        {
            dynamic app = host.Application;
            dynamic target;
            if (kind == "wps")
            {
                target = app.ActiveDocument;
            }
            else if (kind == "et")
            {
                target = app.ActiveWorkbook;
            }
            else
            {
                target = app.ActivePresentation;
            }

            if (!string.IsNullOrEmpty(path))
            {
                try
                {
                    target.SaveAs2(path);
                }
                catch
                {
                    target.SaveAs(path);
                }
                return new Dictionary<string, object>
                {
                    { "saved", true },
                    { "fullName", Convert.ToString(target.FullName) }
                };
            }

            target.Save();
            return new Dictionary<string, object>
            {
                { "saved", true },
                { "fullName", Convert.ToString(target.FullName) }
            };
        }
    }
}

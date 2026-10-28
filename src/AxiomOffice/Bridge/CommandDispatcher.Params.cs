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
            v = UnwrapScalar(v);
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

        // Số bị bọc kiểu mảng XML ({"item": 6}) hoặc mảng 1 phần tử ([6]): lấy giá trị bên trong.
        private static object UnwrapScalar(object v)
        {
            for (int depth = 0; depth < 8; depth++)
            {
                var dict = v as IDictionary<string, object>;
                if (dict != null && dict.Count == 1)
                {
                    string key = dict.Keys.First();
                    if (key != "item" && key != "items" && key != "value")
                    {
                        return v;
                    }
                    v = dict[key];
                    continue;
                }
                var list = v as IList;
                if (list == null || list.Count != 1)
                {
                    return v;
                }
                v = list[0];
            }
            return v;
        }

        // Như ParamInt nhưng giữ phần thập phân (kích thước biểu đồ tính bằng cm).
        private static double ParamDouble(Dictionary<string, object> p, string name, double fallback)
        {
            object v;
            if (p == null || !p.TryGetValue(name, out v) || v == null)
            {
                return fallback;
            }
            v = UnwrapScalar(v);
            try
            {
                var text = v as string;
                double number;
                if (text != null && double.TryParse(text.Trim(), NumberStyles.Float, CultureInfo.InvariantCulture, out number))
                {
                    return number;
                }
                return Convert.ToDouble(v, CultureInfo.InvariantCulture);
            }
            catch (Exception ex) when (ex is FormatException || ex is InvalidCastException || ex is OverflowException)
            {
                throw new ArgumentException("'" + name + "' must be a number, got " + DescribeValue(v));
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

        // Ma loi Excel -> ten hien thi; KHONG nhan ra thi tra null de cho goi tra ve so goc.
        // Ten khac ban trong CommandDispatcher.Checks.cs (ban do tra "#ERR<ma>" va co them #N/A) vi hai noi
        // co hop dong khac nhau - doi ten de tranh trung thanh vien trong cung lop partial (CS0111).
        private static string ExcelErrorNameOrNull(int code)
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
                string excelError = ExcelErrorNameOrNull(intValue);
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
        // ParamObjects: mảng ĐỐI TƯỢNG trong params (ví dụ `rules` của et.setConditionalFormat).
        // Nhận cả mảng đã giải mã lẫn chuỗi JSON, vì có client gửi mảng dưới dạng chuỗi (ParamMatrix
        // cũng phải đỡ trường hợp đó).
        private static List<Dictionary<string, object>> ParamObjects(Dictionary<string, object> p, string name)
        {
            object raw;
            if (p == null || !p.TryGetValue(name, out raw) || raw == null)
            {
                return null;
            }
            var text = raw as string;
            if (text != null && text.TrimStart().StartsWith("[", StringComparison.Ordinal))
            {
                try
                {
                    raw = new System.Web.Script.Serialization.JavaScriptSerializer().DeserializeObject(text);
                }
                catch (Exception)
                {
                }
            }
            var items = new List<Dictionary<string, object>>();
            var list = raw as IList;
            if (list == null)
            {
                var single = AsStringMap(raw);
                if (single != null)
                {
                    items.Add(single);
                }
                return items;
            }
            foreach (object item in list)
            {
                var row = AsStringMap(item);
                if (row != null)
                {
                    items.Add(row);
                }
            }
            return items;
        }

        private static Dictionary<string, object> AsStringMap(object value)
        {
            var row = value as Dictionary<string, object>;
            if (row != null)
            {
                return row;
            }
            var map = value as IDictionary;
            if (map == null)
            {
                return null;
            }
            row = new Dictionary<string, object>();
            foreach (DictionaryEntry entry in map)
            {
                row[Convert.ToString(entry.Key)] = entry.Value;
            }
            return row;
        }

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
                // Dòng bị bọc thêm một lớp ({"item":{"item":[...]}} → [[...]]): lấy dòng bên trong.
                IList inner;
                while (source.Count == 1 && (inner = source[0] as IList) != null)
                {
                    source = inner;
                }
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

        private static Dictionary<string, object> SaveDocument(IAppHost host, string kind, string path, string presentation = null)
        {
            dynamic app = host.Application;
            dynamic target;
            if (kind == "wpp" && !string.IsNullOrWhiteSpace(presentation))
            {
                target = TargetPresentation(app, new Dictionary<string, object> { { "presentation", presentation } });
            }
            else if (kind == "wps")
            {
                target = app.ActiveDocument;
            }
            else if (kind == "et")
            {
                target = app.ActiveWorkbook;
            }
            else
            {
                target = CurrentPresentation(app);
            }

            if (!string.IsNullOrEmpty(path))
            {
                // Word 16.0.20527 (Windows) không còn suy diễn định dạng được khi chỉ truyền tên file:
                // SaveAs2("x.docx") trên tài liệu mới -> "Incompatible file type and file extension".
                // Do đó chọn định dạng theo phần mở rộng; tên file lệch (không đường dẫn) lùi về Documents\Axiom.
                string full = ResolveSavePath(path);
                int format = SaveFormatFor(kind, full);
                // Excel/ET hỏi "file đã tồn tại, thay không?" bằng hộp thoại: tắt cảnh báo trong lúc lưu.
                object alerts = null;
                try { alerts = app.DisplayAlerts; } catch { }
                try
                {
                    SetAlertsOff(app, kind);
                    if (format >= 0)
                    {
                        try { target.SaveAs2(full, format); }
                        catch { target.SaveAs(full, format); }
                    }
                    else
                    {
                        // Phần mở rộng lạ: giữ hành vi cũ (để COM tự quyết, lỗi sẽ báo lên như trước).
                        try { target.SaveAs2(full); }
                        catch { target.SaveAs(full); }
                    }
                }
                finally
                {
                    if (alerts != null)
                    {
                        try { app.DisplayAlerts = alerts; } catch { }
                    }
                }
                return new Dictionary<string, object>
                {
                    { "saved", true },
                    { "fullName", Convert.ToString(target.FullName) }
                };
            }

            // Tài liệu chưa từng lưu (trên đĩa không có file): Save() sẽ khoe hộp thoại Save As
            // làm treo bridge. Lưu luôn vào Documents\Axiom với định dạng mặc định của app đó.
            try
            {
                string fullName = Convert.ToString(target.FullName);
                if (string.IsNullOrEmpty(fullName) || !System.IO.File.Exists(fullName))
                {
                    string extension;
                    int defFormat = DefaultSaveFormat(kind, out extension);
                    string stem = SanitizeFileName(System.IO.Path.GetFileName(fullName));
                    System.IO.Directory.CreateDirectory(DefaultSaveDir());
                    // Không ghi đè file có sẵn (vd Document1.docx của lần trước): thêm " (2)", " (3)"...
                    string full = System.IO.Path.Combine(DefaultSaveDir(), stem + extension);
                    for (int n = 2; System.IO.File.Exists(full); n++)
                    {
                        full = System.IO.Path.Combine(DefaultSaveDir(), stem + " (" + n + ")" + extension);
                    }
                    object previousAlerts = null;
                    try { previousAlerts = app.DisplayAlerts; } catch { }
                    try
                    {
                        SetAlertsOff(app, kind);
                        try { target.SaveAs2(full, defFormat); }
                        catch { target.SaveAs(full, defFormat); }
                    }
                    finally
                    {
                        // Trả lại cảnh báo của Office: để tắt thì đóng tài liệu chưa lưu sẽ không còn hỏi người dùng.
                        if (previousAlerts != null)
                        {
                            try { app.DisplayAlerts = previousAlerts; } catch { }
                        }
                    }
                    return new Dictionary<string, object>
                    {
                        { "saved", true },
                        { "fullName", Convert.ToString(target.FullName) }
                    };
                }
            }
            catch (ArgumentException) { throw; }
            catch (OutOfMemoryException) { throw; }
            catch { /* Lưu thường như cũ để không đổi hành vi không cần thiết. */ }

            target.Save();
            return new Dictionary<string, object>
            {
                { "saved", true },
                { "fullName", Convert.ToString(target.FullName) }
            };
        }

        // Thư mục lưu mặc định khi 'path' chỉ mang tên file lệch (không có đường dẫn): Documents\Axiom.
        private static string DefaultSaveDir()
        {
            return System.IO.Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.MyDocuments),
                "Axiom");
        }

        private static string ResolveSavePath(string path)
        {
            if (System.IO.Path.IsPathRooted(path))
            {
                return path;
            }
            if (string.IsNullOrWhiteSpace(System.IO.Path.GetFileName(path)))
            {
                throw new ArgumentException("'path' không hợp lệ, cần có tên file, ví dụ 'bao-cao.docx'");
            }
            string full = System.IO.Path.Combine(DefaultSaveDir(), path);
            // Core chỉ hỏi "ghi đè file đã có" theo đường dẫn nó nhận được (policy.go, os.Stat) - đường dẫn tương đối thì
            // Core không thấy file trong Documents\Axiom, còn ở đây cảnh báo của Office bị tắt: ghi đè sẽ im lặng. Báo lỗi
            // kèm đường dẫn đầy đủ; gửi lại đúng đường dẫn đó thì Core hỏi người dùng trước khi ghi đè.
            if (System.IO.File.Exists(full))
            {
                throw new ArgumentException("file already exists: " + full +
                    " - to overwrite it, call again with this full path (the user is asked first); otherwise pick another name");
            }
            string folder = System.IO.Path.GetDirectoryName(full);
            if (!string.IsNullOrEmpty(folder))
            {
                System.IO.Directory.CreateDirectory(folder);
            }
            return full;
        }

        // Mã định dạng lưu (WdSaveFormat/XlFileFormat/PpSaveAsFileType) theo phần mở rộng, ứng với từng app.
        // Truyền cụ thể để không ngầm suy diễn - WPS Writer/ET/WPP cũng hiểu các mã này như Word/Excel/PowerPoint.
        private static int SaveFormatFor(string kind, string path)
        {
            string ext = (System.IO.Path.GetExtension(path) ?? "").ToLowerInvariant();
            switch (kind)
            {
                case "wps":
                    switch (ext)
                    {
                        case ".doc": return 0;    // wdFormatDocument (Word 97-2003)
                        case ".docx": return 16;  // wdFormatDocumentDefault
                        case ".docm": return 13;  // wdFormatXMLDocumentMacroEnabled
                        case ".rtf": return 6;    // wdFormatRTF
                        case ".txt": return 7;    // wdFormatUnicodeText
                        case ".pdf": return 17;   // wdFormatPDF
                    }
                    break;

                case "et":
                    switch (ext)
                    {
                        case ".xlsx": return 51;  // xlOpenXMLWorkbook
                        case ".xlsm": return 52;  // xlOpenXMLWorkbookMacroEnabled
                        case ".xltx": return 54;  // xlOpenXMLTemplate
                        case ".xls": return 56;   // xlExcel8 (Excel 97-2003)
                        case ".csv": return 62;   // xlCSVUTF8
                        case ".ods": return 61;   // xlOpenDocumentSpreadsheet
                        case ".pdf": return 57;   // xlTypePDF
                    }
                    break;

                case "wpp":
                    switch (ext)
                    {
                        case ".ppt": return 1;    // ppSaveAsPresentation (PowerPoint 97-2003)
                        case ".pptx": return 24;  // ppSaveAsOpenXMLPresentation
                        case ".pptm": return 25;  // ppSaveAsOpenXMLPresentationMacroEnabled
                        case ".potx": return 26;  // ppSaveAsOpenXMLTemplate
                        case ".ppsx": return 28;  // ppSaveAsOpenXMLShow
                        case ".ppsm": return 29;  // ppSaveAsOpenXMLShowMacroEnabled
                        case ".odp": return 31;   // ppSaveAsODP
                        case ".pdf": return 32;   // ppSaveAsPDF
                    }
                    break;
            }
            return -1; // Phần mở rộng lạ: người gọi tự quyết, không can thiệp COM.
        }

        // Định dạng và phần mở rộng mặc định cho tài liệu chưa từng lưu (wps=docx, et=xlsx, wpp=pptx).
        private static int DefaultSaveFormat(string kind, out string extension)
        {
            if (kind == "et") { extension = ".xlsx"; return 51; }
            if (kind == "wpp") { extension = ".pptx"; return 24; }
            extension = ".docx"; return 16;
        }

        private static string SanitizeFileName(string name)
        {
            if (string.IsNullOrWhiteSpace(name))
            {
                return "tai-lieu";
            }
            foreach (char c in System.IO.Path.GetInvalidFileNameChars())
            {
                name = name.Replace(c, '_');
            }
            return name;
        }

        // Word cần WdAlertLevel (0 = tắt), Excel/ET là bool, PowerPoint là PpAlertLevel (1 = tắt).
        private static void SetAlertsOff(dynamic app, string kind)
        {
            try
            {
                if (kind == "et") { app.DisplayAlerts = false; }
                else if (kind == "wpp") { app.DisplayAlerts = 1; } // ppAlertsNone
                else { app.DisplayAlerts = 0; } // wdAlertsNone
            }
            catch { }
        }
    }
}

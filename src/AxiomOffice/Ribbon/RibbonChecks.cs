using System;
using System.Collections;
using System.Collections.Generic;
using System.Globalization;
using System.Text;
using System.Text.RegularExpressions;

namespace AxiomOffice.Ribbon
{
    // Ba nút "Soát" trên ribbon (docs/architecture/ribbon-ui.md, đợt 1): gọi thẳng lệnh soát chỉ đọc của bridge rồi
    // diễn giải kết quả cho người dùng. Không qua model, không cần Core: 0 token, kết quả tức thì.
    // Chi tiết trong kết quả lệnh viết cho model (tiếng Anh), nên ở đây dựng lại câu tiếng Việt theo loại lỗi.
    internal static class RibbonChecks
    {
        private const int MaxListed = 15;

        // tag nút -> (lệnh bridge, app hợp lệ, tiêu đề hộp kết quả)
        internal sealed class CheckInfo
        {
            public string Action;
            public string Kind;
            public string Title;
        }

        internal static readonly Dictionary<string, CheckInfo> Checks = new Dictionary<string, CheckInfo>(StringComparer.Ordinal)
        {
            { "checkLayout", new CheckInfo { Action = "wpp.checkLayout", Kind = "wpp", Title = "Soát bố cục" } },
            { "checkTables", new CheckInfo { Action = "writer.checkTables", Kind = "wps", Title = "Soát bảng" } },
            { "checkRange", new CheckInfo { Action = "et.checkRange", Kind = "et", Title = "Soát vùng dữ liệu" } }
        };

        private static readonly Dictionary<string, string> Labels = new Dictionary<string, string>(StringComparer.Ordinal)
        {
            { "overflow", "Chữ tràn khung" },
            { "offslide", "Ra ngoài slide" },
            { "small-font", "Chữ quá nhỏ" },
            { "overlap", "Hai khung chồng nhau" },
            { "dense", "Slide quá nhiều chữ" },
            { "outside-table", "Có dữ liệu lạc ra ngoài bảng" },
            { "empty-reference", "Công thức trỏ vào ô trống" },
            { "empty-header", "Cột có dữ liệu nhưng thiếu tiêu đề" },
            { "numbers-as-text", "Số đang lưu dạng chữ" },
            { "mixed-types", "Cột lẫn số và chữ" },
            { "error-values", "Ô đang báo lỗi" },
            { "no-number-format", "Số thập phân chưa định dạng" },
            { "empty-cells", "Bảng có ô trống" },
            { "long-header-cell", "Ô tiêu đề quá dài" }
        };

        private static readonly Regex ColumnRef = new Regex(@"\bcolumn ([A-Z]{1,3})\b");
        private static readonly Regex Count = new Regex(@"\b(?:has|uses) ([0-9]+(?:[.,][0-9]+)?)");

        // Kết quả Execute: {"ok":true,"result":{...}} hoặc {"ok":false,"error":"..."}.
        internal static string Describe(string tag, object response, out bool clean)
        {
            clean = false;
            var root = response as IDictionary<string, object>;
            if (root == null)
            {
                return Ui.T("Không đọc được kết quả soát.");
            }
            object ok;
            if (!root.TryGetValue("ok", out ok) || !(ok is bool) || !(bool)ok)
            {
                object error;
                root.TryGetValue("error", out error);
                return Ui.T("Không soát được: ") + Convert.ToString(error, CultureInfo.InvariantCulture);
            }
            object resultValue;
            root.TryGetValue("result", out resultValue);
            var result = resultValue as IDictionary<string, object>;
            if (result == null)
            {
                return Ui.T("Không đọc được kết quả soát.");
            }

            var lines = new List<string>();
            string scope;
            if (tag == "checkLayout")
            {
                scope = CollectLayout(result, lines);
            }
            else if (tag == "checkTables")
            {
                scope = Ui.T("Đã soát ") + ToInt(Get(result, "tableCount")) + Ui.T(" bảng.");
                foreach (IDictionary<string, object> issue in Items(Get(result, "issues")))
                {
                    object table = Get(issue, "table");
                    lines.Add((table != null ? Ui.T("Bảng ") + ToInt(table) + " · " : "") + IssueText(issue));
                }
            }
            else
            {
                scope = Ui.T("Đã soát vùng ") + Convert.ToString(Get(result, "table") ?? Get(result, "range"), CultureInfo.InvariantCulture)
                    + " (trang " + Convert.ToString(Get(result, "sheet"), CultureInfo.InvariantCulture) + ").";
                foreach (IDictionary<string, object> issue in Items(Get(result, "issues")))
                {
                    lines.Add(IssueText(issue));
                }
            }

            var sb = new StringBuilder();
            sb.AppendLine(scope);
            sb.AppendLine();
            if (lines.Count == 0)
            {
                clean = true;
                sb.AppendLine(Ui.T("Không thấy lỗi nào."));
            }
            else
            {
                sb.AppendLine(Ui.T("Có ") + lines.Count + Ui.T(" chỗ cần xem:"));
                for (int i = 0; i < lines.Count && i < MaxListed; i++)
                {
                    sb.AppendLine("• " + lines[i]);
                }
                if (lines.Count > MaxListed)
                {
                    sb.AppendLine(Ui.T("… và ") + (lines.Count - MaxListed) + Ui.T(" chỗ khác."));
                }
                sb.AppendLine();
                sb.AppendLine(Ui.T("Muốn AI sửa giúp: mở Ask AI và nhờ \"soát lại và sửa các lỗi trình bày\"."));
            }
            return sb.ToString().TrimEnd();
        }

        private static string CollectLayout(IDictionary<string, object> result, List<string> lines)
        {
            int slides = 0;
            foreach (IDictionary<string, object> slide in Items(Get(result, "slides")))
            {
                slides++;
                int index = ToInt(Get(slide, "index"));
                foreach (IDictionary<string, object> issue in Items(Get(slide, "issues")))
                {
                    lines.Add("Slide " + index + " · " + IssueText(issue));
                }
            }
            return Ui.T("Đã soát ") + slides + " slide.";
        }

        internal static string IssueText(IDictionary<string, object> issue)
        {
            string type = Convert.ToString(Get(issue, "type"), CultureInfo.InvariantCulture) ?? "";
            string detail = Convert.ToString(Get(issue, "detail"), CultureInfo.InvariantCulture) ?? "";
            string label;
            if (!Labels.TryGetValue(type, out label))
            {
                return detail;
            }
            if (type == "empty-reference")
            {
                // Chi tiết loại này vốn đã viết bằng tiếng Việt.
                return detail;
            }

            var where = new List<string>();
            var shapes = new List<string>();
            foreach (object shape in ItemsRaw(Get(issue, "shapes")))
            {
                shapes.Add("\"" + Convert.ToString(shape, CultureInfo.InvariantCulture) + "\"");
            }
            if (shapes.Count > 0)
            {
                where.Add(string.Join(Ui.T(" và "), shapes));
            }
            Match column = ColumnRef.Match(detail);
            if (column.Success)
            {
                where.Add(Ui.T("cột ") + column.Groups[1].Value);
            }
            Match count = Count.Match(detail);
            if (count.Success && (type == "numbers-as-text" || type == "error-values" || type == "empty-cells"))
            {
                where.Add(count.Groups[1].Value + Ui.T(" ô"));
            }
            else if (count.Success && type == "small-font")
            {
                where.Add(Ui.T("cỡ ") + count.Groups[1].Value + "pt");
            }
            label = Ui.T(label);
            return where.Count > 0 ? label + ": " + string.Join(", ", where) : label;
        }

        private static object Get(IDictionary<string, object> map, string key)
        {
            object value;
            return map != null && map.TryGetValue(key, out value) ? value : null;
        }

        private static IEnumerable<IDictionary<string, object>> Items(object list)
        {
            var items = list as IEnumerable;
            if (items == null || list is string)
            {
                yield break;
            }
            foreach (object item in items)
            {
                var map = item as IDictionary<string, object>;
                if (map != null)
                {
                    yield return map;
                }
            }
        }

        private static IEnumerable<object> ItemsRaw(object list)
        {
            var items = list as IEnumerable;
            if (items == null || list is string)
            {
                yield break;
            }
            foreach (object item in items)
            {
                yield return item;
            }
        }

        private static int ToInt(object value)
        {
            try
            {
                return Convert.ToInt32(value, CultureInfo.InvariantCulture);
            }
            catch (Exception)
            {
                return 0;
            }
        }
    }
}

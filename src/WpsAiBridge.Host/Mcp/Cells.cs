using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.RegularExpressions;

namespace WpsAiBridge.Host.Mcp
{
    // Tiện ích ô bảng tính: địa chỉ A1, chuyển giá trị JSON -> chữ như str() của Python.
    internal static class Cells
    {
        private static readonly Regex CellRef = new Regex(@"^\$?([A-Za-z]{1,3})\$?([0-9]+)$");
        private static readonly Regex ColRef = new Regex(@"^\$?([A-Za-z]{1,3})$");
        private static readonly Regex RowRef = new Regex(@"^\$?([0-9]+)$");

        public static string ToText(object value)
        {
            if (value == null)
            {
                return "";
            }
            if (value is bool)
            {
                return (bool)value ? "True" : "False";
            }
            if (value is double)
            {
                return ((double)value).ToString("R", CultureInfo.InvariantCulture);
            }
            if (value is float)
            {
                return ((float)value).ToString("R", CultureInfo.InvariantCulture);
            }
            var formattable = value as IFormattable;
            if (formattable != null)
            {
                return formattable.ToString(null, CultureInfo.InvariantCulture);
            }
            return value.ToString();
        }

        public static bool IsNumber(object value)
        {
            return value is int || value is long || value is double || value is decimal || value is float || value is short || value is byte;
        }

        public static int ColumnIndex(string letters)
        {
            int result = 0;
            foreach (char c in letters.ToUpperInvariant())
            {
                result = result * 26 + (c - 'A' + 1);
            }
            return result;
        }

        public static string ColumnLetters(int index)
        {
            var sb = new StringBuilder();
            while (index > 0)
            {
                int rem = (index - 1) % 26;
                sb.Insert(0, (char)('A' + rem));
                index = (index - 1) / 26;
            }
            return sb.ToString();
        }

        public static string Address(int row, int col)
        {
            return ColumnLetters(col) + row.ToString(CultureInfo.InvariantCulture);
        }

        public static void ParseCell(string a1, out int row, out int col)
        {
            Match match = CellRef.Match((a1 ?? "").Trim());
            if (!match.Success)
            {
                throw new ArgumentException("invalid cell reference '" + a1 + "' (expected like 'B2')");
            }
            col = ColumnIndex(match.Groups[1].Value);
            row = int.Parse(match.Groups[2].Value, CultureInfo.InvariantCulture);
            if (row < 1 || col < 1 || col > 16384 || row > 1048576)
            {
                throw new ArgumentException("cell reference out of range: " + a1);
            }
        }

        // Như openpyxl.range_boundaries: "A1:C5", "B2", "A:C" (không giới hạn dòng), "2:5" (không giới hạn cột).
        public static void ParseRange(string range, out int? minCol, out int? minRow, out int? maxCol, out int? maxRow)
        {
            string text = (range ?? "").Trim();
            int bang = text.LastIndexOf('!');
            if (bang >= 0)
            {
                text = text.Substring(bang + 1);
            }
            string[] parts = text.Split(':');
            if (parts.Length > 2 || parts[0].Length == 0)
            {
                throw new ArgumentException("invalid range '" + range + "'");
            }
            int? c1, r1, c2, r2;
            ParseEdge(parts[0], range, out c1, out r1);
            if (parts.Length == 2)
            {
                ParseEdge(parts[1], range, out c2, out r2);
            }
            else
            {
                c2 = c1;
                r2 = r1;
            }
            minCol = Min(c1, c2);
            maxCol = Max(c1, c2);
            minRow = Min(r1, r2);
            maxRow = Max(r1, r2);
        }

        public static void ParseBoundedRange(string range, out int minCol, out int minRow, out int maxCol, out int maxRow)
        {
            int? c1, r1, c2, r2;
            ParseRange(range, out c1, out r1, out c2, out r2);
            if (!c1.HasValue || !r1.HasValue || !c2.HasValue || !r2.HasValue)
            {
                throw new ArgumentException("range must have explicit cells like 'A1:D10': " + range);
            }
            minCol = c1.Value;
            minRow = r1.Value;
            maxCol = c2.Value;
            maxRow = r2.Value;
        }

        private static void ParseEdge(string edge, string range, out int? col, out int? row)
        {
            Match cell = CellRef.Match(edge);
            if (cell.Success)
            {
                col = ColumnIndex(cell.Groups[1].Value);
                row = int.Parse(cell.Groups[2].Value, CultureInfo.InvariantCulture);
                return;
            }
            Match colOnly = ColRef.Match(edge);
            if (colOnly.Success)
            {
                col = ColumnIndex(colOnly.Groups[1].Value);
                row = null;
                return;
            }
            Match rowOnly = RowRef.Match(edge);
            if (rowOnly.Success)
            {
                col = null;
                row = int.Parse(rowOnly.Groups[1].Value, CultureInfo.InvariantCulture);
                return;
            }
            throw new ArgumentException("invalid range '" + range + "'");
        }

        private static int? Min(int? a, int? b)
        {
            if (!a.HasValue)
            {
                return b;
            }
            if (!b.HasValue)
            {
                return a;
            }
            return Math.Min(a.Value, b.Value);
        }

        private static int? Max(int? a, int? b)
        {
            if (!a.HasValue)
            {
                return b;
            }
            if (!b.HasValue)
            {
                return a;
            }
            return Math.Max(a.Value, b.Value);
        }

        // Excel serial -> chuỗi ISO như datetime.isoformat() của Python.
        public static string SerialToIso(double serial, bool date1904)
        {
            if (serial < 1 && serial >= 0 && !date1904)
            {
                TimeSpan time = TimeSpan.FromSeconds(Math.Round(serial * 86400));
                return new DateTime(1, 1, 1).Add(time).ToString("HH:mm:ss", CultureInfo.InvariantCulture);
            }
            double adjusted = date1904 ? serial + 1462 : (serial < 60 ? serial + 1 : serial);
            DateTime value = DateTime.FromOADate(adjusted);
            value = new DateTime((long)(Math.Round(value.Ticks / (double)TimeSpan.TicksPerSecond) * TimeSpan.TicksPerSecond));
            return value.ToString("yyyy-MM-ddTHH:mm:ss", CultureInfo.InvariantCulture);
        }
    }

    // Đọc/ghi CSV/TSV như bản Python (csv.Sniffer + ép kiểu số, ghi UTF-8 có BOM).
    internal static class CsvTable
    {
        public static bool IsCsvExtension(string ext)
        {
            return ext == ".csv" || ext == ".tsv" || ext == ".txt";
        }

        public static List<List<object>> Read(string path)
        {
            string text = ReadText(path);
            char delimiter = Path.GetExtension(path).ToLowerInvariant() == ".tsv" ? '\t' : Sniff(text);
            var rows = new List<List<object>>();
            foreach (List<string> raw in Parse(text, delimiter))
            {
                rows.Add(raw.Select(Coerce).ToList());
            }
            return rows;
        }

        private static string ReadText(string path)
        {
            byte[] bytes = File.ReadAllBytes(path);
            int offset = bytes.Length >= 3 && bytes[0] == 0xEF && bytes[1] == 0xBB && bytes[2] == 0xBF ? 3 : 0;
            try
            {
                return new UTF8Encoding(false, true).GetString(bytes, offset, bytes.Length - offset);
            }
            catch (DecoderFallbackException)
            {
                return Encoding.Default.GetString(bytes);
            }
        }

        // Chọn dấu phân cách xuất hiện đều nhất trong các dòng đầu (",;\t|").
        private static char Sniff(string text)
        {
            char[] candidates = { ',', ';', '\t', '|' };
            string[] lines = text.Replace("\r\n", "\n").Split('\n').Where(l => l.Length > 0).Take(20).ToArray();
            char best = ',';
            int bestScore = 0;
            foreach (char candidate in candidates)
            {
                int[] counts = lines.Select(line => CountOutsideQuotes(line, candidate)).ToArray();
                if (counts.Length == 0 || counts.Max() == 0)
                {
                    continue;
                }
                int mode = counts.GroupBy(c => c).OrderByDescending(g => g.Count()).First().Key;
                int consistent = counts.Count(c => c == mode && c > 0);
                int score = consistent * 1000 + mode;
                if (score > bestScore)
                {
                    bestScore = score;
                    best = candidate;
                }
            }
            return best;
        }

        private static int CountOutsideQuotes(string line, char delimiter)
        {
            int count = 0;
            bool quoted = false;
            foreach (char c in line)
            {
                if (c == '"')
                {
                    quoted = !quoted;
                }
                else if (c == delimiter && !quoted)
                {
                    count++;
                }
            }
            return count;
        }

        private static IEnumerable<List<string>> Parse(string text, char delimiter)
        {
            var row = new List<string>();
            var field = new StringBuilder();
            bool quoted = false;
            bool any = false;
            for (int i = 0; i < text.Length; i++)
            {
                char c = text[i];
                any = true;
                if (quoted)
                {
                    if (c == '"')
                    {
                        if (i + 1 < text.Length && text[i + 1] == '"')
                        {
                            field.Append('"');
                            i++;
                        }
                        else
                        {
                            quoted = false;
                        }
                    }
                    else
                    {
                        field.Append(c);
                    }
                    continue;
                }
                if (c == '"' && field.Length == 0)
                {
                    quoted = true;
                }
                else if (c == delimiter)
                {
                    row.Add(field.ToString());
                    field.Clear();
                }
                else if (c == '\r' || c == '\n')
                {
                    if (c == '\r' && i + 1 < text.Length && text[i + 1] == '\n')
                    {
                        i++;
                    }
                    row.Add(field.ToString());
                    field.Clear();
                    yield return row;
                    row = new List<string>();
                    any = false;
                }
                else
                {
                    field.Append(c);
                }
            }
            if (any || field.Length > 0 || row.Count > 0)
            {
                row.Add(field.ToString());
                yield return row;
            }
        }

        private static object Coerce(string text)
        {
            string stripped = text.Trim();
            if (stripped.Length == 0)
            {
                return "";
            }
            long asLong;
            if (long.TryParse(stripped, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out asLong)
                && asLong.ToString(CultureInfo.InvariantCulture) == stripped)
            {
                return asLong;
            }
            double asDouble;
            if (double.TryParse(stripped, NumberStyles.Float, CultureInfo.InvariantCulture, out asDouble))
            {
                string repr = asDouble.ToString("R", CultureInfo.InvariantCulture);
                if (repr.IndexOfAny(new[] { '.', 'E', 'e' }) < 0)
                {
                    repr += ".0";
                }
                if (repr == stripped)
                {
                    return asDouble;
                }
            }
            return text;
        }

        public static void WriteAtomic(string path, IEnumerable<IList<object>> rows)
        {
            string temp = FileSafety.TempPathFor(path);
            try
            {
                using (var writer = new StreamWriter(temp, false, new UTF8Encoding(true)))
                {
                    foreach (IList<object> row in rows)
                    {
                        writer.Write(string.Join(",", row.Select(Quote)));
                        writer.Write("\r\n");
                    }
                }
            }
            catch (Exception)
            {
                FileSafety.DeleteQuietly(temp);
                throw;
            }
            FileSafety.Replace(temp, path);
        }

        private static string Quote(object value)
        {
            string text = value == null ? "" : Cells.ToText(value);
            if (text.IndexOfAny(new[] { ',', '"', '\r', '\n' }) >= 0)
            {
                return "\"" + text.Replace("\"", "\"\"") + "\"";
            }
            return text;
        }
    }
}

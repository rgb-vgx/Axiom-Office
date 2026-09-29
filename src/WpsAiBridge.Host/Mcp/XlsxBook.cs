using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.RegularExpressions;
using System.Xml;
using System.Xml.Linq;

namespace WpsAiBridge.Host.Mcp
{
    internal sealed class SheetRef
    {
        public string Name;
        public string RelId;
        public string Part;
        public int SheetId;
        public XElement Element;
    }

    // Lưới ô thưa: chỉ giữ các dòng trong vùng cần đọc; MaxRow/MaxCol là kích thước sheet.
    internal sealed class SheetGrid
    {
        public string Name;
        public int MaxRow;
        public int MaxCol;
        public bool HasCells;
        private readonly Dictionary<int, Dictionary<int, object>> _rows = new Dictionary<int, Dictionary<int, object>>();

        public object Get(int row, int col)
        {
            Dictionary<int, object> cells;
            object value;
            return _rows.TryGetValue(row, out cells) && cells.TryGetValue(col, out value) ? value : null;
        }

        public void Set(int row, int col, object value)
        {
            Dictionary<int, object> cells;
            if (!_rows.TryGetValue(row, out cells))
            {
                cells = new Dictionary<int, object>();
                _rows[row] = cells;
            }
            cells[col] = value;
            HasCells = true;
            MaxRow = Math.Max(MaxRow, row);
            MaxCol = Math.Max(MaxCol, col);
        }

        public List<object> Row(int row, int fromCol, int toCol)
        {
            var result = new List<object>();
            for (int c = fromCol; c <= toCol; c++)
            {
                result.Add(Get(row, c));
            }
            return result;
        }
    }

    // Workbook .xlsx/.xlsm: đọc streaming, sửa trực tiếp XML (giữ nguyên chart/ảnh/pivot/macro).
    internal sealed class XlsxBook
    {
        public const string WorksheetContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml";
        private const string SharedStringsContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml";
        private const string TableContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.table+xml";
        private static readonly XNamespace R = OoxmlPackage.OfficeRelNs;
        private static readonly XNamespace XmlNs = XNamespace.Xml;

        private static readonly int[] BuiltinDateFormats = { 14, 15, 16, 17, 18, 19, 20, 21, 22, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 45, 46, 47, 50, 51, 52, 53, 54, 55, 56, 57, 58 };

        public static readonly Dictionary<string, int> BuiltinFormats = new Dictionary<string, int>(StringComparer.Ordinal)
        {
            { "General", 0 }, { "0", 1 }, { "0.00", 2 }, { "#,##0", 3 }, { "#,##0.00", 4 }, { "0%", 9 }, { "0.00%", 10 }, { "0.00E+00", 11 },
            { "# ?/?", 12 }, { "# ??/??", 13 }, { "mm-dd-yy", 14 }, { "d-mmm-yy", 15 }, { "d-mmm", 16 }, { "mmm-yy", 17 }, { "h:mm AM/PM", 18 },
            { "h:mm:ss AM/PM", 19 }, { "h:mm", 20 }, { "h:mm:ss", 21 }, { "m/d/yy h:mm", 22 }, { "#,##0 ;(#,##0)", 37 }, { "#,##0 ;[Red](#,##0)", 38 },
            { "#,##0.00;(#,##0.00)", 39 }, { "#,##0.00;[Red](#,##0.00)", 40 }, { "mm:ss", 45 }, { "[h]:mm:ss", 46 }, { "mmss.0", 47 }, { "##0.0E+0", 48 }, { "@", 49 },
        };

        // Thứ tự phần tử con của <worksheet> theo schema (chèn phần tử mới đúng chỗ).
        public static readonly string[] WorksheetOrder =
        {
            "sheetPr", "dimension", "sheetViews", "sheetFormatPr", "cols", "sheetData", "sheetCalcPr", "sheetProtection", "protectedRanges",
            "scenarios", "autoFilter", "sortState", "dataConsolidate", "customSheetViews", "mergeCells", "phoneticPr", "conditionalFormatting",
            "dataValidations", "hyperlinks", "printOptions", "pageMargins", "pageSetup", "headerFooter", "rowBreaks", "colBreaks",
            "customProperties", "cellWatches", "ignoredErrors", "smartTags", "drawing", "legacyDrawing", "legacyDrawingHF", "drawingHF",
            "picture", "oleObjects", "controls", "webPublishItems", "tableParts", "extLst"
        };

        private static readonly string[] WorkbookOrder =
        {
            "fileVersion", "fileSharing", "workbookPr", "workbookProtection", "bookViews", "sheets", "functionGroups", "externalReferences",
            "definedNames", "calcPr", "oleSize", "customWorkbookViews", "pivotCaches", "smartTagPr", "smartTagTypes", "webPublishing",
            "fileRecoveryPr", "webPublishObjects", "extLst"
        };

        public readonly OoxmlPackage Package;
        public readonly string WorkbookPart;
        public readonly XNamespace S;
        public readonly List<SheetRef> Sheets = new List<SheetRef>();
        private readonly bool _date1904;
        private List<string> _shared;
        private Dictionary<string, int> _sharedIndex;
        private HashSet<int> _dateStyles;

        public XlsxBook(OoxmlPackage package)
        {
            Package = package;
            Rel main = package.RelOfType("", "officeDocument");
            if (main == null || !package.Exists(main.TargetPart))
            {
                throw new InvalidOperationException("not an Excel workbook (no workbook part)");
            }
            WorkbookPart = main.TargetPart;
            XElement root = Workbook.Root;
            S = root.Name.Namespace;
            XElement pr = root.Element(S + "workbookPr");
            _date1904 = pr != null && IsTrue((string)pr.Attribute("date1904"));
            LoadSheets();
        }

        public XDocument Workbook
        {
            get { return Package.Xml(WorkbookPart); }
        }

        private void LoadSheets()
        {
            Sheets.Clear();
            List<Rel> rels = Package.Rels(WorkbookPart);
            XElement sheets = Workbook.Root.Element(S + "sheets");
            if (sheets == null)
            {
                return;
            }
            foreach (XElement sheet in sheets.Elements(S + "sheet"))
            {
                string rid = (string)sheet.Attribute(R + "id");
                Rel rel = rels.FirstOrDefault(r => r.Id == rid);
                int sheetId;
                int.TryParse((string)sheet.Attribute("sheetId"), out sheetId);
                Sheets.Add(new SheetRef
                {
                    Name = (string)sheet.Attribute("name"),
                    RelId = rid,
                    Part = rel != null ? rel.TargetPart : null,
                    SheetId = sheetId,
                    Element = sheet
                });
            }
        }

        public List<string> SheetNames
        {
            get { return Sheets.Select(s => s.Name).ToList(); }
        }

        public SheetRef FindSheet(string name)
        {
            SheetRef sheet = Sheets.FirstOrDefault(s => s.Name == name);
            if (sheet == null)
            {
                throw new ArgumentException("sheet not found: " + name);
            }
            if (sheet.Part == null || !Package.Exists(sheet.Part))
            {
                throw new InvalidOperationException("sheet '" + name + "' is not a worksheet (chart sheet or missing part)");
            }
            return sheet;
        }

        public bool HasSheet(string name)
        {
            return Sheets.Any(s => string.Equals(s.Name, name, StringComparison.OrdinalIgnoreCase));
        }

        // Như wb.active của openpyxl: workbookView@activeTab.
        public SheetRef ActiveSheet()
        {
            int index = 0;
            XElement view = Workbook.Root.Element(S + "bookViews")?.Element(S + "workbookView");
            if (view != null)
            {
                int.TryParse((string)view.Attribute("activeTab") ?? "0", out index);
            }
            if (Sheets.Count == 0)
            {
                throw new InvalidOperationException("workbook has no sheets");
            }
            return FindSheet(Sheets[Math.Max(0, Math.Min(index, Sheets.Count - 1))].Name);
        }

        // ---- đọc ----

        // Đọc sheet: chỉ giữ ô trong [rowFrom..rowTo] x [colFrom..colTo]. Nếu có <dimension> hợp lệ thì
        // lấy kích thước từ đó và dừng sớm khi đã qua rowTo (như read-only mode của openpyxl).
        public SheetGrid Read(SheetRef sheet, bool formulas, int rowFrom = 1, int rowTo = int.MaxValue, int colFrom = 1, int colTo = int.MaxValue)
        {
            var grid = new SheetGrid { Name = sheet.Name };
            var shared = new Dictionary<string, KeyValuePair<string, int[]>>();
            bool dimensionKnown = false;
            using (Stream stream = Package.OpenStream(sheet.Part))
            using (XmlReader reader = XmlReader.Create(stream, new XmlReaderSettings { DtdProcessing = DtdProcessing.Prohibit, IgnoreComments = true }))
            {
                int row = 0;
                int col = 0;
                reader.MoveToContent();
                while (!reader.EOF)
                {
                    if (reader.NodeType != XmlNodeType.Element)
                    {
                        reader.Read();
                        continue;
                    }
                    string local = reader.LocalName;
                    if (local == "dimension")
                    {
                        int? c1, r1, c2, r2;
                        try
                        {
                            Cells.ParseRange(reader.GetAttribute("ref"), out c1, out r1, out c2, out r2);
                            if (c2.HasValue && r2.HasValue)
                            {
                                // Như openpyxl read-only: kích thước lấy từ <dimension>. Riêng "A1" (Excel ghi cho
                                // sheet trống, vài thư viện ghi sai) thì vẫn quét hết để không cắt mất dữ liệu.
                                grid.MaxRow = r2.Value;
                                grid.MaxCol = c2.Value;
                                dimensionKnown = c2.Value > 1 || r2.Value > 1;
                            }
                        }
                        catch (ArgumentException)
                        {
                        }
                        reader.Read();
                    }
                    else if (local == "row")
                    {
                        string r = reader.GetAttribute("r");
                        row = r != null ? int.Parse(r, CultureInfo.InvariantCulture) : row + 1;
                        col = 0;
                        if (dimensionKnown && row > rowTo)
                        {
                            break;
                        }
                        reader.Read();
                    }
                    else if (local == "c")
                    {
                        var cell = (XElement)XNode.ReadFrom(reader);
                        string reference = (string)cell.Attribute("r");
                        if (reference != null)
                        {
                            int parsedRow;
                            Cells.ParseCell(reference, out parsedRow, out col);
                        }
                        else
                        {
                            col++;
                        }
                        bool wanted = row >= rowFrom && row <= rowTo && col >= colFrom && col <= colTo;
                        object value = CellValue(cell, formulas, row, col, shared, wanted);
                        grid.HasCells = true;
                        grid.MaxRow = Math.Max(grid.MaxRow, row);
                        grid.MaxCol = Math.Max(grid.MaxCol, col);
                        if (wanted && value != null)
                        {
                            grid.Set(row, col, value);
                        }
                    }
                    else
                    {
                        reader.Read();
                    }
                }
            }
            return grid;
        }

        private object CellValue(XElement cell, bool formulas, int row, int col, Dictionary<string, KeyValuePair<string, int[]>> shared, bool wanted)
        {
            XElement f = Child(cell, "f");
            if (formulas && f != null)
            {
                string text = f.Value;
                string type = (string)f.Attribute("t");
                string si = (string)f.Attribute("si");
                if (type == "shared" && si != null)
                {
                    if (text.Length > 0)
                    {
                        shared[si] = new KeyValuePair<string, int[]>(text, new[] { row, col });
                    }
                    else
                    {
                        KeyValuePair<string, int[]> master;
                        if (shared.TryGetValue(si, out master))
                        {
                            text = FormulaShift.Shift(master.Key, row - master.Value[0], col - master.Value[1]);
                        }
                    }
                }
                if (text.Length > 0)
                {
                    return wanted ? "=" + text : null;
                }
            }
            if (!wanted)
            {
                return null;
            }
            string t = (string)cell.Attribute("t");
            XElement v = Child(cell, "v");
            switch (t)
            {
                case "s":
                    if (v == null)
                    {
                        return null;
                    }
                    int index = int.Parse(v.Value, CultureInfo.InvariantCulture);
                    List<string> strings = SharedStrings();
                    return index >= 0 && index < strings.Count ? strings[index] : null;
                case "inlineStr":
                    XElement inline = Child(cell, "is");
                    return inline == null ? null : RichText(inline);
                case "str":
                    return v == null ? "" : v.Value;
                case "b":
                    return v == null ? null : (object)(v.Value == "1" || v.Value == "true");
                case "e":
                case "d":
                    return v == null ? null : v.Value;
                default:
                    if (v == null || v.Value.Length == 0)
                    {
                        return null;
                    }
                    string raw = v.Value.Trim();
                    double number;
                    if (!double.TryParse(raw, NumberStyles.Float, CultureInfo.InvariantCulture, out number))
                    {
                        return raw;
                    }
                    int style;
                    if (int.TryParse((string)cell.Attribute("s") ?? "0", out style) && DateStyles().Contains(style))
                    {
                        return Cells.SerialToIso(number, _date1904);
                    }
                    long integer;
                    if (raw.IndexOfAny(new[] { '.', 'E', 'e' }) < 0 && long.TryParse(raw, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out integer))
                    {
                        return integer;
                    }
                    return double.IsNaN(number) || double.IsInfinity(number) ? null : (object)number;
            }
        }

        private static XElement Child(XElement parent, string localName)
        {
            return parent.Elements().FirstOrDefault(e => e.Name.LocalName == localName);
        }

        // Chuỗi rich text: nối các <t>, bỏ phần phiên âm <rPh>.
        private static string RichText(XElement container)
        {
            var sb = new StringBuilder();
            foreach (XElement t in container.Descendants().Where(e => e.Name.LocalName == "t"))
            {
                if (t.Ancestors().Any(a => a.Name.LocalName == "rPh"))
                {
                    continue;
                }
                sb.Append(t.Value);
            }
            return sb.ToString();
        }

        private string SharedStringsPart()
        {
            Rel rel = Package.RelOfType(WorkbookPart, "sharedStrings");
            return rel != null && Package.Exists(rel.TargetPart) ? rel.TargetPart : null;
        }

        private List<string> SharedStrings()
        {
            if (_shared != null)
            {
                return _shared;
            }
            _shared = new List<string>();
            string part = SharedStringsPart();
            if (part != null)
            {
                foreach (XElement si in Package.Xml(part).Root.Elements().Where(e => e.Name.LocalName == "si"))
                {
                    _shared.Add(RichText(si));
                }
            }
            return _shared;
        }

        private string StylesPart()
        {
            Rel rel = Package.RelOfType(WorkbookPart, "styles");
            return rel != null && Package.Exists(rel.TargetPart) ? rel.TargetPart : null;
        }

        private HashSet<int> DateStyles()
        {
            if (_dateStyles != null)
            {
                return _dateStyles;
            }
            _dateStyles = new HashSet<int>();
            string part = StylesPart();
            if (part == null)
            {
                return _dateStyles;
            }
            XElement root = Package.Xml(part).Root;
            var custom = new Dictionary<int, string>();
            XElement numFmts = root.Element(S + "numFmts");
            if (numFmts != null)
            {
                foreach (XElement fmt in numFmts.Elements(S + "numFmt"))
                {
                    int id;
                    if (int.TryParse((string)fmt.Attribute("numFmtId"), out id))
                    {
                        custom[id] = (string)fmt.Attribute("formatCode") ?? "";
                    }
                }
            }
            XElement cellXfs = root.Element(S + "cellXfs");
            if (cellXfs == null)
            {
                return _dateStyles;
            }
            int index = 0;
            foreach (XElement xf in cellXfs.Elements(S + "xf"))
            {
                int fmtId;
                int.TryParse((string)xf.Attribute("numFmtId") ?? "0", out fmtId);
                string code;
                if (custom.TryGetValue(fmtId, out code) ? IsDateFormat(code) : Array.IndexOf(BuiltinDateFormats, fmtId) >= 0)
                {
                    _dateStyles.Add(index);
                }
                index++;
            }
            return _dateStyles;
        }

        // Như is_date_format của openpyxl: bỏ chuỗi "...", [..] và ký tự escape rồi tìm d/m/y/h/s.
        public static bool IsDateFormat(string code)
        {
            if (string.IsNullOrEmpty(code))
            {
                return false;
            }
            string first = code.Split(';')[0];
            string stripped = Regex.Replace(first, "\"[^\"]*\"|\\[[^\\]]*\\]|\\\\.|_.|\\*.", "");
            if (stripped.Equals("General", StringComparison.OrdinalIgnoreCase))
            {
                return false;
            }
            return Regex.IsMatch(stripped, "[dmyhsDMYHS]");
        }

        private static bool IsTrue(string value)
        {
            return value == "1" || value == "true";
        }

        // ---- sửa ----

        public XDocument SheetDoc(SheetRef sheet)
        {
            return Package.Xml(sheet.Part);
        }

        public XElement SheetData(XDocument doc)
        {
            XElement data = doc.Root.Element(S + "sheetData");
            if (data == null)
            {
                data = new XElement(S + "sheetData");
                InsertInOrder(doc.Root, data, WorksheetOrder);
            }
            return data;
        }

        // Ghi giá trị như openpyxl: chuỗi bắt đầu '=' là công thức, null xoá giá trị (giữ style).
        public void SetValue(XElement sheetData, int row, int col, object value)
        {
            XElement cell = GetOrCreateCell(sheetData, row, col);
            foreach (XElement child in cell.Elements().Where(e => e.Name.LocalName == "f" || e.Name.LocalName == "v" || e.Name.LocalName == "is").ToList())
            {
                child.Remove();
            }
            cell.SetAttributeValue("t", null);
            cell.SetAttributeValue("vm", null);
            cell.SetAttributeValue("cm", null);
            if (value == null)
            {
                return;
            }
            var text = value as string;
            if (text != null)
            {
                if (text.StartsWith("=", StringComparison.Ordinal) && text.Length > 1)
                {
                    cell.Add(new XElement(S + "f", text.Substring(1)));
                }
                else
                {
                    cell.SetAttributeValue("t", "s");
                    cell.Add(new XElement(S + "v", SharedStringIndex(text)));
                }
                return;
            }
            if (value is bool)
            {
                cell.SetAttributeValue("t", "b");
                cell.Add(new XElement(S + "v", (bool)value ? "1" : "0"));
                return;
            }
            if (Cells.IsNumber(value))
            {
                double check = Convert.ToDouble(value, CultureInfo.InvariantCulture);
                if (double.IsNaN(check) || double.IsInfinity(check))
                {
                    return;
                }
                cell.Add(new XElement(S + "v", Cells.ToText(value)));
                return;
            }
            throw new ArgumentException("unsupported cell value at " + Cells.Address(row, col) + ": use string, number, boolean or null");
        }

        public XElement GetOrCreateCell(XElement sheetData, int row, int col)
        {
            XElement rowElement = GetOrCreateRow(sheetData, row);
            XElement before = null;
            int expected = 0;
            foreach (XElement c in rowElement.Elements(S + "c"))
            {
                int cRow, cCol;
                string reference = (string)c.Attribute("r");
                if (reference == null)
                {
                    cCol = expected + 1;
                    c.SetAttributeValue("r", Cells.Address(row, cCol));
                }
                else
                {
                    Cells.ParseCell(reference, out cRow, out cCol);
                }
                expected = cCol;
                if (cCol == col)
                {
                    return c;
                }
                if (cCol > col)
                {
                    before = c;
                    break;
                }
            }
            var cell = new XElement(S + "c", new XAttribute("r", Cells.Address(row, col)));
            int style = RowDefaultStyle(rowElement);
            if (style > 0)
            {
                cell.SetAttributeValue("s", style);
            }
            if (before != null)
            {
                before.AddBeforeSelf(cell);
            }
            else
            {
                XElement ext = rowElement.Element(S + "extLst");
                if (ext != null)
                {
                    ext.AddBeforeSelf(cell);
                }
                else
                {
                    rowElement.Add(cell);
                }
            }
            return cell;
        }

        private static int RowDefaultStyle(XElement row)
        {
            int style;
            return IsTrue((string)row.Attribute("customFormat")) && int.TryParse((string)row.Attribute("s"), out style) ? style : 0;
        }

        private XElement GetOrCreateRow(XElement sheetData, int row)
        {
            int expected = 0;
            foreach (XElement r in sheetData.Elements(S + "row"))
            {
                int index;
                string attr = (string)r.Attribute("r");
                if (attr == null || !int.TryParse(attr, out index))
                {
                    index = expected + 1;
                    r.SetAttributeValue("r", index);
                }
                expected = index;
                if (index == row)
                {
                    r.SetAttributeValue("spans", null);
                    return r;
                }
                if (index > row)
                {
                    var created = new XElement(S + "row", new XAttribute("r", row));
                    r.AddBeforeSelf(created);
                    return created;
                }
            }
            var appended = new XElement(S + "row", new XAttribute("r", row));
            sheetData.Add(appended);
            return appended;
        }

        public string CellText(XElement sheetData, int row, int col)
        {
            XElement cell = GetOrCreateCell(sheetData, row, col);
            string t = (string)cell.Attribute("t");
            XElement v = cell.Element(S + "v");
            if (t == "s" && v != null)
            {
                int index = int.Parse(v.Value, CultureInfo.InvariantCulture);
                List<string> strings = SharedStrings();
                return index < strings.Count ? strings[index] : "";
            }
            if (t == "inlineStr")
            {
                XElement inline = cell.Element(S + "is");
                return inline == null ? "" : RichText(inline);
            }
            return v != null ? v.Value : "";
        }

        public int SharedStringIndex(string text)
        {
            List<string> strings = SharedStrings();
            if (_sharedIndex == null)
            {
                _sharedIndex = new Dictionary<string, int>(StringComparer.Ordinal);
                for (int i = 0; i < strings.Count; i++)
                {
                    if (!_sharedIndex.ContainsKey(strings[i]))
                    {
                        _sharedIndex[strings[i]] = i;
                    }
                }
            }
            int existing;
            if (_sharedIndex.TryGetValue(text, out existing))
            {
                return existing;
            }
            string part = SharedStringsPart();
            XDocument doc;
            if (part == null)
            {
                part = "xl/sharedStrings.xml";
                doc = new XDocument(new XDeclaration("1.0", "UTF-8", "yes"), new XElement(S + "sst"));
                Package.AddRel(WorkbookPart, "sharedStrings", part);
                Package.SetContentTypeOverride(part, SharedStringsContentType);
            }
            else
            {
                doc = Package.Xml(part);
            }
            var t = new XElement(S + "t", text);
            if (text.Length > 0 && (char.IsWhiteSpace(text[0]) || char.IsWhiteSpace(text[text.Length - 1])))
            {
                t.Add(new XAttribute(XmlNs + "space", "preserve"));
            }
            doc.Root.Add(new XElement(S + "si", t));
            strings.Add(text);
            int index = strings.Count - 1;
            _sharedIndex[text] = index;
            doc.Root.SetAttributeValue("uniqueCount", strings.Count);
            int count;
            int.TryParse((string)doc.Root.Attribute("count") ?? "0", out count);
            doc.Root.SetAttributeValue("count", Math.Max(count + 1, strings.Count));
            Package.Put(part, doc);
            return index;
        }

        // Cập nhật <dimension> theo các ô thực có.
        public void UpdateDimension(XDocument doc)
        {
            XElement data = SheetData(doc);
            int minRow = int.MaxValue, minCol = int.MaxValue, maxRow = 0, maxCol = 0;
            foreach (XElement row in data.Elements(S + "row"))
            {
                foreach (XElement cell in row.Elements(S + "c"))
                {
                    string reference = (string)cell.Attribute("r");
                    if (reference == null)
                    {
                        continue;
                    }
                    int r, c;
                    Cells.ParseCell(reference, out r, out c);
                    minRow = Math.Min(minRow, r);
                    minCol = Math.Min(minCol, c);
                    maxRow = Math.Max(maxRow, r);
                    maxCol = Math.Max(maxCol, c);
                }
            }
            string reference2 = maxRow == 0 ? "A1" : (minRow == maxRow && minCol == maxCol
                ? Cells.Address(minRow, minCol)
                : Cells.Address(minRow, minCol) + ":" + Cells.Address(maxRow, maxCol));
            XElement dimension = doc.Root.Element(S + "dimension");
            if (dimension == null)
            {
                dimension = new XElement(S + "dimension");
                InsertInOrder(doc.Root, dimension, WorksheetOrder);
            }
            dimension.SetAttributeValue("ref", reference2);
        }

        // Công thức/giá trị đổi: bỏ calcChain (Excel tự dựng lại) và yêu cầu tính lại khi mở.
        public void InvalidateCalculation()
        {
            Rel calcChain = Package.RelOfType(WorkbookPart, "calcChain");
            if (calcChain != null)
            {
                Package.RemoveRel(WorkbookPart, calcChain.Id);
                if (Package.Exists(calcChain.TargetPart))
                {
                    Package.Delete(calcChain.TargetPart);
                }
            }
            XDocument workbook = Workbook;
            XElement calcPr = workbook.Root.Element(S + "calcPr");
            if (calcPr == null)
            {
                calcPr = new XElement(S + "calcPr");
                InsertInOrder(workbook.Root, calcPr, WorkbookOrder);
            }
            calcPr.SetAttributeValue("fullCalcOnLoad", "1");
            Package.Put(WorkbookPart, workbook);
        }

        public void SaveSheet(SheetRef sheet, XDocument doc)
        {
            Package.Put(sheet.Part, doc);
        }

        public static void InsertInOrder(XElement parent, XElement element, string[] order)
        {
            int rank = Array.IndexOf(order, element.Name.LocalName);
            foreach (XElement child in parent.Elements())
            {
                int childRank = Array.IndexOf(order, child.Name.LocalName);
                if (childRank > rank)
                {
                    child.AddBeforeSelf(element);
                    return;
                }
            }
            parent.Add(element);
        }

        // ---- sheet ----

        public static void ValidateSheetName(string name)
        {
            if (string.IsNullOrEmpty(name) || name.Length > 31 || name.IndexOfAny(new[] { ':', '\\', '/', '?', '*', '[', ']' }) >= 0
                || name.StartsWith("'", StringComparison.Ordinal) || name.EndsWith("'", StringComparison.Ordinal))
            {
                throw new ArgumentException("invalid sheet name '" + name + "' (1-31 chars, no : \\ / ? * [ ] and no leading/trailing apostrophe)");
            }
        }

        public SheetRef AddSheet(string name, XDocument content = null)
        {
            ValidateSheetName(name);
            if (HasSheet(name))
            {
                throw new ArgumentException("sheet already exists: " + name);
            }
            string part = Package.NextPartName("xl/worksheets/sheet{0}.xml");
            XDocument doc = content ?? new XDocument(new XDeclaration("1.0", "UTF-8", "yes"),
                new XElement(S + "worksheet", new XAttribute(XNamespace.Xmlns + "r", R.NamespaceName), new XElement(S + "sheetData")));
            Package.Put(part, doc);
            Package.SetContentTypeOverride(part, WorksheetContentType);
            string rid = Package.AddRel(WorkbookPart, "worksheet", part);
            XDocument workbook = Workbook;
            XElement sheets = workbook.Root.Element(S + "sheets");
            if (sheets == null)
            {
                sheets = new XElement(S + "sheets");
                InsertInOrder(workbook.Root, sheets, WorkbookOrder);
            }
            int sheetId = Sheets.Select(s => s.SheetId).DefaultIfEmpty(0).Max() + 1;
            sheets.Add(new XElement(S + "sheet", new XAttribute("name", name), new XAttribute("sheetId", sheetId), new XAttribute(R + "id", rid)));
            Package.Put(WorkbookPart, workbook);
            LoadSheets();
            return FindSheet(name);
        }

        public void DeleteSheet(SheetRef sheet)
        {
            int index = Sheets.IndexOf(sheet);
            XDocument workbook = Workbook;
            sheet.Element.Remove();
            Package.RemoveRel(WorkbookPart, sheet.RelId);
            if (sheet.Part != null && Package.Exists(sheet.Part))
            {
                Package.Delete(sheet.Part);
            }
            // Tên định nghĩa cục bộ của sheet bị xoá; chỉ số localSheetId phía sau lùi 1.
            XElement names = workbook.Root.Element(S + "definedNames");
            if (names != null)
            {
                foreach (XElement definedName in names.Elements(S + "definedName").ToList())
                {
                    int local;
                    if (int.TryParse((string)definedName.Attribute("localSheetId"), out local))
                    {
                        if (local == index)
                        {
                            definedName.Remove();
                        }
                        else if (local > index)
                        {
                            definedName.SetAttributeValue("localSheetId", local - 1);
                        }
                    }
                }
                if (!names.Elements().Any())
                {
                    names.Remove();
                }
            }
            int remaining = Sheets.Count - 1;
            foreach (XElement view in workbook.Root.Element(S + "bookViews")?.Elements(S + "workbookView") ?? Enumerable.Empty<XElement>())
            {
                foreach (string attribute in new[] { "activeTab", "firstSheet" })
                {
                    int value;
                    if (int.TryParse((string)view.Attribute(attribute), out value) && value >= remaining)
                    {
                        view.SetAttributeValue(attribute, Math.Max(0, remaining - 1));
                    }
                }
            }
            Package.Put(WorkbookPart, workbook);
            LoadSheets();
        }

        public void RenameSheet(SheetRef sheet, string newName)
        {
            ValidateSheetName(newName);
            if (Sheets.Any(s => s != sheet && string.Equals(s.Name, newName, StringComparison.OrdinalIgnoreCase)))
            {
                throw new ArgumentException("sheet already exists: " + newName);
            }
            XDocument workbook = Workbook;
            string oldName = sheet.Name;
            sheet.Element.SetAttributeValue("name", newName);
            XElement names = workbook.Root.Element(S + "definedNames");
            if (names != null)
            {
                foreach (XElement definedName in names.Elements(S + "definedName"))
                {
                    definedName.Value = ReplaceSheetReference(definedName.Value, oldName, newName);
                }
            }
            Package.Put(WorkbookPart, workbook);
            LoadSheets();
        }

        public static string QuoteSheet(string name)
        {
            return Regex.IsMatch(name, "^[A-Za-z_][A-Za-z0-9_.]*$") ? name : "'" + name.Replace("'", "''") + "'";
        }

        private static string ReplaceSheetReference(string formula, string oldName, string newName)
        {
            string quotedOld = "'" + oldName.Replace("'", "''") + "'!";
            string result = formula.Replace(quotedOld, QuoteSheet(newName) + "!");
            return Regex.Replace(result, "(?<![A-Za-z0-9_.'])" + Regex.Escape(oldName) + "!", QuoteSheet(newName) + "!");
        }

        // Như copy_worksheet của openpyxl: sao chép ô/style/định dạng; bỏ drawing/table/comment/ảnh.
        public SheetRef CopySheet(SheetRef source, string newName)
        {
            XElement copy = new XElement(SheetDoc(source).Root);
            string[] dropped = { "drawing", "legacyDrawing", "legacyDrawingHF", "drawingHF", "picture", "oleObjects", "controls", "tableParts", "webPublishItems" };
            foreach (XElement element in copy.Elements().Where(e => dropped.Contains(e.Name.LocalName)).ToList())
            {
                element.Remove();
            }
            XElement hyperlinks = copy.Element(S + "hyperlinks");
            if (hyperlinks != null)
            {
                hyperlinks.Elements().Where(h => h.Attribute(R + "id") != null).Remove();
                if (!hyperlinks.Elements().Any())
                {
                    hyperlinks.Remove();
                }
            }
            foreach (XElement view in copy.Descendants(S + "sheetView"))
            {
                view.SetAttributeValue("tabSelected", null);
            }
            // codeName của VBA phải duy nhất trong .xlsm; để Excel tự đặt cho sheet mới.
            XElement sheetPr = copy.Element(S + "sheetPr");
            if (sheetPr != null)
            {
                sheetPr.SetAttributeValue("codeName", null);
            }
            return AddSheet(newName, new XDocument(new XDeclaration("1.0", "UTF-8", "yes"), copy));
        }

        // ---- table ----

        public List<string> AllTableNames()
        {
            var names = new List<string>();
            foreach (string part in Package.PartNames.Where(p => p.StartsWith("xl/tables/", StringComparison.OrdinalIgnoreCase) && p.EndsWith(".xml", StringComparison.OrdinalIgnoreCase)))
            {
                XElement root = Package.Xml(part).Root;
                names.Add((string)root.Attribute("displayName") ?? (string)root.Attribute("name"));
            }
            return names;
        }

        public List<string> SheetTableNames(SheetRef sheet)
        {
            var names = new List<string>();
            foreach (Rel rel in Package.Rels(sheet.Part).Where(r => r.Type.EndsWith("/table", StringComparison.Ordinal)))
            {
                if (Package.Exists(rel.TargetPart))
                {
                    XElement root = Package.Xml(rel.TargetPart).Root;
                    names.Add((string)root.Attribute("displayName") ?? (string)root.Attribute("name"));
                }
            }
            return names;
        }

        public void AddTable(SheetRef sheet, string range, string name)
        {
            int minCol, minRow, maxCol, maxRow;
            Cells.ParseBoundedRange(range, out minCol, out minRow, out maxCol, out maxRow);
            if (maxRow <= minRow)
            {
                throw new ArgumentException("table range needs a header row and at least one data row");
            }
            XDocument doc = SheetDoc(sheet);
            XElement data = SheetData(doc);
            // Excel yêu cầu ô tiêu đề là chữ, không trùng, khớp tên cột của bảng.
            var columns = new List<string>();
            var used = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
            for (int c = minCol; c <= maxCol; c++)
            {
                string header = CellText(data, minRow, c).Trim();
                if (header.Length == 0)
                {
                    header = "Column" + (c - minCol + 1);
                }
                string unique = header;
                for (int n = 2; used.Contains(unique); n++)
                {
                    unique = header + n;
                }
                used.Add(unique);
                columns.Add(unique);
                SetValue(data, minRow, c, unique);
            }
            UpdateDimension(doc);

            int tableId = 1;
            foreach (string part in Package.PartNames.Where(p => p.StartsWith("xl/tables/", StringComparison.OrdinalIgnoreCase) && p.EndsWith(".xml", StringComparison.OrdinalIgnoreCase)))
            {
                int id;
                if (int.TryParse((string)Package.Xml(part).Root.Attribute("id"), out id))
                {
                    tableId = Math.Max(tableId, id + 1);
                }
            }
            string reference = Cells.Address(minRow, minCol) + ":" + Cells.Address(maxRow, maxCol);
            var tableColumns = new XElement(S + "tableColumns", new XAttribute("count", columns.Count));
            for (int i = 0; i < columns.Count; i++)
            {
                tableColumns.Add(new XElement(S + "tableColumn", new XAttribute("id", i + 1), new XAttribute("name", columns[i])));
            }
            var table = new XDocument(new XDeclaration("1.0", "UTF-8", "yes"),
                new XElement(S + "table",
                    new XAttribute("id", tableId), new XAttribute("name", name), new XAttribute("displayName", name),
                    new XAttribute("ref", reference), new XAttribute("totalsRowShown", 0),
                    new XElement(S + "autoFilter", new XAttribute("ref", reference)),
                    tableColumns,
                    new XElement(S + "tableStyleInfo", new XAttribute("name", "TableStyleMedium9"), new XAttribute("showFirstColumn", 0),
                        new XAttribute("showLastColumn", 0), new XAttribute("showRowStripes", 1), new XAttribute("showColumnStripes", 0))));
            string tablePart = Package.NextPartName("xl/tables/table{0}.xml");
            Package.Put(tablePart, table);
            Package.SetContentTypeOverride(tablePart, TableContentType);
            string rid = Package.AddRel(sheet.Part, "table", tablePart);

            XElement tableParts = doc.Root.Element(S + "tableParts");
            if (tableParts == null)
            {
                tableParts = new XElement(S + "tableParts");
                InsertInOrder(doc.Root, tableParts, WorksheetOrder);
            }
            tableParts.Add(new XElement(S + "tablePart", new XAttribute(R + "id", rid)));
            tableParts.SetAttributeValue("count", tableParts.Elements(S + "tablePart").Count());
            if (doc.Root.Attribute(XNamespace.Xmlns + "r") == null && doc.Root.GetPrefixOfNamespace(R) == null)
            {
                doc.Root.Add(new XAttribute(XNamespace.Xmlns + "r", R.NamespaceName));
            }
            SaveSheet(sheet, doc);
        }

        // ---- style ----

        public XlsxStyles Styles()
        {
            string part = StylesPart();
            if (part == null)
            {
                part = "xl/styles.xml";
                Package.Put(part, XlsxTemplate.StylesXml(S));
                Package.AddRel(WorkbookPart, "styles", part);
                Package.SetContentTypeOverride(part, "application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml");
            }
            return new XlsxStyles(Package, part, S);
        }
    }

    // Dời tham chiếu tương đối của shared formula (ô con của công thức dùng chung).
    internal static class FormulaShift
    {
        private static readonly Regex Reference = new Regex(@"(?<![A-Za-z0-9_\.])(\$?)([A-Za-z]{1,3})(\$?)([0-9]{1,7})(?![0-9A-Za-z_\(])");

        public static string Shift(string formula, int dRow, int dCol)
        {
            var sb = new StringBuilder();
            int start = 0;
            bool inString = false;
            for (int i = 0; i <= formula.Length; i++)
            {
                bool end = i == formula.Length;
                if (!end && formula[i] == '"')
                {
                    if (!inString)
                    {
                        sb.Append(ShiftSegment(formula.Substring(start, i - start), dRow, dCol));
                        start = i;
                    }
                    else
                    {
                        sb.Append(formula.Substring(start, i - start + 1));
                        start = i + 1;
                    }
                    inString = !inString;
                }
                else if (end)
                {
                    string tail = formula.Substring(start);
                    sb.Append(inString ? tail : ShiftSegment(tail, dRow, dCol));
                }
            }
            return sb.ToString();
        }

        private static string ShiftSegment(string segment, int dRow, int dCol)
        {
            return Reference.Replace(segment, m =>
            {
                int col = Cells.ColumnIndex(m.Groups[2].Value);
                int row = int.Parse(m.Groups[4].Value, CultureInfo.InvariantCulture);
                if (m.Groups[1].Value.Length == 0)
                {
                    col += dCol;
                }
                if (m.Groups[3].Value.Length == 0)
                {
                    row += dRow;
                }
                if (col < 1 || row < 1)
                {
                    return "#REF!";
                }
                return m.Groups[1].Value + Cells.ColumnLetters(col) + m.Groups[3].Value + row.ToString(CultureInfo.InvariantCulture);
            });
        }
    }
}

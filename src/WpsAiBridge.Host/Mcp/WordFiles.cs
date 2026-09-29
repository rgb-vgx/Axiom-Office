using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Text.RegularExpressions;
using System.Xml.Linq;

namespace WpsAiBridge.Host.Mcp
{
    // Làn file cho .docx (thay python-docx). Chỉ paragraph/table cấp body như python-docx.
    internal static class WordFiles
    {
        private static readonly XNamespace W = "http://schemas.openxmlformats.org/wordprocessingml/2006/main";
        private static readonly XNamespace Cp = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties";
        private static readonly XNamespace Dc = "http://purl.org/dc/elements/1.1/";
        private static readonly XNamespace DcTerms = "http://purl.org/dc/terms/";
        private static readonly XNamespace Xsi = "http://www.w3.org/2001/XMLSchema-instance";
        private static readonly XNamespace XmlNs = XNamespace.Xml;

        public static IEnumerable<McpTool> Tools()
        {
            yield return new McpTool("doc_profile",
                "Inspect a .docx file: paragraph/table/section counts, style usage, core properties.",
                new[] { Param.Str("path", null, true) },
                a => Profile(a.Req("path")));
            yield return new McpTool("doc_get_text",
                "Extract all text from a .docx file (paragraphs + tables), optionally truncated.",
                new[] { Param.Str("path", null, true), Param.Int("max_chars", null, false, 0), Param.Bool("include_tables", null, false, true) },
                a => GetText(a.Req("path"), a.Int("max_chars", 0), a.Bool("include_tables", true)));
            yield return new McpTool("doc_find_text",
                "Find paragraphs containing a query string in a .docx file.",
                new[] { Param.Str("path", null, true), Param.Str("query", null, true), Param.Int("max_results", null, false, 50) },
                a => FindText(a.Req("path"), a.Str("query", ""), a.Int("max_results", 50)));
            yield return new McpTool("doc_extract_table",
                "Extract one table (by index) from a .docx file as a 2D array.",
                new[] { Param.Str("path", null, true), Param.Int("index", null, false, 0) },
                a => ExtractTable(a.Req("path"), a.Int("index", 0)));
            yield return new McpTool("doc_create",
                "Create a .docx file. paragraphs items: \"text\" | {\"text\": \"...\", \"style\": \"Heading 1\"} | {\"table\": [[...]]}. Atomic save.",
                new[] { Param.Str("path", null, true), Param.Arr("paragraphs"), Param.Str("title"), Param.Str("author"), Param.Bool("overwrite", null, false, false) },
                a => Create(a.Req("path"), a.Arr("paragraphs"), a.Str("title"), a.Str("author"), a.Bool("overwrite", false)));
        }

        // ---- đọc ----

        private sealed class DocxModel
        {
            public List<XElement> Paragraphs = new List<XElement>();
            public List<XElement> Tables = new List<XElement>();
            public int Sections;
            public Dictionary<string, string> StyleNames = new Dictionary<string, string>(StringComparer.Ordinal);
            public string DefaultParagraphStyle = "Normal";
            public XDocument Core;
        }

        private static DocxModel Load(string path)
        {
            FileSafety.RequireFile(path);
            using (OoxmlPackage package = OoxmlPackage.OpenRead(path))
            {
                string documentPart = MainPart(package);
                XElement body = package.Xml(documentPart).Root.Element(W + "body");
                var model = new DocxModel();
                if (body != null)
                {
                    model.Paragraphs = body.Elements(W + "p").ToList();
                    model.Tables = body.Elements(W + "tbl").ToList();
                    model.Sections = body.Elements(W + "sectPr").Count()
                        + body.Elements(W + "p").Count(p => p.Element(W + "pPr") != null && p.Element(W + "pPr").Element(W + "sectPr") != null);
                }
                Rel styles = package.RelOfType(documentPart, "styles");
                if (styles != null && package.Exists(styles.TargetPart))
                {
                    foreach (XElement style in package.Xml(styles.TargetPart).Root.Elements(W + "style"))
                    {
                        string id = (string)style.Attribute(W + "styleId");
                        XElement name = style.Element(W + "name");
                        if (id != null)
                        {
                            model.StyleNames[id] = UiName(name != null ? (string)name.Attribute(W + "val") : id);
                        }
                        if ((string)style.Attribute(W + "type") == "paragraph" && IsOn(style.Attribute(W + "default")) && id != null)
                        {
                            model.DefaultParagraphStyle = model.StyleNames[id];
                        }
                    }
                }
                Rel core = package.RelOfType("", "metadata/core-properties");
                if (core != null && package.Exists(core.TargetPart))
                {
                    model.Core = package.Xml(core.TargetPart);
                }
                return model;
            }
        }

        private static string MainPart(OoxmlPackage package)
        {
            Rel main = package.RelOfType("", "officeDocument");
            if (main == null || !package.Exists(main.TargetPart))
            {
                throw new InvalidOperationException("not a Word document (no main document part)");
            }
            return main.TargetPart;
        }

        // Tên style nội bộ -> tên hiển thị như python-docx ("heading 1" -> "Heading 1").
        private static string UiName(string name)
        {
            if (name == null)
            {
                return "None";
            }
            Match heading = Regex.Match(name, "^heading ([1-9])$");
            if (heading.Success)
            {
                return "Heading " + heading.Groups[1].Value;
            }
            switch (name)
            {
                case "caption": return "Caption";
                case "header": return "Header";
                case "footer": return "Footer";
                default: return name;
            }
        }

        private static bool IsOn(XAttribute attribute)
        {
            if (attribute == null)
            {
                return false;
            }
            string value = attribute.Value;
            return value == "1" || value == "true" || value == "on";
        }

        private static string StyleOf(DocxModel model, XElement paragraph)
        {
            XElement pPr = paragraph.Element(W + "pPr");
            XElement pStyle = pPr != null ? pPr.Element(W + "pStyle") : null;
            string id = pStyle != null ? (string)pStyle.Attribute(W + "val") : null;
            if (id == null)
            {
                return model.DefaultParagraphStyle;
            }
            string name;
            return model.StyleNames.TryGetValue(id, out name) ? name : id;
        }

        // Như paragraph.text của python-docx: run trực tiếp + run trong hyperlink/ins/smartTag/fldSimple.
        private static string ParagraphText(XElement paragraph)
        {
            var sb = new StringBuilder();
            AppendRuns(paragraph, sb);
            return sb.ToString();
        }

        private static void AppendRuns(XElement container, StringBuilder sb)
        {
            foreach (XElement child in container.Elements())
            {
                string local = child.Name.LocalName;
                if (child.Name.Namespace != W)
                {
                    continue;
                }
                if (local == "r")
                {
                    foreach (XElement item in child.Elements())
                    {
                        if (item.Name == W + "t")
                        {
                            sb.Append(item.Value);
                        }
                        else if (item.Name == W + "tab")
                        {
                            sb.Append('\t');
                        }
                        else if (item.Name == W + "br" || item.Name == W + "cr")
                        {
                            sb.Append('\n');
                        }
                        else if (item.Name == W + "noBreakHyphen")
                        {
                            sb.Append('-');
                        }
                    }
                }
                else if (local == "hyperlink" || local == "ins" || local == "smartTag" || local == "fldSimple" || local == "customXml" || local == "sdt" || local == "sdtContent")
                {
                    AppendRuns(child, sb);
                }
            }
        }

        // Ô gộp: gridSpan lặp lại ô, vMerge "continue" lấy ô phía trên (như row.cells của python-docx).
        private static List<List<string>> TableRows(XElement table)
        {
            var rows = new List<List<string>>();
            List<string> previous = null;
            foreach (XElement tr in table.Elements(W + "tr"))
            {
                var cells = new List<string>();
                foreach (XElement tc in tr.Elements(W + "tc"))
                {
                    XElement tcPr = tc.Element(W + "tcPr");
                    int span = 1;
                    bool continueMerge = false;
                    if (tcPr != null)
                    {
                        XElement gridSpan = tcPr.Element(W + "gridSpan");
                        if (gridSpan != null)
                        {
                            int.TryParse((string)gridSpan.Attribute(W + "val"), out span);
                            span = Math.Max(1, span);
                        }
                        XElement vMerge = tcPr.Element(W + "vMerge");
                        continueMerge = vMerge != null && ((string)vMerge.Attribute(W + "val") ?? "continue") == "continue";
                    }
                    string text = string.Join("\n", tc.Elements(W + "p").Select(ParagraphText));
                    for (int i = 0; i < span; i++)
                    {
                        int column = cells.Count;
                        cells.Add(continueMerge && previous != null && column < previous.Count ? previous[column] : text);
                    }
                }
                rows.Add(cells);
                previous = cells;
            }
            return rows;
        }

        public static Dictionary<string, object> Profile(string path)
        {
            DocxModel model = Load(path);
            var styleCounts = new Dictionary<string, object>();
            foreach (XElement paragraph in model.Paragraphs)
            {
                string name = StyleOf(model, paragraph);
                styleCounts[name] = styleCounts.ContainsKey(name) ? (int)styleCounts[name] + 1 : 1;
            }
            return new Dictionary<string, object>
            {
                { "path", path },
                { "paragraphs", model.Paragraphs.Count },
                { "tables", model.Tables.Count },
                { "sections", model.Sections },
                { "style_counts", styleCounts },
                { "core", new Dictionary<string, object>
                    {
                        { "title", CoreValue(model.Core, Dc + "title") },
                        { "author", CoreValue(model.Core, Dc + "creator") },
                        { "created", CoreValue(model.Core, DcTerms + "created") },
                        { "modified", CoreValue(model.Core, DcTerms + "modified") }
                    }
                }
            };
        }

        private static string CoreValue(XDocument core, XName name)
        {
            if (core == null)
            {
                return null;
            }
            XElement element = core.Root.Element(name);
            string value = element != null ? element.Value : null;
            return string.IsNullOrEmpty(value) ? null : value;
        }

        public static Dictionary<string, object> GetText(string path, int maxChars, bool includeTables)
        {
            DocxModel model = Load(path);
            var parts = model.Paragraphs.Select(ParagraphText).ToList();
            if (includeTables)
            {
                foreach (XElement table in model.Tables)
                {
                    parts.AddRange(TableRows(table).Select(row => string.Join(" | ", row)));
                }
            }
            string text = string.Join("\n", parts);
            int total = text.Length;
            bool truncated = false;
            if (maxChars > 0 && text.Length > maxChars)
            {
                text = text.Substring(0, maxChars);
                truncated = true;
            }
            return new Dictionary<string, object> { { "path", path }, { "chars", total }, { "truncated", truncated }, { "text", text } };
        }

        public static Dictionary<string, object> FindText(string path, string query, int maxResults)
        {
            DocxModel model = Load(path);
            string needle = (query ?? "").ToLowerInvariant();
            var matches = new List<object>();
            for (int index = 0; index < model.Paragraphs.Count && needle.Length > 0; index++)
            {
                string text = ParagraphText(model.Paragraphs[index]);
                if (text.ToLowerInvariant().Contains(needle))
                {
                    matches.Add(new Dictionary<string, object>
                    {
                        { "index", index },
                        { "style", StyleOf(model, model.Paragraphs[index]) },
                        { "text", text.Length > 300 ? text.Substring(0, 300) : text }
                    });
                    if (matches.Count >= maxResults)
                    {
                        break;
                    }
                }
            }
            return new Dictionary<string, object> { { "query", query }, { "count", matches.Count }, { "matches", matches } };
        }

        public static Dictionary<string, object> ExtractTable(string path, int index)
        {
            DocxModel model = Load(path);
            if (index < 0 || index >= model.Tables.Count)
            {
                throw new ArgumentException("table index out of range (file has " + model.Tables.Count + " tables)");
            }
            List<List<string>> rows = TableRows(model.Tables[index]);
            return new Dictionary<string, object> { { "path", path }, { "index", index }, { "row_count", rows.Count }, { "rows", rows } };
        }

        // ---- tạo ----

        public static Dictionary<string, object> Create(string path, object[] paragraphs, string title, string author, bool overwrite)
        {
            FileSafety.RequireNewOrOverwrite(path, overwrite);
            int added = 0;
            OoxmlPackage.CreateFromTemplate(path, Templates.Docx, package =>
            {
                string documentPart = MainPart(package);
                XDocument document = package.Xml(documentPart);
                XElement body = document.Root.Element(W + "body");
                XElement sectPr = body.Element(W + "sectPr");
                Dictionary<string, string> styleIds = ParagraphStyleIds(package, documentPart);
                int blockWidth = BlockWidth(sectPr);
                foreach (object spec in paragraphs ?? new object[0])
                {
                    XElement block = null;
                    var text = spec as string;
                    var dict = spec as Dictionary<string, object>;
                    if (text != null)
                    {
                        block = Paragraph(text, null);
                    }
                    else if (dict != null && dict.ContainsKey("table"))
                    {
                        object[][] data = ToMatrix(dict["table"]);
                        if (data.Length > 0)
                        {
                            block = Table(data, blockWidth);
                        }
                        else
                        {
                            continue;
                        }
                    }
                    else if (dict != null)
                    {
                        string style = dict.ContainsKey("style") && dict["style"] != null ? Convert.ToString(dict["style"]) : null;
                        string styleId = null;
                        if (!string.IsNullOrEmpty(style))
                        {
                            if (!styleIds.TryGetValue(style.ToLowerInvariant(), out styleId))
                            {
                                throw new ArgumentException("no paragraph style with name '" + style + "'");
                            }
                        }
                        block = Paragraph(dict.ContainsKey("text") && dict["text"] != null ? Convert.ToString(dict["text"]) : "", styleId);
                    }
                    else
                    {
                        continue;
                    }
                    if (sectPr != null)
                    {
                        sectPr.AddBeforeSelf(block);
                    }
                    else
                    {
                        body.Add(block);
                    }
                    added++;
                }
                package.Put(documentPart, document);
                WriteCore(package, title, author);
            });
            return new Dictionary<string, object> { { "created", path }, { "blocks", added } };
        }

        // Tên style (cả tên nội bộ lẫn tên hiển thị, không phân biệt hoa thường) -> styleId, chỉ style đoạn.
        private static Dictionary<string, string> ParagraphStyleIds(OoxmlPackage package, string documentPart)
        {
            var result = new Dictionary<string, string>(StringComparer.Ordinal);
            Rel styles = package.RelOfType(documentPart, "styles");
            if (styles == null)
            {
                return result;
            }
            foreach (XElement style in package.Xml(styles.TargetPart).Root.Elements(W + "style"))
            {
                if ((string)style.Attribute(W + "type") != "paragraph")
                {
                    continue;
                }
                string id = (string)style.Attribute(W + "styleId");
                XElement name = style.Element(W + "name");
                string raw = name != null ? (string)name.Attribute(W + "val") : id;
                if (id == null || raw == null)
                {
                    continue;
                }
                result[raw.ToLowerInvariant()] = id;
                result[UiName(raw).ToLowerInvariant()] = id;
                if (!result.ContainsKey(id.ToLowerInvariant()))
                {
                    result[id.ToLowerInvariant()] = id;
                }
            }
            return result;
        }

        private static int BlockWidth(XElement sectPr)
        {
            try
            {
                XElement size = sectPr.Element(W + "pgSz");
                XElement margin = sectPr.Element(W + "pgMar");
                int width = int.Parse((string)size.Attribute(W + "w"));
                int left = int.Parse((string)margin.Attribute(W + "left"));
                int right = int.Parse((string)margin.Attribute(W + "right"));
                return Math.Max(1440, width - left - right);
            }
            catch (Exception)
            {
                return 8640;
            }
        }

        private static XElement Paragraph(string text, string styleId)
        {
            var p = new XElement(W + "p");
            if (styleId != null)
            {
                p.Add(new XElement(W + "pPr", new XElement(W + "pStyle", new XAttribute(W + "val", styleId))));
            }
            if (!string.IsNullOrEmpty(text))
            {
                p.Add(Run(text));
            }
            return p;
        }

        // "\n" -> <w:br/>, "\t" -> <w:tab/> như run.text của python-docx.
        private static XElement Run(string text)
        {
            var run = new XElement(W + "r");
            var buffer = new StringBuilder();
            Action flush = delegate
            {
                if (buffer.Length > 0)
                {
                    run.Add(new XElement(W + "t", new XAttribute(XmlNs + "space", "preserve"), buffer.ToString()));
                    buffer.Clear();
                }
            };
            foreach (char c in text.Replace("\r\n", "\n"))
            {
                if (c == '\n' || c == '\r')
                {
                    flush();
                    run.Add(new XElement(W + "br"));
                }
                else if (c == '\t')
                {
                    flush();
                    run.Add(new XElement(W + "tab"));
                }
                else
                {
                    buffer.Append(c);
                }
            }
            flush();
            return run;
        }

        private static XElement Table(object[][] data, int blockWidth)
        {
            int cols = Math.Max(1, data.Max(row => row.Length));
            int colWidth = blockWidth / cols;
            var grid = new XElement(W + "tblGrid");
            for (int c = 0; c < cols; c++)
            {
                grid.Add(new XElement(W + "gridCol", new XAttribute(W + "w", colWidth)));
            }
            var table = new XElement(W + "tbl",
                new XElement(W + "tblPr",
                    new XElement(W + "tblStyle", new XAttribute(W + "val", "TableGrid")),
                    new XElement(W + "tblW", new XAttribute(W + "type", "auto"), new XAttribute(W + "w", 0)),
                    new XElement(W + "tblLook", new XAttribute(W + "firstColumn", 1), new XAttribute(W + "firstRow", 1), new XAttribute(W + "lastColumn", 0),
                        new XAttribute(W + "lastRow", 0), new XAttribute(W + "noHBand", 0), new XAttribute(W + "noVBand", 1), new XAttribute(W + "val", "04A0"))),
                grid);
            foreach (object[] row in data)
            {
                var tr = new XElement(W + "tr");
                for (int c = 0; c < cols; c++)
                {
                    object value = c < row.Length ? row[c] : null;
                    tr.Add(new XElement(W + "tc",
                        new XElement(W + "tcPr", new XElement(W + "tcW", new XAttribute(W + "type", "dxa"), new XAttribute(W + "w", colWidth))),
                        Paragraph(value == null ? "" : Cells.ToText(value), null)));
                }
                table.Add(tr);
            }
            return table;
        }

        private static object[][] ToMatrix(object value)
        {
            var rows = value as object[];
            if (rows == null)
            {
                return new object[0][];
            }
            return rows.Select(row => row as object[] ?? new object[] { row }).ToArray();
        }

        // Thuộc tính tài liệu: bỏ dấu vết "python-docx" của template, ghi title/author/thời gian.
        private static void WriteCore(OoxmlPackage package, string title, string author)
        {
            Rel core = package.RelOfType("", "metadata/core-properties");
            if (core == null || !package.Exists(core.TargetPart))
            {
                return;
            }
            XDocument doc = package.Xml(core.TargetPart);
            string now = DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ");
            SetCore(doc, Dc + "title", title ?? "");
            SetCore(doc, Dc + "creator", author ?? "");
            SetCore(doc, Dc + "description", "");
            SetCore(doc, Cp + "lastModifiedBy", author ?? "");
            SetCore(doc, DcTerms + "created", now);
            SetCore(doc, DcTerms + "modified", now);
            package.Put(core.TargetPart, doc);
        }

        public static void SetCore(XDocument doc, XName name, string value)
        {
            XElement element = doc.Root.Element(name);
            if (element == null)
            {
                element = new XElement(name);
                if (name.Namespace == DcTerms)
                {
                    element.Add(new XAttribute(Xsi + "type", "dcterms:W3CDTF"));
                }
                doc.Root.Add(element);
            }
            element.Value = value;
        }
    }
}

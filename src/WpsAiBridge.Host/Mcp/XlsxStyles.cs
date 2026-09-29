using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Text;
using System.Xml;
using System.Xml.Linq;

namespace WpsAiBridge.Host.Mcp
{
    // styles.xml: tạo xf mới từ xf hiện có của ô + spec (font/fill/border/alignment/numFmt).
    internal sealed class XlsxStyles
    {
        private static readonly string[] StyleSheetOrder = { "numFmts", "fonts", "fills", "borders", "cellStyleXfs", "cellXfs", "cellStyles", "dxfs", "tableStyles", "colors", "extLst" };
        private static readonly string[] FontOrder = { "b", "i", "strike", "condense", "extend", "outline", "shadow", "u", "vertAlign", "sz", "color", "name", "family", "charset", "scheme" };
        private static readonly string[] BorderOrder = { "start", "left", "end", "right", "top", "bottom", "diagonal", "vertical", "horizontal" };
        private static readonly string[] XfOrder = { "alignment", "protection", "extLst" };

        private static readonly Dictionary<string, string> BorderStyles = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase)
        {
            { "thin", "thin" }, { "continuous", "thin" }, { "medium", "medium" }, { "thick", "thick" }, { "dash", "dashed" }, { "dashed", "dashed" },
            { "dot", "dotted" }, { "dotted", "dotted" }, { "double", "double" }, { "hair", "hair" }, { "mediumdash", "mediumDashed" },
            { "mediumdashed", "mediumDashed" }, { "dashdot", "dashDot" }, { "mediumdashdot", "mediumDashDot" }, { "dashdotdot", "dashDotDot" },
            { "mediumdashdotdot", "mediumDashDotDot" }, { "slantdashdot", "slantDashDot" }, { "none", null },
        };

        private static readonly Dictionary<string, string> Underlines = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase)
        {
            { "none", null }, { "single", "single" }, { "double", "double" }, { "singleaccounting", "singleAccounting" }, { "doubleaccounting", "doubleAccounting" },
        };

        private readonly OoxmlPackage _package;
        private readonly string _part;
        private readonly XDocument _doc;
        private readonly XNamespace S;
        private readonly Dictionary<string, int> _cache = new Dictionary<string, int>(StringComparer.Ordinal);

        public XlsxStyles(OoxmlPackage package, string part, XNamespace ns)
        {
            _package = package;
            _part = part;
            _doc = package.Xml(part);
            S = ns;
        }

        private XElement Section(string name)
        {
            XElement section = _doc.Root.Element(S + name);
            if (section == null)
            {
                section = new XElement(S + name);
                XlsxBook.InsertInOrder(_doc.Root, section, StyleSheetOrder);
            }
            return section;
        }

        private static int Append(XElement section, XElement item)
        {
            section.Add(item);
            int count = section.Elements(item.Name).Count();
            section.SetAttributeValue("count", count);
            return count - 1;
        }

        public int Derive(int baseXf, Dictionary<string, object> spec)
        {
            string key = baseXf + "|" + Json.Serialize(spec);
            int cached;
            if (_cache.TryGetValue(key, out cached))
            {
                return cached;
            }
            XElement cellXfs = Section("cellXfs");
            List<XElement> xfs = cellXfs.Elements(S + "xf").ToList();
            if (xfs.Count == 0)
            {
                Append(Section("fonts"), new XElement(S + "font", new XElement(S + "sz", new XAttribute("val", 11)), new XElement(S + "name", new XAttribute("val", "Calibri"))));
                Append(Section("fills"), new XElement(S + "fill", new XElement(S + "patternFill", new XAttribute("patternType", "none"))));
                Append(Section("borders"), new XElement(S + "border", new XElement(S + "left"), new XElement(S + "right"), new XElement(S + "top"), new XElement(S + "bottom"), new XElement(S + "diagonal")));
                Append(cellXfs, new XElement(S + "xf", new XAttribute("numFmtId", 0), new XAttribute("fontId", 0), new XAttribute("fillId", 0), new XAttribute("borderId", 0), new XAttribute("xfId", 0)));
                xfs = cellXfs.Elements(S + "xf").ToList();
            }
            XElement xf = new XElement(xfs[baseXf >= 0 && baseXf < xfs.Count ? baseXf : 0]);

            var font = spec.ContainsKey("font") ? spec["font"] as Dictionary<string, object> : null;
            if (font != null)
            {
                XElement current = ItemAt("fonts", "font", AttrInt(xf, "fontId")) ?? new XElement(S + "font");
                xf.SetAttributeValue("fontId", Append(Section("fonts"), BuildFont(current, font)));
                xf.SetAttributeValue("applyFont", 1);
            }
            var fill = spec.ContainsKey("fill") ? spec["fill"] as Dictionary<string, object> : null;
            if (fill != null)
            {
                string pattern = fill.ContainsKey("pattern") && fill["pattern"] != null ? Convert.ToString(fill["pattern"]) : "solid";
                object color = fill.ContainsKey("color") ? fill["color"] : (fill.ContainsKey("fgColor") ? fill["fgColor"] : null);
                var patternFill = new XElement(S + "patternFill", new XAttribute("patternType", pattern),
                    new XElement(S + "fgColor", new XAttribute("rgb", "FF" + Rgb(color))),
                    new XElement(S + "bgColor", new XAttribute("indexed", 64)));
                xf.SetAttributeValue("fillId", Append(Section("fills"), new XElement(S + "fill", patternFill)));
                xf.SetAttributeValue("applyFill", 1);
            }
            var border = spec.ContainsKey("border") ? spec["border"] as object[] : null;
            if (border != null)
            {
                XElement current = ItemAt("borders", "border", AttrInt(xf, "borderId")) ?? new XElement(S + "border");
                xf.SetAttributeValue("borderId", Append(Section("borders"), BuildBorder(current, border)));
                xf.SetAttributeValue("applyBorder", 1);
            }
            var alignment = spec.ContainsKey("alignment") ? spec["alignment"] as Dictionary<string, object> : null;
            if (alignment != null)
            {
                XElement align = xf.Element(S + "alignment");
                if (align == null)
                {
                    align = new XElement(S + "alignment");
                    XlsxBook.InsertInOrder(xf, align, XfOrder);
                }
                if (alignment.ContainsKey("horizontal"))
                {
                    align.SetAttributeValue("horizontal", alignment["horizontal"] == null ? null : Convert.ToString(alignment["horizontal"]));
                }
                if (alignment.ContainsKey("vertical"))
                {
                    align.SetAttributeValue("vertical", alignment["vertical"] == null ? null : Convert.ToString(alignment["vertical"]));
                }
                if (alignment.ContainsKey("wrap"))
                {
                    align.SetAttributeValue("wrapText", Truthy(alignment["wrap"]) ? "1" : null);
                }
                if (alignment.ContainsKey("rotation") && alignment["rotation"] != null)
                {
                    align.SetAttributeValue("textRotation", Convert.ToInt32(alignment["rotation"], CultureInfo.InvariantCulture));
                }
                xf.SetAttributeValue("applyAlignment", 1);
            }
            string numFmt = spec.ContainsKey("numFmt") && spec["numFmt"] != null ? Convert.ToString(spec["numFmt"]) : null;
            if (string.IsNullOrEmpty(numFmt) && spec.ContainsKey("decimalPlaces") && spec["decimalPlaces"] != null)
            {
                int places = Math.Max(0, Math.Min(30, Convert.ToInt32(spec["decimalPlaces"], CultureInfo.InvariantCulture)));
                numFmt = places == 0 ? "0" : "0." + new string('0', places);
            }
            if (!string.IsNullOrEmpty(numFmt))
            {
                xf.SetAttributeValue("numFmtId", NumFmtId(numFmt));
                xf.SetAttributeValue("applyNumberFormat", 1);
            }

            int index = Append(cellXfs, xf);
            _package.Put(_part, _doc);
            _cache[key] = index;
            return index;
        }

        private XElement ItemAt(string section, string item, int index)
        {
            XElement container = _doc.Root.Element(S + section);
            return container == null ? null : container.Elements(S + item).ElementAtOrDefault(index);
        }

        private static int AttrInt(XElement element, string name)
        {
            int value;
            return int.TryParse((string)element.Attribute(name), out value) ? value : 0;
        }

        private XElement BuildFont(XElement current, Dictionary<string, object> spec)
        {
            var parts = new Dictionary<string, XElement>();
            foreach (XElement child in current.Elements())
            {
                parts[child.Name.LocalName] = new XElement(child);
            }
            Action<string, string> flag = delegate(string key, string element)
            {
                if (spec.ContainsKey(key))
                {
                    if (Truthy(spec[key]))
                    {
                        parts[element] = new XElement(S + element);
                    }
                    else
                    {
                        parts.Remove(element);
                    }
                }
            };
            flag("bold", "b");
            flag("italic", "i");
            flag("strike", "strike");
            if (spec.ContainsKey("size") && spec["size"] != null)
            {
                parts["sz"] = new XElement(S + "sz", new XAttribute("val", Cells.ToText(Convert.ToDouble(spec["size"], CultureInfo.InvariantCulture))));
            }
            if (spec.ContainsKey("name") && spec["name"] != null && Convert.ToString(spec["name"]).Length > 0)
            {
                parts["name"] = new XElement(S + "name", new XAttribute("val", Convert.ToString(spec["name"])));
                parts.Remove("scheme"); // font theo theme sẽ đè tên font
            }
            if (spec.ContainsKey("color") && spec["color"] != null)
            {
                parts["color"] = new XElement(S + "color", new XAttribute("rgb", "FF" + Rgb(spec["color"])));
            }
            if (spec.ContainsKey("underline"))
            {
                object raw = spec["underline"];
                string kind;
                if (raw is bool)
                {
                    kind = (bool)raw ? "single" : null;
                }
                else if (raw == null || !Underlines.TryGetValue(Convert.ToString(raw), out kind))
                {
                    kind = raw == null ? null : Convert.ToString(raw);
                }
                if (kind == null)
                {
                    parts.Remove("u");
                }
                else
                {
                    parts["u"] = kind == "single" ? new XElement(S + "u") : new XElement(S + "u", new XAttribute("val", kind));
                }
            }
            if (spec.ContainsKey("vertAlign"))
            {
                string value = Convert.ToString(spec["vertAlign"] ?? "").ToLowerInvariant();
                if (value == "superscript" || value == "subscript")
                {
                    parts["vertAlign"] = new XElement(S + "vertAlign", new XAttribute("val", value));
                }
                else
                {
                    parts.Remove("vertAlign");
                }
            }
            var font = new XElement(S + "font");
            foreach (string name in FontOrder)
            {
                XElement part;
                if (parts.TryGetValue(name, out part))
                {
                    font.Add(part);
                }
            }
            return font;
        }

        private XElement BuildBorder(XElement current, object[] items)
        {
            var border = new XElement(current);
            foreach (object item in items)
            {
                var side = item as Dictionary<string, object>;
                if (side == null)
                {
                    continue;
                }
                string edge = Convert.ToString(side.ContainsKey("type") ? side["type"] : "").ToLowerInvariant();
                string styleName = Convert.ToString(side.ContainsKey("style") && side["style"] != null ? side["style"] : "thin");
                string style;
                if (!BorderStyles.TryGetValue(styleName, out style))
                {
                    style = "thin";
                }
                string color = Rgb(side.ContainsKey("color") && side["color"] != null ? side["color"] : "#000000");
                string element;
                switch (edge)
                {
                    case "left":
                    case "right":
                    case "top":
                    case "bottom":
                    case "diagonal":
                        element = edge;
                        break;
                    case "diagonalup":
                        element = "diagonal";
                        border.SetAttributeValue("diagonalUp", 1);
                        break;
                    case "diagonaldown":
                        element = "diagonal";
                        border.SetAttributeValue("diagonalDown", 1);
                        break;
                    default:
                        continue;
                }
                var sideElement = new XElement(S + element);
                if (style != null)
                {
                    sideElement.Add(new XAttribute("style", style), new XElement(S + "color", new XAttribute("rgb", "FF" + color)));
                }
                border.Elements(S + element).Remove();
                border.Elements(S + (element == "left" ? "start" : element == "right" ? "end" : "_")).Remove();
                XlsxBook.InsertInOrder(border, sideElement, BorderOrder);
            }
            return border;
        }

        private int NumFmtId(string code)
        {
            int builtin;
            if (XlsxBook.BuiltinFormats.TryGetValue(code, out builtin))
            {
                return builtin;
            }
            XElement numFmts = Section("numFmts");
            XElement existing = numFmts.Elements(S + "numFmt").FirstOrDefault(e => (string)e.Attribute("formatCode") == code);
            if (existing != null)
            {
                return AttrInt(existing, "numFmtId");
            }
            int id = Math.Max(163, numFmts.Elements(S + "numFmt").Select(e => AttrInt(e, "numFmtId")).DefaultIfEmpty(163).Max()) + 1;
            Append(numFmts, new XElement(S + "numFmt", new XAttribute("numFmtId", id), new XAttribute("formatCode", code)));
            return id;
        }

        public static string Rgb(object value)
        {
            var array = value as object[];
            if (array != null)
            {
                value = array.Length > 0 ? array[0] : null;
            }
            string text = value == null ? "" : Convert.ToString(value).Trim().TrimStart('#').ToUpperInvariant();
            if (text.Length == 6 && IsHex(text))
            {
                return text;
            }
            if (text.Length == 8 && IsHex(text))
            {
                return text.Substring(2);
            }
            throw new ArgumentException("invalid color '" + value + "' (expected #RRGGBB)");
        }

        private static bool IsHex(string text)
        {
            return text.All(c => (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F'));
        }

        private static bool Truthy(object value)
        {
            if (value == null)
            {
                return false;
            }
            if (value is bool)
            {
                return (bool)value;
            }
            string text = Convert.ToString(value, CultureInfo.InvariantCulture).ToLowerInvariant();
            return text == "true" || text == "1";
        }
    }

    // Workbook trống tối thiểu (thay openpyxl.Workbook()).
    internal static class XlsxTemplate
    {
        private static readonly XNamespace S = "http://schemas.openxmlformats.org/spreadsheetml/2006/main";
        private static readonly XNamespace R = OoxmlPackage.OfficeRelNs;

        public static XDocument StylesXml(XNamespace ns)
        {
            return new XDocument(new XDeclaration("1.0", "UTF-8", "yes"),
                new XElement(ns + "styleSheet",
                    new XElement(ns + "fonts", new XAttribute("count", 1),
                        new XElement(ns + "font", new XElement(ns + "sz", new XAttribute("val", 11)), new XElement(ns + "color", new XAttribute("theme", 1)),
                            new XElement(ns + "name", new XAttribute("val", "Calibri")), new XElement(ns + "family", new XAttribute("val", 2)),
                            new XElement(ns + "scheme", new XAttribute("val", "minor")))),
                    new XElement(ns + "fills", new XAttribute("count", 2),
                        new XElement(ns + "fill", new XElement(ns + "patternFill", new XAttribute("patternType", "none"))),
                        new XElement(ns + "fill", new XElement(ns + "patternFill", new XAttribute("patternType", "gray125")))),
                    new XElement(ns + "borders", new XAttribute("count", 1),
                        new XElement(ns + "border", new XElement(ns + "left"), new XElement(ns + "right"), new XElement(ns + "top"), new XElement(ns + "bottom"), new XElement(ns + "diagonal"))),
                    new XElement(ns + "cellStyleXfs", new XAttribute("count", 1),
                        new XElement(ns + "xf", new XAttribute("numFmtId", 0), new XAttribute("fontId", 0), new XAttribute("fillId", 0), new XAttribute("borderId", 0))),
                    new XElement(ns + "cellXfs", new XAttribute("count", 1),
                        new XElement(ns + "xf", new XAttribute("numFmtId", 0), new XAttribute("fontId", 0), new XAttribute("fillId", 0), new XAttribute("borderId", 0), new XAttribute("xfId", 0))),
                    new XElement(ns + "cellStyles", new XAttribute("count", 1),
                        new XElement(ns + "cellStyle", new XAttribute("name", "Normal"), new XAttribute("xfId", 0), new XAttribute("builtinId", 0)))));
        }

        // Gói zip workbook trống với một sheet tên firstSheet; kiểu nội dung theo đuôi file.
        public static byte[] Build(string extension, string firstSheet)
        {
            string mainType;
            switch (extension)
            {
                case ".xlsm": mainType = "application/vnd.ms-excel.sheet.macroEnabled.main+xml"; break;
                case ".xltx": mainType = "application/vnd.openxmlformats-officedocument.spreadsheetml.template.main+xml"; break;
                case ".xltm": mainType = "application/vnd.ms-excel.template.macroEnabled.main+xml"; break;
                default: mainType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"; break;
            }
            XNamespace ct = OoxmlPackage.CtNs;
            XNamespace rel = OoxmlPackage.RelNs;
            var parts = new Dictionary<string, XDocument>
            {
                { "[Content_Types].xml", new XDocument(new XElement(ct + "Types",
                    new XElement(ct + "Default", new XAttribute("Extension", "rels"), new XAttribute("ContentType", "application/vnd.openxmlformats-package.relationships+xml")),
                    new XElement(ct + "Default", new XAttribute("Extension", "xml"), new XAttribute("ContentType", "application/xml")),
                    new XElement(ct + "Override", new XAttribute("PartName", "/xl/workbook.xml"), new XAttribute("ContentType", mainType)),
                    new XElement(ct + "Override", new XAttribute("PartName", "/xl/worksheets/sheet1.xml"), new XAttribute("ContentType", XlsxBook.WorksheetContentType)),
                    new XElement(ct + "Override", new XAttribute("PartName", "/xl/styles.xml"), new XAttribute("ContentType", "application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml")))) },
                { "_rels/.rels", new XDocument(new XElement(rel + "Relationships",
                    new XElement(rel + "Relationship", new XAttribute("Id", "rId1"), new XAttribute("Type", OoxmlPackage.RelTypeBase + "officeDocument"), new XAttribute("Target", "xl/workbook.xml")))) },
                { "xl/workbook.xml", new XDocument(new XElement(S + "workbook", new XAttribute(XNamespace.Xmlns + "r", R.NamespaceName),
                    new XElement(S + "bookViews", new XElement(S + "workbookView", new XAttribute("activeTab", 0))),
                    new XElement(S + "sheets", new XElement(S + "sheet", new XAttribute("name", firstSheet), new XAttribute("sheetId", 1), new XAttribute(R + "id", "rId1"))),
                    new XElement(S + "calcPr", new XAttribute("fullCalcOnLoad", 1)))) },
                { "xl/_rels/workbook.xml.rels", new XDocument(new XElement(rel + "Relationships",
                    new XElement(rel + "Relationship", new XAttribute("Id", "rId1"), new XAttribute("Type", OoxmlPackage.RelTypeBase + "worksheet"), new XAttribute("Target", "worksheets/sheet1.xml")),
                    new XElement(rel + "Relationship", new XAttribute("Id", "rId2"), new XAttribute("Type", OoxmlPackage.RelTypeBase + "styles"), new XAttribute("Target", "styles.xml")))) },
                { "xl/worksheets/sheet1.xml", new XDocument(new XElement(S + "worksheet", new XAttribute(XNamespace.Xmlns + "r", R.NamespaceName),
                    new XElement(S + "sheetViews", new XElement(S + "sheetView", new XAttribute("tabSelected", 1), new XAttribute("workbookViewId", 0))),
                    new XElement(S + "sheetData"))) },
                { "xl/styles.xml", StylesXml(S) },
            };
            var settings = new XmlWriterSettings { Encoding = new UTF8Encoding(false), Indent = false };
            using (var memory = new MemoryStream())
            {
                using (var zip = new ZipArchive(memory, ZipArchiveMode.Create, true))
                {
                    foreach (KeyValuePair<string, XDocument> part in parts)
                    {
                        part.Value.Declaration = new XDeclaration("1.0", "UTF-8", "yes");
                        using (Stream stream = zip.CreateEntry(part.Key, CompressionLevel.Optimal).Open())
                        using (XmlWriter writer = XmlWriter.Create(stream, settings))
                        {
                            part.Value.Save(writer);
                        }
                    }
                }
                return memory.ToArray();
            }
        }
    }
}

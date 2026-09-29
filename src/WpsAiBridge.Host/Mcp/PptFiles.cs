using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Text;
using System.Xml.Linq;

namespace WpsAiBridge.Host.Mcp
{
    // Làn file cho .pptx (thay python-pptx).
    internal static class PptFiles
    {
        private static readonly XNamespace P = "http://schemas.openxmlformats.org/presentationml/2006/main";
        private static readonly XNamespace A = "http://schemas.openxmlformats.org/drawingml/2006/main";
        private static readonly XNamespace R = OoxmlPackage.OfficeRelNs;
        private const string SlideContentType = "application/vnd.openxmlformats-officedocument.presentationml.slide+xml";

        public static IEnumerable<McpTool> Tools()
        {
            yield return new McpTool("ppt_profile",
                "Inspect a .pptx file: slide count, per-slide layout, shapes, texts and speaker notes.",
                new[] { Param.Str("path", null, true) },
                a => Profile(a.Req("path")));
            yield return new McpTool("ppt_get_text",
                "Extract all text (slides + notes) from a .pptx file.",
                new[] { Param.Str("path", null, true), Param.Int("max_chars", null, false, 0) },
                a => GetText(a.Req("path"), a.Int("max_chars", 0)));
            yield return new McpTool("ppt_create",
                "Create a .pptx file. slides items: {\"title\": \"...\", \"bullets\": [\"...\", \"...\"], \"layout\": 1}. Atomic save.",
                new[] { Param.Str("path", null, true), Param.Arr("slides"), Param.Bool("overwrite", null, false, false) },
                a => Create(a.Req("path"), a.Arr("slides"), a.Bool("overwrite", false)));
            yield return new McpTool("ppt_add_slide_file",
                "Add a slide to a .pptx file on disk (offline mode).",
                new[] { Param.Str("path", null, true), Param.Str("title"), Param.Arr("bullets"), Param.Int("layout", null, false, 1) },
                a => AddSlideFile(a.Req("path"), a.Str("title"), a.Arr("bullets"), a.Int("layout", 1)));
        }

        // ---- đọc ----

        private static string PresentationPart(OoxmlPackage package)
        {
            Rel main = package.RelOfType("", "officeDocument");
            if (main == null || !package.Exists(main.TargetPart))
            {
                throw new InvalidOperationException("not a PowerPoint presentation (no main part)");
            }
            return main.TargetPart;
        }

        private static List<string> SlideParts(OoxmlPackage package, string presentationPart)
        {
            XElement list = package.Xml(presentationPart).Root.Element(P + "sldIdLst");
            List<Rel> rels = package.Rels(presentationPart);
            var result = new List<string>();
            if (list == null)
            {
                return result;
            }
            foreach (XElement sldId in list.Elements(P + "sldId"))
            {
                string rid = (string)sldId.Attribute(R + "id");
                Rel rel = rels.FirstOrDefault(r => r.Id == rid);
                if (rel != null && package.Exists(rel.TargetPart))
                {
                    result.Add(rel.TargetPart);
                }
            }
            return result;
        }

        private static IEnumerable<XElement> ShapeElements(XElement spTree)
        {
            string[] kinds = { "sp", "grpSp", "graphicFrame", "cxnSp", "pic", "contentPart" };
            return spTree.Elements().Where(e => e.Name.Namespace == P && kinds.Contains(e.Name.LocalName));
        }

        private static string TextFrameText(XElement txBody)
        {
            if (txBody == null)
            {
                return "";
            }
            return string.Join("\n", txBody.Elements(A + "p").Select(p =>
            {
                var sb = new StringBuilder();
                foreach (XElement item in p.Elements())
                {
                    if (item.Name == A + "r" || item.Name == A + "fld")
                    {
                        XElement t = item.Element(A + "t");
                        if (t != null)
                        {
                            sb.Append(t.Value);
                        }
                    }
                    else if (item.Name == A + "br")
                    {
                        sb.Append('\v');
                    }
                }
                return sb.ToString();
            }));
        }

        private static List<string> SlideTexts(XElement spTree)
        {
            var texts = new List<string>();
            foreach (XElement shape in ShapeElements(spTree))
            {
                if (shape.Name != P + "sp")
                {
                    continue;
                }
                string text = TextFrameText(shape.Element(P + "txBody"));
                if (text.Trim().Length > 0)
                {
                    texts.Add(text);
                }
            }
            return texts;
        }

        private static string NotesText(OoxmlPackage package, string slidePart)
        {
            Rel notes = package.RelOfType(slidePart, "notesSlide");
            if (notes == null || !package.Exists(notes.TargetPart))
            {
                return "";
            }
            XElement spTree = package.Xml(notes.TargetPart).Root.Element(P + "cSld").Element(P + "spTree");
            foreach (XElement shape in spTree.Elements(P + "sp"))
            {
                XElement ph = shape.Descendants(P + "ph").FirstOrDefault();
                if (ph != null && (string)ph.Attribute("type") == "body")
                {
                    return TextFrameText(shape.Element(P + "txBody"));
                }
            }
            return "";
        }

        public static Dictionary<string, object> Profile(string path)
        {
            FileSafety.RequireFile(path);
            using (OoxmlPackage package = OoxmlPackage.OpenRead(path))
            {
                string presentation = PresentationPart(package);
                var slides = new List<object>();
                List<string> slideParts = SlideParts(package, presentation);
                for (int i = 0; i < slideParts.Count; i++)
                {
                    string slidePart = slideParts[i];
                    XElement spTree = package.Xml(slidePart).Root.Element(P + "cSld").Element(P + "spTree");
                    Rel layout = package.RelOfType(slidePart, "slideLayout");
                    string layoutName = "";
                    if (layout != null && package.Exists(layout.TargetPart))
                    {
                        layoutName = (string)package.Xml(layout.TargetPart).Root.Element(P + "cSld").Attribute("name") ?? "";
                    }
                    string notes = NotesText(package, slidePart);
                    slides.Add(new Dictionary<string, object>
                    {
                        { "index", i + 1 },
                        { "layout", layoutName },
                        { "shapes", ShapeElements(spTree).Count() },
                        { "texts", SlideTexts(spTree) },
                        { "notes", notes.Length > 300 ? notes.Substring(0, 300) : notes }
                    });
                }
                XElement size = package.Xml(presentation).Root.Element(P + "sldSz");
                return new Dictionary<string, object>
                {
                    { "path", path },
                    { "slide_count", slideParts.Count },
                    { "size", new Dictionary<string, object>
                        {
                            { "width", size != null ? (object)long.Parse((string)size.Attribute("cx"), CultureInfo.InvariantCulture) : null },
                            { "height", size != null ? (object)long.Parse((string)size.Attribute("cy"), CultureInfo.InvariantCulture) : null }
                        }
                    },
                    { "slides", slides }
                };
            }
        }

        public static Dictionary<string, object> GetText(string path, int maxChars)
        {
            FileSafety.RequireFile(path);
            var parts = new List<string>();
            using (OoxmlPackage package = OoxmlPackage.OpenRead(path))
            {
                List<string> slideParts = SlideParts(package, PresentationPart(package));
                for (int i = 0; i < slideParts.Count; i++)
                {
                    parts.Add("[Slide " + (i + 1) + "]");
                    XElement spTree = package.Xml(slideParts[i]).Root.Element(P + "cSld").Element(P + "spTree");
                    parts.AddRange(SlideTexts(spTree));
                    string notes = NotesText(package, slideParts[i]);
                    if (notes.Trim().Length > 0)
                    {
                        parts.Add("[Notes] " + notes);
                    }
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

        // ---- ghi ----

        public static Dictionary<string, object> Create(string path, object[] slides, bool overwrite)
        {
            FileSafety.RequireNewOrOverwrite(path, overwrite);
            int created = 0;
            OoxmlPackage.CreateFromTemplate(path, Templates.Pptx, package =>
            {
                foreach (object item in slides ?? new object[0])
                {
                    var spec = item as Dictionary<string, object>;
                    if (item is string)
                    {
                        spec = new Dictionary<string, object> { { "title", item } };
                    }
                    if (spec == null)
                    {
                        continue;
                    }
                    int layout = spec.ContainsKey("layout") && spec["layout"] != null ? Convert.ToInt32(spec["layout"], CultureInfo.InvariantCulture) : 1;
                    string title = spec.ContainsKey("title") && spec["title"] != null ? Cells.ToText(spec["title"]) : null;
                    List<string> bullets = null;
                    if (spec.ContainsKey("bullets") && spec["bullets"] is object[])
                    {
                        bullets = ((object[])spec["bullets"]).Select(b => Cells.ToText(b)).ToList();
                    }
                    else if (spec.ContainsKey("text") && spec["text"] != null)
                    {
                        bullets = new List<string> { Cells.ToText(spec["text"]) };
                    }
                    AddSlide(package, layout, title, bullets);
                    created++;
                }
                ResetCore(package);
            });
            return new Dictionary<string, object> { { "created", path }, { "slides", created } };
        }

        public static Dictionary<string, object> AddSlideFile(string path, string title, object[] bullets, int layout)
        {
            FileSafety.RequireFile(path);
            int count = 0;
            OoxmlPackage.Edit(path, package =>
            {
                AddSlide(package, layout, title, bullets == null ? null : bullets.Select(b => Cells.ToText(b)).ToList());
                count = SlideParts(package, PresentationPart(package)).Count;
            });
            return new Dictionary<string, object> { { "path", path }, { "slide_count", count } };
        }

        private static void AddSlide(OoxmlPackage package, int layoutIndex, string title, List<string> bullets)
        {
            string presentation = PresentationPart(package);
            List<string> layouts = LayoutParts(package, presentation);
            if (layouts.Count == 0)
            {
                throw new InvalidOperationException("presentation has no slide layouts");
            }
            string layoutPart = layouts[Math.Max(0, Math.Min(layoutIndex, layouts.Count - 1))];
            string slidePart = package.NextPartName("ppt/slides/slide{0}.xml");

            XElement spTree = new XElement(P + "spTree",
                new XElement(P + "nvGrpSpPr",
                    new XElement(P + "cNvPr", new XAttribute("id", 1), new XAttribute("name", "")),
                    new XElement(P + "cNvGrpSpPr"),
                    new XElement(P + "nvPr")),
                new XElement(P + "grpSpPr",
                    new XElement(A + "xfrm",
                        new XElement(A + "off", new XAttribute("x", 0), new XAttribute("y", 0)),
                        new XElement(A + "ext", new XAttribute("cx", 0), new XAttribute("cy", 0)),
                        new XElement(A + "chOff", new XAttribute("x", 0), new XAttribute("y", 0)),
                        new XElement(A + "chExt", new XAttribute("cx", 0), new XAttribute("cy", 0)))));

            // Clone placeholder của layout (bỏ ngày/footer/số trang) như python-pptx.
            int nextId = 2;
            XElement layoutTree = package.Xml(layoutPart).Root.Element(P + "cSld").Element(P + "spTree");
            foreach (XElement shape in layoutTree.Elements(P + "sp"))
            {
                XElement ph = shape.Descendants(P + "ph").FirstOrDefault();
                if (ph == null)
                {
                    continue;
                }
                string type = (string)ph.Attribute("type") ?? "obj";
                if (type == "dt" || type == "ftr" || type == "sldNum")
                {
                    continue;
                }
                XElement layoutCNvPr = shape.Descendants(P + "cNvPr").FirstOrDefault();
                var newPh = new XElement(P + "ph");
                foreach (XAttribute attribute in ph.Attributes())
                {
                    newPh.Add(new XAttribute(attribute));
                }
                var sp = new XElement(P + "sp",
                    new XElement(P + "nvSpPr",
                        new XElement(P + "cNvPr", new XAttribute("id", nextId), new XAttribute("name", layoutCNvPr != null ? (string)layoutCNvPr.Attribute("name") ?? "" : "Placeholder " + nextId)),
                        new XElement(P + "cNvSpPr", new XElement(A + "spLocks", new XAttribute("noGrp", 1))),
                        new XElement(P + "nvPr", newPh)),
                    new XElement(P + "spPr"));
                if (type == "title" || type == "ctrTitle" || type == "subTitle" || type == "body" || type == "obj" || type == "vertTitle")
                {
                    sp.Add(new XElement(P + "txBody", new XElement(A + "bodyPr"), new XElement(A + "lstStyle"), new XElement(A + "p")));
                }
                spTree.Add(sp);
                nextId++;
            }

            if (!string.IsNullOrEmpty(title))
            {
                XElement titleShape = PlaceholderByIdx(spTree, 0);
                if (titleShape != null)
                {
                    SetText(titleShape, new List<string> { title });
                }
            }
            if (bullets != null && bullets.Count > 0)
            {
                XElement body = PlaceholderByIdx(spTree, 1);
                if (body != null)
                {
                    SetText(body, bullets);
                }
            }

            var slide = new XDocument(new XDeclaration("1.0", "UTF-8", "yes"),
                new XElement(P + "sld",
                    new XAttribute(XNamespace.Xmlns + "a", A.NamespaceName),
                    new XAttribute(XNamespace.Xmlns + "r", R.NamespaceName),
                    new XAttribute(XNamespace.Xmlns + "p", P.NamespaceName),
                    new XElement(P + "cSld", spTree),
                    new XElement(P + "clrMapOvr", new XElement(A + "masterClrMapping"))));
            package.Put(slidePart, slide);
            package.SetContentTypeOverride(slidePart, SlideContentType);
            package.AddRel(slidePart, "slideLayout", layoutPart);
            string rid = package.AddRel(presentation, "slide", slidePart);

            XDocument pres = package.Xml(presentation);
            XElement list = pres.Root.Element(P + "sldIdLst");
            if (list == null)
            {
                list = new XElement(P + "sldIdLst");
                XElement anchor = pres.Root.Element(P + "handoutMasterIdLst") ?? pres.Root.Element(P + "notesMasterIdLst") ?? pres.Root.Element(P + "sldMasterIdLst");
                if (anchor != null)
                {
                    anchor.AddAfterSelf(list);
                }
                else
                {
                    pres.Root.AddFirst(list);
                }
            }
            long id = Math.Max(255, list.Elements(P + "sldId").Select(e => (long)e.Attribute("id")).DefaultIfEmpty(255).Max()) + 1;
            list.Add(new XElement(P + "sldId", new XAttribute("id", id), new XAttribute(R + "id", rid)));
            package.Put(presentation, pres);
        }

        // Layout theo thứ tự sldLayoutIdLst của slide master đầu tiên (= prs.slide_layouts).
        private static List<string> LayoutParts(OoxmlPackage package, string presentation)
        {
            var result = new List<string>();
            XElement masterId = package.Xml(presentation).Root.Element(P + "sldMasterIdLst")?.Element(P + "sldMasterId");
            if (masterId == null)
            {
                return result;
            }
            Rel masterRel = package.Rels(presentation).FirstOrDefault(r => r.Id == (string)masterId.Attribute(R + "id"));
            if (masterRel == null)
            {
                return result;
            }
            List<Rel> masterRels = package.Rels(masterRel.TargetPart);
            XElement list = package.Xml(masterRel.TargetPart).Root.Element(P + "sldLayoutIdLst");
            if (list == null)
            {
                return result;
            }
            foreach (XElement layoutId in list.Elements(P + "sldLayoutId"))
            {
                Rel rel = masterRels.FirstOrDefault(r => r.Id == (string)layoutId.Attribute(R + "id"));
                if (rel != null)
                {
                    result.Add(rel.TargetPart);
                }
            }
            return result;
        }

        private static XElement PlaceholderByIdx(XElement spTree, int idx)
        {
            foreach (XElement shape in spTree.Elements(P + "sp"))
            {
                XElement ph = shape.Descendants(P + "ph").FirstOrDefault();
                if (ph == null)
                {
                    continue;
                }
                int value = 0;
                XAttribute attribute = ph.Attribute("idx");
                if (attribute != null)
                {
                    int.TryParse(attribute.Value, out value);
                }
                if (value == idx)
                {
                    return shape;
                }
            }
            return null;
        }

        private static void SetText(XElement shape, List<string> paragraphs)
        {
            XElement txBody = shape.Element(P + "txBody");
            if (txBody == null)
            {
                txBody = new XElement(P + "txBody", new XElement(A + "bodyPr"), new XElement(A + "lstStyle"));
                shape.Add(txBody);
            }
            txBody.Elements(A + "p").Remove();
            foreach (string paragraph in paragraphs.SelectMany(t => (t ?? "").Replace("\r\n", "\n").Split('\n')))
            {
                var p = new XElement(A + "p");
                if (paragraph.Length > 0)
                {
                    p.Add(new XElement(A + "r", new XElement(A + "rPr", new XAttribute("lang", "vi-VN"), new XAttribute("dirty", 0)), new XElement(A + "t", paragraph)));
                }
                txBody.Add(p);
            }
        }

        private static void ResetCore(OoxmlPackage package)
        {
            Rel core = package.RelOfType("", "metadata/core-properties");
            if (core == null || !package.Exists(core.TargetPart))
            {
                return;
            }
            XNamespace dc = "http://purl.org/dc/elements/1.1/";
            XNamespace dcterms = "http://purl.org/dc/terms/";
            XNamespace cp = "http://schemas.openxmlformats.org/package/2006/metadata/core-properties";
            XDocument doc = package.Xml(core.TargetPart);
            string now = DateTime.UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ");
            WordFiles.SetCore(doc, dc + "creator", "");
            WordFiles.SetCore(doc, cp + "lastModifiedBy", "");
            WordFiles.SetCore(doc, dcterms + "created", now);
            WordFiles.SetCore(doc, dcterms + "modified", now);
            package.Put(core.TargetPart, doc);
        }
    }
}

using System;
using System.Collections;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.IO.Compression;
using System.Linq;
using System.Text.RegularExpressions;
using System.Web.Script.Serialization;

namespace AxiomOffice.Bridge
{
    // Làm bài theo MẪU đính kèm (docs/template-design.md mục 6). Thí nghiệm 13/10/2026 trên PowerPoint 16 với mẫu
    // của trường: master/layout là Office mặc định, logo + dải màu nằm trong ẢNH NỀN trên từng slide, chữ là
    // textbox tự vẽ. Vì vậy:
    //   - AddSlide theo CustomLayout của slide mẫu ra slide TRẮNG -> nhân bản slide mẫu bằng InsertFromFile
    //     (giữ đủ ảnh nền, vị trí, định dạng chữ, kể cả khi chèn vào bài khác);
    //   - thay TextRange.Text giữ font/cỡ/màu của khung (khung nhiều đoạn: mọi đoạn nhận định dạng đoạn đầu);
    //   - chép ảnh bằng Shape.Export (hàm ẩn, ra đúng độ phân giải gốc) rồi AddPicture - Copy/Paste ghi đè
    //     clipboard của người dùng;
    //   - ApplyTemplate lên bài cũ giữ khổ 4:3, nền thành trắng, không có logo -> không dùng.
    // ppt_profile (Core) đọc mẫu ra các "kiểu" (kinds): tên ô chữ, sức chứa, mục lặp, vùng an toàn, ô đặt ảnh.
    internal static partial class CommandDispatcher
    {
        private static IEnumerable<CommandInfo> TemplateCommands()
        {
            return new[]
            {
                Command("wpp.newFromTemplate", "wpp", WppNewFromTemplate,
                    "Tạo bài MỚI từ tệp mẫu (.pptx/.potx): giữ khổ + master, bỏ hết slide mẫu, lưu ở `path` và mở ra",
                    Req("template", "path of the template file"), Req("path", "new .pptx to create; must not exist")).ForAgent(),
                Command("wpp.addSlideFromTemplate", "wpp", WppAddSlideFromTemplate,
                    "Chèn bản sao slide số `kind` của tệp mẫu (giữ ảnh nền, khung chữ, định dạng); trả tên các khung chữ",
                    Req("template"), Req("kind", "slide number in the template (kinds of ppt_profile)"), Opt("slide", "insert after this slide; default: at the end"),
                    Opt("tag", "source slide number; a slide already tagged so is replaced in place"),
                    Opt("expect", "shape name the inserted slide must have (checks it is the template slide)"),
                    Opt("presentation", "full path of the presentation to work on; default: the active one")).ForAgent(),
                Command("wpp.sourceManifest", "wpp", WppSourceManifest,
                    "Đọc bài nguồn (vd bài cũ .ppt) theo slide: tiêu đề, chữ, danh sách ảnh - để dựng lại theo mẫu",
                    Req("from", "path of the source presentation"), Opt("first"), Opt("last")).ForAgent(),
                Command("wpp.setShapeText", "wpp", WppSetShapeText,
                    "Thay chữ của một khung theo TÊN shape, giữ định dạng của khung (font, cỡ, màu)",
                    Req("shape", "shape name, e.g. 'TextBox 2'"), Req("text", "\\n = new paragraph"), Opt("slide"),
                    Opt("presentation", "full path of the presentation to work on; default: the active one")).ForAgent(),
                Command("wpp.deleteShapes", "wpp", WppDeleteShapes,
                    "Xoá các shape theo tên (mục lặp thừa của slide mẫu); không xoá ảnh nền",
                    Req("shapes", "array of shape names"), Opt("slide"), Opt("presentation", "full path of the presentation to work on; default: the active one")).ForAgent(),
                Command("wpp.copyPicture", "wpp", WppCopyPicture,
                    "Chép ảnh từ slide của bài khác (vd bài cũ) vào slide của bài đang làm, co vừa khung, giữ tỉ lệ",
                    Req("from", "path of the source presentation"), Req("fromSlide"), Opt("shape", "picture name; default: largest picture"),
                    Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height"),
                    Opt("overlays", "true: also copy the labels/arrows drawn over the picture"),
                    Opt("format", "png (default) or jpg (much smaller for photos and scans)"),
                    Opt("presentation", "full path of the presentation to work on; default: the active one")).ForAgent(),
                Command("wpp.addParagraphs", "wpp", WppAddParagraphs,
                    "Khung chữ nhiều đoạn giữ phân cấp (cấp thụt lề, đậm, nghiêng, số thứ tự, gạch đầu dòng); tự chọn cỡ chữ lớn nhất vừa khung",
                    Req("paragraphs", "array of {t: text, l: level 1-5, b: bold, i: italic, n: number prefix like '1.', u: bullet}"),
                    Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height"),
                    Opt("fontSize", "largest size to try (default 24)"), Opt("minFontSize", "smallest size (default 16)"),
                    Opt("color", "#RRGGBB body text"), Opt("accent", "#RRGGBB bold (heading) paragraphs"), Opt("font", "font name"),
                    Opt("anchor", "top (default) or middle"),
                    Opt("presentation", "full path of the presentation to work on; default: the active one"))
            };
        }

        private static string RequireExistingFile(Dictionary<string, object> p, string name)
        {
            string path = ParamString(p, name, null);
            if (string.IsNullOrWhiteSpace(path))
            {
                throw new ArgumentException("'" + name + "' is required");
            }
            path = Path.GetFullPath(path.Trim());
            if (!File.Exists(path))
            {
                throw new ArgumentException("'" + name + "' file not found: " + path);
            }
            return path;
        }

        private static Dictionary<string, object> WppNewFromTemplate(IAppHost host, Dictionary<string, object> p)
        {
            string template = RequireExistingFile(p, "template");
            string path = ParamString(p, "path", null);
            if (string.IsNullOrWhiteSpace(path))
            {
                throw new ArgumentException("'path' is required");
            }
            path = Path.GetFullPath(path.Trim());
            if (!string.Equals(Path.GetExtension(path), ".pptx", StringComparison.OrdinalIgnoreCase))
            {
                throw new ArgumentException("'path' must end with .pptx");
            }
            if (File.Exists(path))
            {
                throw new ArgumentException("'path' already exists: " + path + " - choose another file name (never overwrite)");
            }
            dynamic app = host.Application;
            // Untitled: mở BẢN SAO của mẫu, tệp mẫu không bị sửa; .potx cũng mở được như vậy.
            dynamic pres = app.Presentations.Open(template, 0, -1, -1);
            int templateSlides = Convert.ToInt32(pres.Slides.Count);
            for (int i = templateSlides; i >= 1; i--)
            {
                pres.Slides[i].Delete();
            }
            pres.SaveAs(path, 24);
            return new Dictionary<string, object>
            {
                { "path", Convert.ToString(pres.FullName) },
                { "templateSlides", templateSlides },
                { "slideCount", 0 },
                { "slideWidth", Convert.ToDouble(pres.PageSetup.SlideWidth) },
                { "slideHeight", Convert.ToDouble(pres.PageSetup.SlideHeight) },
                { "next", "wpp.addSlideFromTemplate {template, kind} for each slide; kinds come from ppt_profile of the template" }
            };
        }

        private static Dictionary<string, object> WppAddSlideFromTemplate(IAppHost host, Dictionary<string, object> p)
        {
            string template = RequireExistingFile(p, "template");
            int kind = ParamInt(p, "kind", 0);
            if (kind < 1)
            {
                throw new ArgumentException("'kind' must be a slide number of the template (1, 2, ...)");
            }
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            RequireSameSize(pres, template);
            int count = Convert.ToInt32(pres.Slides.Count);
            int after = ParamInt(p, "slide", count);
            if (after < 0 || after > count)
            {
                after = count;
            }
            string tag = ParamString(p, "tag", null);
            if (!string.IsNullOrWhiteSpace(tag))
            {
                tag = tag.Trim();
                after = TaggedPosition(pres, tag);
                count = Convert.ToInt32(pres.Slides.Count);
            }
            string expect = ParamString(p, "expect", null);
            dynamic slide = null;
            for (int attempt = 1; attempt <= 2 && slide == null; attempt++)
            {
                slide = InsertTemplateSlide(pres, template, kind, after);
                // Slide chèn vào phải đúng slide mẫu (có khung `expect`): lượt thật 13/10/2026 18:42, PowerPoint đang bận
                // (COM call rejected) -> slide ở vị trí chèn là slide trống "Title 1 / Subtitle 2", setShapeText hỏng
                // 5 slide liền và model chuyển chữ vào ghi chú. Sai thì xoá và chèn lại một lần.
                if (!string.IsNullOrWhiteSpace(expect) && FindShape(slide, expect) == null)
                {
                    string got = ShapeNames(slide, true);
                    slide.Delete();
                    slide = null;
                    if (attempt == 2)
                    {
                        throw new ArgumentException("the inserted slide is not slide " + kind + " of the template (it has " + got
                            + "); PowerPoint may be busy - call again");
                    }
                }
            }
            int index = Convert.ToInt32(slide.SlideIndex);
            if (!string.IsNullOrWhiteSpace(tag))
            {
                slide.Tags.Add(SourceTag, tag);
            }
            return new Dictionary<string, object>
            {
                { "slide", index },
                { "kind", kind },
                { "slideCount", Convert.ToInt32(pres.Slides.Count) },
                { "textShapes", TextShapes(slide) }
            };
        }

        // Tag trên slide dựng từ mẫu = số slide nguồn: chạy lại cùng slide nguồn thì THAY slide cũ tại chỗ (không tạo
        // trùng khi một lô bị dừng giữa chừng rồi chạy lại); slide mới chèn sau slide cuối có tag nhỏ hơn để giữ thứ tự.
        internal const string SourceTag = "AXIOM_SRC";

        internal static string SlideTag(dynamic slide)
        {
            try
            {
                string value = Convert.ToString(slide.Tags[SourceTag]);
                return string.IsNullOrEmpty(value) ? null : value;
            }
            catch
            {
                return null;
            }
        }

        // Chèn slide `kind` của mẫu sau slide `after`; trả ĐÚNG slide vừa chèn (nhận theo SlideID mới, không đoán theo
        // vị trí) và đưa nó về vị trí after + 1 nếu PowerPoint chèn chỗ khác.
        private static dynamic InsertTemplateSlide(dynamic pres, string template, int kind, int after)
        {
            var before = new HashSet<int>();
            foreach (dynamic existing in pres.Slides)
            {
                before.Add(Convert.ToInt32(existing.SlideID));
            }
            try
            {
                pres.Slides.InsertFromFile(template, Math.Min(after, before.Count), kind, kind);
            }
            catch (Exception ex)
            {
                throw new ArgumentException("cannot insert slide " + kind + " of the template (does it have that many slides? see kinds of ppt_profile): " + ex.Message);
            }
            dynamic inserted = null;
            foreach (dynamic candidate in pres.Slides)
            {
                if (!before.Contains(Convert.ToInt32(candidate.SlideID)))
                {
                    inserted = candidate;
                    break;
                }
            }
            if (inserted == null)
            {
                throw new ArgumentException("the template has no slide " + kind);
            }
            int wanted = Math.Min(after + 1, Convert.ToInt32(pres.Slides.Count));
            if (Convert.ToInt32(inserted.SlideIndex) != wanted)
            {
                inserted.MoveTo(wanted);
            }
            return inserted;
        }

        // Vị trí chèn (chèn SAU slide số này) cho slide nguồn `tag`; slide đã mang tag này bị xoá trước. Đọc tag MỘT lượt
        // qua bộ duyệt Slides (lượt thật 13/10/2026 18:41: Count = 47 nhưng Slides(47) ngoài khoảng 1-46 khi PowerPoint
        // đang bận - đếm rồi lấy theo số không an toàn).
        private static int TaggedPosition(dynamic pres, string tag)
        {
            var tags = new List<string>();
            var slides = new List<dynamic>();
            foreach (dynamic slide in pres.Slides)
            {
                slides.Add(slide);
                tags.Add(SlideTag(slide));
            }
            for (int i = tags.Count - 1; i >= 0; i--)
            {
                if (string.Equals(tags[i], tag, StringComparison.OrdinalIgnoreCase))
                {
                    int position = Convert.ToInt32(slides[i].SlideIndex);
                    slides[i].Delete();
                    return position - 1;
                }
            }
            double number;
            bool numeric = double.TryParse(tag, NumberStyles.Float, CultureInfo.InvariantCulture, out number);
            int after = 0;
            bool anyTag = false;
            for (int i = 0; i < tags.Count; i++)
            {
                string other = tags[i];
                if (other == null)
                {
                    continue;
                }
                anyTag = true;
                double otherNumber;
                bool earlier = numeric && double.TryParse(other, NumberStyles.Float, CultureInfo.InvariantCulture, out otherNumber)
                    ? otherNumber < number
                    : string.CompareOrdinal(other, tag) < 0;
                if (earlier)
                {
                    after = i + 1;
                }
            }
            // Chưa slide nào mang tag (bài dựng tay) -> chèn cuối như cũ.
            return anyTag ? after : tags.Count;
        }

        // Bài nguồn: đọc theo slide (một lần, thay cho listSlides + checkLayout từng slide). Tiêu đề = khung chữ cao
        // nhất (không tính số trang / chân trang / ngày); ảnh đánh số 1, 2... theo vị trí (trên xuống, trái sang) -
        // đúng thứ tự mà kế hoạch dựng lại dùng.
        private static Dictionary<string, object> WppSourceManifest(IAppHost host, Dictionary<string, object> p)
        {
            string from = RequireExistingFile(p, "from");
            dynamic app = host.Application;
            dynamic source = OpenSource(app, from);
            int count = Convert.ToInt32(source.Slides.Count);
            int first = Math.Max(1, ParamInt(p, "first", 1));
            int last = Math.Min(count, ParamInt(p, "last", count));
            var slides = new List<object>();
            for (int i = first; i <= last; i++)
            {
                slides.Add(DescribeSourceSlide(source.Slides[i], i));
            }
            return new Dictionary<string, object>
            {
                { "from", Convert.ToString(source.FullName) },
                { "slideCount", count },
                { "slideWidth", Convert.ToDouble(source.PageSetup.SlideWidth) },
                { "slideHeight", Convert.ToDouble(source.PageSetup.SlideHeight) },
                { "slides", slides }
            };
        }

        // Ảnh của slide nguồn theo thứ tự đọc (hàng 20 pt, trái sang phải) - số ảnh `n` của sourceManifest.
        private static List<dynamic> SourcePictures(dynamic slide)
        {
            var pictures = new List<dynamic>();
            int shapeCount = Convert.ToInt32(slide.Shapes.Count);
            for (int s = 1; s <= shapeCount; s++)
            {
                dynamic shape = slide.Shapes[s];
                if (IsPicture(shape))
                {
                    pictures.Add(shape);
                }
            }
            // Cùng hàng = tâm dọc của ảnh nằm trong khoảng cao của ảnh đầu hàng (làm tròn top theo 20 pt tách hai ảnh
            // lệch nhau 30 pt thành hai hàng - Y4.5 slide 91: sơ đồ bên trái thành ảnh 2, đảo thứ tự khi dựng lại).
            var rows = new List<List<dynamic>>();
            foreach (dynamic shape in pictures.OrderBy(picture => Convert.ToDouble(picture.Top)))
            {
                double middle = Convert.ToDouble(shape.Top) + Convert.ToDouble(shape.Height) / 2;
                List<dynamic> row = rows.LastOrDefault();
                if (row != null)
                {
                    double top = Convert.ToDouble(row[0].Top);
                    if (middle >= top && middle <= top + Convert.ToDouble(row[0].Height))
                    {
                        row.Add(shape);
                        continue;
                    }
                }
                rows.Add(new List<dynamic> { shape });
            }
            return rows.SelectMany(row => row.OrderBy(picture => Convert.ToDouble(picture.Left))).ToList();
        }

        // Khung của shape (pt).
        private static System.Drawing.RectangleF Box(dynamic shape)
        {
            return new System.Drawing.RectangleF(
                (float)Convert.ToDouble(shape.Left), (float)Convert.ToDouble(shape.Top),
                (float)Convert.ToDouble(shape.Width), (float)Convert.ToDouble(shape.Height));
        }

        // Số trang / chân trang / ngày: không phải nội dung.
        private static bool IsFooterPlaceholder(dynamic shape)
        {
            try
            {
                if (Convert.ToInt32(shape.Type) != 14)
                {
                    return false;
                }
                int placeholder = Convert.ToInt32(shape.PlaceholderFormat.Type);
                return placeholder == 13 || placeholder == 15 || placeholder == 16;
            }
            catch
            {
                return false;
            }
        }

        // Hình vẽ ĐÈ lên ảnh (nhãn, mũi tên, đường chỉ, khung chú thích) - phần của hình y khoa, không phải chữ của
        // slide. Bài Y4.4-GAN slide 5: ảnh phân thuỳ + 17 đường kẻ/khung nhãn vẽ đè; chỉ chép ảnh là mất nhãn.
        private static List<dynamic> Overlays(dynamic slide, dynamic picture)
        {
            var result = new List<dynamic>();
            System.Drawing.RectangleF area = Box(picture);
            area.Inflate(6, 6);
            string pictureName = Convert.ToString(picture.Name);
            int shapeCount = Convert.ToInt32(slide.Shapes.Count);
            for (int s = 1; s <= shapeCount; s++)
            {
                dynamic shape = slide.Shapes[s];
                try
                {
                    if (Convert.ToString(shape.Name) == pictureName || IsFooterPlaceholder(shape) || IsPicture(shape))
                    {
                        continue;
                    }
                    System.Drawing.RectangleF box = Box(shape);
                    bool hasText = false;
                    try
                    {
                        hasText = Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.TextFrame.HasText) == -1;
                    }
                    catch
                    {
                    }
                    if (hasText)
                    {
                        // Khung chữ chỉ là nhãn khi nằm phần lớn TRONG ảnh; khối chữ đặt sát mép ảnh là nội dung của
                        // slide (đo trên Y4.4-GAN: tính theo "chạm mép" thì kéo cả mảng nền xanh cũ vào ảnh).
                        System.Drawing.RectangleF inside = System.Drawing.RectangleF.Intersect(box, area);
                        if (!inside.IsEmpty && inside.Width * inside.Height >= 0.5 * box.Width * box.Height)
                        {
                            result.Add(shape);
                        }
                        continue;
                    }
                    // Đường kẻ / mũi tên / khung không chữ: chạm ảnh là thuộc ảnh. Đường ngang/dọc rộng 0: nới 1 pt.
                    box.Inflate(box.Width < 1 ? 1 : 0, box.Height < 1 ? 1 : 0);
                    if (box.IntersectsWith(area))
                    {
                        result.Add(shape);
                    }
                }
                catch
                {
                }
            }
            return result;
        }

        private static Dictionary<string, object> DescribeSourceSlide(dynamic slide, int index)
        {
            List<dynamic> pictureShapes = SourcePictures(slide);
            var overlayNames = new HashSet<string>();
            var pictures = new List<object>();
            int number = 1;
            foreach (dynamic picture in pictureShapes)
            {
                List<dynamic> overlays = Overlays(slide, picture);
                foreach (dynamic overlay in overlays)
                {
                    overlayNames.Add(Convert.ToString(overlay.Name));
                }
                var entry = new Dictionary<string, object>
                {
                    { "n", number++ },
                    { "shape", Convert.ToString(picture.Name) },
                    { "w", Math.Round(Convert.ToDouble(picture.Width)) },
                    { "h", Math.Round(Convert.ToDouble(picture.Height)) }
                };
                if (overlays.Count > 0)
                {
                    entry["overlays"] = overlays.Count;
                }
                pictures.Add(entry);
            }

            var texts = new List<KeyValuePair<double, string>>();
            var paragraphs = new Dictionary<string, List<object>>();
            int other = 0;
            int shapeCount = Convert.ToInt32(slide.Shapes.Count);
            for (int s = 1; s <= shapeCount; s++)
            {
                dynamic shape = slide.Shapes[s];
                try
                {
                    if (IsFooterPlaceholder(shape) || IsPicture(shape) || overlayNames.Contains(Convert.ToString(shape.Name)))
                    {
                        continue;
                    }
                    int type = Convert.ToInt32(shape.Type);
                    if (Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.TextFrame.HasText) == -1)
                    {
                        string text = Convert.ToString(shape.TextFrame.TextRange.Text).Replace("\r", "\n").Replace("\v", "\n").Trim();
                        if (text.Length > 0)
                        {
                            texts.Add(new KeyValuePair<double, string>(Convert.ToDouble(shape.Top), text));
                            if (!paragraphs.ContainsKey(text))
                            {
                                paragraphs[text] = DescribeParagraphs(shape.TextFrame.TextRange);
                            }
                        }
                    }
                    else if (type == 6 || type == 3 || type == 19 || type == 24 || type == 12)
                    {
                        other++; // nhóm, biểu đồ, bảng, SmartArt, điều khiển: không dựng lại tự động được
                    }
                }
                catch
                {
                }
            }
            List<string> ordered = texts.OrderBy(t => t.Key).Select(t => t.Value).ToList();
            var result = new Dictionary<string, object>
            {
                { "n", index },
                { "title", ordered.Count > 0 ? ordered[0] : "" },
                { "texts", ordered.Skip(1).Select(t => t.Length > 400 ? t.Substring(0, 400) + "..." : t).ToList() },
                { "pictures", pictures }
            };
            // Từng đoạn kèm phân cấp (cấp, đậm, nghiêng, số tự động) - dựng lại giữ cấu trúc chữ của bài cũ thay vì một
            // khối chữ phẳng (lượt thật 13/10/2026 18:10: "2. Siêu âm:" mất in đậm, 4 mục con mất số thứ tự).
            if (ordered.Count > 0)
            {
                result["titleParas"] = paragraphs[ordered[0]];
                result["textParas"] = ordered.Skip(1).Select(t => (object)paragraphs[t]).ToList();
            }
            if (other > 0)
            {
                result["other"] = other;
            }
            return result;
        }

        // Đoạn văn của một khung chữ: {t, l, b, i, n, u}. Số tự động của PowerPoint (Bullet.Type 2) không nằm trong
        // Text nên ghi ra thành tiền tố "1." (đúng số đang hiển thị) - dựng lại ở bài khác không phụ thuộc chuỗi đánh số.
        private static List<object> DescribeParagraphs(dynamic range)
        {
            var result = new List<object>();
            int count = Convert.ToInt32(range.Paragraphs().Count);
            for (int k = 1; k <= count && result.Count < 60; k++)
            {
                dynamic paragraph = range.Paragraphs(k);
                string raw = Convert.ToString(paragraph.Text).TrimEnd('\r', '\n', '\v').Replace("\v", "\n");
                string text = raw.Trim();
                if (text.Length == 0)
                {
                    continue;
                }
                var entry = new Dictionary<string, object> { { "t", text } };
                // Bài cũ hay thụt mục con bằng phím Tab ("\t1. SA gan mật") thay vì cấp đoạn: mỗi Tab đầu dòng = một cấp.
                int tabs = raw.TakeWhile(char.IsWhiteSpace).Count(c => c == '\t');
                try
                {
                    int level = Convert.ToInt32(paragraph.IndentLevel) + tabs;
                    if (level > 1)
                    {
                        entry["l"] = Math.Min(level, 5);
                    }
                }
                catch
                {
                }
                try
                {
                    if (Convert.ToInt32(paragraph.Font.Bold) == -1)
                    {
                        entry["b"] = true;
                    }
                    if (Convert.ToInt32(paragraph.Font.Italic) == -1)
                    {
                        entry["i"] = true;
                    }
                }
                catch
                {
                }
                try
                {
                    dynamic bullet = paragraph.ParagraphFormat.Bullet;
                    if (Convert.ToInt32(bullet.Visible) == -1)
                    {
                        int type = Convert.ToInt32(bullet.Type);
                        if (type == 2)
                        {
                            entry["n"] = NumberPrefix(Convert.ToInt32(bullet.Style), Convert.ToInt32(bullet.Number));
                        }
                        else if (type == 1 || type == 3)
                        {
                            entry["u"] = true;
                        }
                    }
                }
                catch
                {
                }
                result.Add(entry);
            }
            return result;
        }

        // PpNumberedBulletStyle -> chữ số đang hiển thị ("3.", "b)", "IV.").
        private static string NumberPrefix(int style, int number)
        {
            string alpha = number >= 1 && number <= 26 ? ((char)('a' + number - 1)).ToString() : number.ToString(CultureInfo.InvariantCulture);
            string arabic = number.ToString(CultureInfo.InvariantCulture);
            string roman = Roman(number);
            switch (style)
            {
                case 0: return alpha + ".";
                case 1: return alpha.ToUpperInvariant() + ".";
                case 2: return arabic + ")";
                case 4: return "(" + roman.ToLowerInvariant() + ")";
                case 5: return roman.ToLowerInvariant() + ")";
                case 6: return roman.ToLowerInvariant() + ".";
                case 7: return roman + ".";
                case 8: return "(" + alpha + ")";
                case 9: return alpha + ")";
                case 10: return "(" + alpha.ToUpperInvariant() + ")";
                case 11: return alpha.ToUpperInvariant() + ")";
                case 12: return "(" + arabic + ")";
                case 13: return arabic;
                default: return arabic + ".";
            }
        }

        private static string Roman(int number)
        {
            if (number < 1 || number > 39)
            {
                return number.ToString(CultureInfo.InvariantCulture);
            }
            string[] tens = { "", "X", "XX", "XXX" };
            string[] ones = { "", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX" };
            return tens[number / 10] + ones[number % 10];
        }

        // Khung chữ nhiều đoạn cho slide dựng lại: giữ cấp thụt lề, đậm (tô màu nhấn), nghiêng, số thứ tự, gạch đầu
        // dòng của bài cũ; thử cỡ chữ từ fontSize xuống minFontSize, lấy cỡ lớn nhất vừa khung (BoundHeight). Khung
        // giữ đúng chiều cao (AutoSize = 0) như wpp.addText; neo giữa khi anchor = middle để khối chữ cân giữa vùng.
        private static Dictionary<string, object> WppAddParagraphs(IAppHost host, Dictionary<string, object> p)
        {
            var items = new List<Dictionary<string, object>>();
            object raw;
            if (p.TryGetValue("paragraphs", out raw) && raw is IEnumerable && !(raw is string))
            {
                foreach (object item in (IEnumerable)raw)
                {
                    var entry = item as Dictionary<string, object>;
                    if (entry != null && entry.ContainsKey("t") && Convert.ToString(entry["t"]).Trim().Length > 0)
                    {
                        items.Add(entry);
                    }
                }
            }
            if (items.Count == 0)
            {
                throw new ArgumentException("'paragraphs' must be a non-empty array of {t, l, b, i, n, u}");
            }
            double left = ParamDouble(p, "left", 60);
            double top = ParamDouble(p, "top", 60);
            double width = ParamDouble(p, "width", 540);
            double height = ParamDouble(p, "height", 300);
            int maxSize = Math.Max(8, ParamInt(p, "fontSize", 24));
            int minSize = Math.Max(8, Math.Min(maxSize, ParamInt(p, "minFontSize", 16)));
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic shape = slide.Shapes.AddTextbox(1, left, top, width, height);
            dynamic frame = shape.TextFrame;
            try
            {
                frame.AutoSize = 0;
            }
            catch
            {
            }
            frame.WordWrap = -1;
            frame.VerticalAnchor = string.Equals(ParamString(p, "anchor", "top"), "middle", StringComparison.OrdinalIgnoreCase) ? 3 : 1;

            Func<Dictionary<string, object>, string, bool> flag = (entry, key) =>
            {
                object value;
                return entry.TryGetValue(key, out value) && value is bool && (bool)value;
            };
            Func<Dictionary<string, object>, int> levelOf = entry =>
            {
                object value;
                int level = entry.TryGetValue("l", out value) ? Convert.ToInt32(value, CultureInfo.InvariantCulture) : 1;
                return Math.Max(1, Math.Min(5, level));
            };
            var lines = new List<string>();
            foreach (var entry in items)
            {
                object number;
                string prefix = entry.TryGetValue("n", out number) ? Convert.ToString(number).Trim() : "";
                string text = Convert.ToString(entry["t"]).Trim().Replace("\r\n", "\n").Replace("\r", "\n").Replace("\n", "\v");
                lines.Add(prefix.Length > 0 ? prefix + " " + text : text);
            }
            dynamic range = frame.TextRange;
            range.Text = string.Join("\r", lines);
            string font = ParamString(p, "font", null);
            if (!string.IsNullOrWhiteSpace(font))
            {
                range.Font.Name = font;
            }
            int? color = ParseBgrColor(ParamString(p, "color", null));
            int? accent = ParseBgrColor(ParamString(p, "accent", null));
            if (color.HasValue)
            {
                range.Font.Color.RGB = color.Value;
            }
            range.Font.Bold = 0;
            range.Font.Italic = 0;
            range.ParagraphFormat.Alignment = 1;

            for (int k = 0; k < items.Count; k++)
            {
                var entry = items[k];
                dynamic paragraph = range.Paragraphs(k + 1);
                int level = levelOf(entry);
                paragraph.IndentLevel = level;
                if (flag(entry, "b"))
                {
                    paragraph.Font.Bold = -1;
                    if (accent.HasValue)
                    {
                        paragraph.Font.Color.RGB = accent.Value;
                    }
                }
                if (flag(entry, "i"))
                {
                    paragraph.Font.Italic = -1;
                }
                dynamic bullet = paragraph.ParagraphFormat.Bullet;
                if (flag(entry, "u"))
                {
                    bullet.Visible = -1;
                    bullet.Type = 1;
                    bullet.Character = 8226;
                }
                else
                {
                    bullet.Visible = 0;
                }
            }

            double inner = Convert.ToDouble(frame.MarginTop) + Convert.ToDouble(frame.MarginBottom);
            int chosen = minSize;
            bool overflow = true;
            dynamic range2 = shape.TextFrame2.TextRange;
            for (int size = maxSize; size >= minSize; size--)
            {
                range.Font.Size = size;
                for (int k = 0; k < items.Count; k++)
                {
                    // Thụt lề đặt trên TỪNG đoạn (TextFrame2): Ruler.Levels ghi marL lệch (lượt thật 13/10/2026 18:40: đoạn
                    // cấp 1 thụt 110 pt, mục con cấp 2 chỉ 36 pt - mục con nằm trái mục cha). Mỗi cấp 1,2 em; đoạn có gạch
                    // đầu dòng treo 0,9 em để dòng sau thẳng chữ dòng đầu.
                    int level = levelOf(items[k]);
                    double hang = flag(items[k], "u") ? Math.Round(0.9 * size, 1) : 0;
                    dynamic format2 = range2.Paragraphs(k + 1).ParagraphFormat;
                    format2.LeftIndent = (float)Math.Round((level - 1) * 1.2 * size + hang, 1);
                    format2.FirstLineIndent = (float)-hang;
                    dynamic format = range.Paragraphs(k + 1).ParagraphFormat;
                    format.LineRuleBefore = 0;
                    // Cách nhóm: đoạn cấp 1 (trừ đoạn đầu) cách trên 0,35 em; đoạn con dính với đoạn cha.
                    format.SpaceBefore = k > 0 && level == 1 ? Math.Round(0.35 * size, 1) : 0;
                }
                if (Convert.ToDouble(range.BoundHeight) <= height - inner + LayoutTolerance)
                {
                    chosen = size;
                    overflow = false;
                    break;
                }
            }
            shape.Height = height;
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "shape", Convert.ToString(shape.Name) },
                { "fontSize", chosen },
                { "textHeight", Math.Round(Convert.ToDouble(range.BoundHeight), 1) },
                { "overflow", overflow }
            };
        }

        // Ảnh + hình vẽ đè: xuất CẢ slide nguồn ra PNG (Slide.Export, không sửa bài nguồn) rồi cắt đúng vùng bao ảnh
        // và nhãn. 3 điểm ảnh mỗi pt (slide 720 pt -> 2160 px), trần 4000 px.
        private static void ExportRegion(dynamic presentation, dynamic slide, dynamic picture, List<dynamic> overlays, string path, bool jpeg)
        {
            double slideWidth = Convert.ToDouble(presentation.PageSetup.SlideWidth);
            double slideHeight = Convert.ToDouble(presentation.PageSetup.SlideHeight);
            double scale = Math.Min(2.5, 4000.0 / Math.Max(slideWidth, slideHeight));
            int widthPx = (int)Math.Round(slideWidth * scale);
            int heightPx = (int)Math.Round(slideHeight * scale);
            System.Drawing.RectangleF region = Box(picture);
            foreach (dynamic overlay in overlays)
            {
                region = System.Drawing.RectangleF.Union(region, Box(overlay));
            }
            region.Inflate(2, 2);
            region.Intersect(new System.Drawing.RectangleF(0, 0, (float)slideWidth, (float)slideHeight));
            string whole = Path.Combine(Path.GetTempPath(), "axiom-slide-" + Guid.NewGuid().ToString("N") + ".png");
            try
            {
                slide.Export(whole, "PNG", widthPx, heightPx);
                using (var bitmap = new System.Drawing.Bitmap(whole))
                {
                    var crop = new System.Drawing.Rectangle(
                        (int)Math.Floor(region.X * scale), (int)Math.Floor(region.Y * scale),
                        (int)Math.Ceiling(region.Width * scale), (int)Math.Ceiling(region.Height * scale));
                    crop.Intersect(new System.Drawing.Rectangle(0, 0, bitmap.Width, bitmap.Height));
                    using (System.Drawing.Bitmap part = bitmap.Clone(crop, bitmap.PixelFormat))
                    {
                        if (jpeg)
                        {
                            SaveJpeg(part, path);
                        }
                        else
                        {
                            part.Save(path, System.Drawing.Imaging.ImageFormat.Png);
                        }
                    }
                }
            }
            finally
            {
                try
                {
                    File.Delete(whole);
                }
                catch
                {
                }
            }
        }

        // JPEG chất lượng 90: ảnh chụp y khoa (SA, CLVT, XQ) gần như không đổi mà nhỏ hơn PNG nhiều lần (bài 87 slide:
        // PNG 98 MB).
        private static void SaveJpeg(System.Drawing.Image image, string path)
        {
            System.Drawing.Imaging.ImageCodecInfo codec = System.Drawing.Imaging.ImageCodecInfo.GetImageEncoders()
                .First(c => c.FormatID == System.Drawing.Imaging.ImageFormat.Jpeg.Guid);
            using (var parameters = new System.Drawing.Imaging.EncoderParameters(1))
            {
                parameters.Param[0] = new System.Drawing.Imaging.EncoderParameter(System.Drawing.Imaging.Encoder.Quality, 90L);
                if (image.PixelFormat == System.Drawing.Imaging.PixelFormat.Format32bppArgb
                    || image.PixelFormat == System.Drawing.Imaging.PixelFormat.Format32bppPArgb)
                {
                    // JPEG không có kênh trong suốt: vẽ lên nền trắng (nền của mẫu) trước khi nén.
                    using (var flat = new System.Drawing.Bitmap(image.Width, image.Height, System.Drawing.Imaging.PixelFormat.Format24bppRgb))
                    {
                        using (System.Drawing.Graphics g = System.Drawing.Graphics.FromImage(flat))
                        {
                            g.Clear(System.Drawing.Color.White);
                            g.DrawImage(image, 0, 0, image.Width, image.Height);
                        }
                        flat.Save(path, codec, parameters);
                    }
                    return;
                }
                image.Save(path, codec, parameters);
            }
        }

        // Bài để làm việc: `presentation` (đường dẫn) nếu có, không thì bài đang hoạt động. Lượt thật 13/10/2026 17:04: khung
        // Axiom gắn vào cửa sổ bài CŨ; giữa lô người dùng bấm vào đó -> "bài đang hoạt động" thành bài cũ, các lệnh sau
        // trúng bài cũ (chặn được nhờ RequireSameSize). Dựng theo mẫu luôn chỉ đích danh bài mới.
        internal static dynamic TargetPresentation(dynamic app, Dictionary<string, object> p)
        {
            string path = ParamString(p, "presentation", null);
            if (string.IsNullOrWhiteSpace(path))
            {
                return app.ActivePresentation;
            }
            dynamic found = FindPresentation(app, path.Trim());
            if (found == null)
            {
                throw new ArgumentException("presentation is not open: " + path + " (open it with wpp.open first)");
            }
            return found;
        }

        // Bài nguồn đang mở thì dùng luôn; chưa mở thì mở chỉ đọc CÓ cửa sổ rồi trả quyền cho bài đang làm.
        private static dynamic OpenSource(dynamic app, string from)
        {
            dynamic source = FindPresentation(app, from);
            if (source != null)
            {
                return source;
            }
            dynamic target = null;
            try
            {
                target = app.ActivePresentation;
            }
            catch
            {
            }
            source = app.Presentations.Open(from, -1, 0, -1);
            if (target != null)
            {
                try
                {
                    target.Windows[1].Activate();
                }
                catch
                {
                }
            }
            return source;
        }

        // Chỉ chèn slide mẫu vào bài CÙNG khổ với mẫu. Lượt thật 13/10/2026 (mimo): model bỏ qua newFromTemplate,
        // chèn slide mẫu 16:9 thẳng vào bài cũ 4:3 của người dùng - bài cũ bị sửa, slide mẫu bị co méo.
        private static void RequireSameSize(dynamic pres, string template)
        {
            double[] size = TemplateSlideSize(template);
            if (size == null)
            {
                return;
            }
            double width = Convert.ToDouble(pres.PageSetup.SlideWidth);
            double height = Convert.ToDouble(pres.PageSetup.SlideHeight);
            if (Math.Abs(width - size[0]) > 1 || Math.Abs(height - size[1]) > 1)
            {
                throw new ArgumentException("the open presentation '" + Convert.ToString(pres.Name) + "' is " + width + "x" + height
                    + "pt but the template is " + size[0] + "x" + size[1] + "pt: call wpp.newFromTemplate {template, path} first "
                    + "to create a NEW presentation from the template (never put template slides into the user's old deck)");
            }
        }

        // Khổ slide (pt) đọc từ p:sldSz của .pptx/.potx; tệp khác (.ppt nhị phân) hoặc lỗi -> null (không chặn).
        private static double[] TemplateSlideSize(string template)
        {
            try
            {
                using (var zip = new ZipArchive(File.OpenRead(template), ZipArchiveMode.Read))
                {
                    ZipArchiveEntry entry = zip.GetEntry("ppt/presentation.xml");
                    if (entry == null)
                    {
                        return null;
                    }
                    string xml;
                    using (var reader = new StreamReader(entry.Open()))
                    {
                        xml = reader.ReadToEnd();
                    }
                    Match size = Regex.Match(xml, @"<p:sldSz[^>]*\bcx=""(\d+)""[^>]*\bcy=""(\d+)""");
                    if (!size.Success)
                    {
                        return null;
                    }
                    return new[] { double.Parse(size.Groups[1].Value) / 12700, double.Parse(size.Groups[2].Value) / 12700 };
                }
            }
            catch (Exception)
            {
                return null;
            }
        }

        // Khung có chữ của slide: tên + chữ mẫu (rút gọn) + cỡ, để model gọi wpp.setShapeText đúng tên.
        private static List<object> TextShapes(dynamic slide)
        {
            var shapes = new List<object>();
            int shapeCount = Convert.ToInt32(slide.Shapes.Count);
            for (int i = 1; i <= shapeCount; i++)
            {
                dynamic shape = slide.Shapes[i];
                try
                {
                    if (Convert.ToInt32(shape.HasTextFrame) != -1 || Convert.ToInt32(shape.TextFrame.HasText) != -1)
                    {
                        continue;
                    }
                    string text = Convert.ToString(shape.TextFrame.TextRange.Text).Replace("\r", " / ");
                    shapes.Add(new Dictionary<string, object>
                    {
                        { "shape", Convert.ToString(shape.Name) },
                        { "text", text.Length > 40 ? text.Substring(0, 40) + "..." : text },
                        { "fontSize", Convert.ToDouble(shape.TextFrame.TextRange.Font.Size) }
                    });
                }
                catch
                {
                }
            }
            return shapes;
        }

        private static dynamic FindShape(dynamic slide, string name)
        {
            int shapeCount = Convert.ToInt32(slide.Shapes.Count);
            dynamic loose = null;
            for (int i = 1; i <= shapeCount; i++)
            {
                dynamic shape = slide.Shapes[i];
                string shapeName = Convert.ToString(shape.Name);
                if (string.Equals(shapeName, name, StringComparison.Ordinal))
                {
                    return shape;
                }
                if (loose == null && string.Equals(shapeName.Trim(), name.Trim(), StringComparison.OrdinalIgnoreCase))
                {
                    loose = shape;
                }
            }
            return loose;
        }

        private static string ShapeNames(dynamic slide, bool textOnly)
        {
            var names = new List<string>();
            int shapeCount = Convert.ToInt32(slide.Shapes.Count);
            for (int i = 1; i <= shapeCount; i++)
            {
                dynamic shape = slide.Shapes[i];
                try
                {
                    // Khung chữ = có chữ, textbox (17) hoặc placeholder (14); hình vẽ nào cũng có TextFrame nên không tính.
                    int type = Convert.ToInt32(shape.Type);
                    bool holdsText = Convert.ToInt32(shape.HasTextFrame) == -1
                        && (type == 17 || type == 14 || Convert.ToInt32(shape.TextFrame.HasText) == -1);
                    if (!textOnly || holdsText)
                    {
                        names.Add("'" + Convert.ToString(shape.Name) + "'");
                    }
                }
                catch
                {
                }
            }
            return names.Count == 0 ? "(none)" : string.Join(", ", names.ToArray());
        }

        private static Dictionary<string, object> WppSetShapeText(IAppHost host, Dictionary<string, object> p)
        {
            string name = ParamString(p, "shape", null);
            if (string.IsNullOrWhiteSpace(name))
            {
                throw new ArgumentException("'shape' is required (a shape name, e.g. 'TextBox 2')");
            }
            string text = ParamString(p, "text", "");
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            int index = Convert.ToInt32(slide.SlideIndex);
            dynamic shape = FindShape(slide, name);
            if (shape == null)
            {
                throw new ArgumentException("slide " + index + " has no shape '" + name + "'; text shapes: " + ShapeNames(slide, true));
            }
            if (Convert.ToInt32(shape.HasTextFrame) != -1)
            {
                throw new ArgumentException("shape '" + name + "' cannot hold text; text shapes: " + ShapeNames(slide, true));
            }
            dynamic range = shape.TextFrame.TextRange;
            range.Text = text.Replace("\r\n", "\n").Replace("\n", "\r");
            var result = new Dictionary<string, object>
            {
                { "slide", index },
                { "shape", Convert.ToString(shape.Name) },
                { "chars", text.Length }
            };
            bool grew = false;
            try
            {
                // Khung "tự giãn theo chữ" của mẫu: PowerPoint điều khiển qua COM không tự giãn lại (đo 13/10/2026:
                // tiêu đề 2 dòng vẫn cao 46,8 pt, cả khi có cửa sổ) -> tự đặt chiều cao theo chữ, giữ mép trên.
                if (Convert.ToInt32(shape.TextFrame.AutoSize) == 1)
                {
                    double needed = Convert.ToDouble(range.BoundHeight) + Convert.ToDouble(shape.TextFrame.MarginTop)
                        + Convert.ToDouble(shape.TextFrame.MarginBottom);
                    if (Math.Abs(needed - Convert.ToDouble(shape.Height)) > LayoutTolerance)
                    {
                        double top = Convert.ToDouble(shape.Top);
                        grew = needed > Convert.ToDouble(shape.Height);
                        shape.Height = needed;
                        shape.Top = top;
                    }
                }
                result["height"] = Math.Round(Convert.ToDouble(shape.Height), 1);
                result["lines"] = Convert.ToInt32(range.Lines().Count);
                bool overflow = Convert.ToDouble(range.BoundHeight) > Convert.ToDouble(shape.Height) + LayoutTolerance;
                result["overflow"] = overflow;
                if (overflow)
                {
                    result["hint"] = "text is taller than the shape: shorten it (maxChars of the slot in ppt_profile kinds), do not lower the font size";
                }
                else if (grew)
                {
                    result["hint"] = "the shape grows with its text: run wpp.checkLayout for this slide to see whether it now overlaps the shapes below";
                }
            }
            catch
            {
            }
            return result;
        }

        private static List<string> ParamNames(Dictionary<string, object> p, string name)
        {
            object value;
            if (p == null || !p.TryGetValue(name, out value) || value == null)
            {
                throw new ArgumentException("'" + name + "' is required (array of shape names)");
            }
            var text = value as string;
            if (text != null && text.TrimStart().StartsWith("[", StringComparison.Ordinal))
            {
                value = new JavaScriptSerializer().DeserializeObject(text);
            }
            var names = new List<string>();
            var list = value as IEnumerable;
            if (list != null && !(value is string))
            {
                foreach (object item in list)
                {
                    if (item != null && Convert.ToString(item).Trim() != "")
                    {
                        names.Add(Convert.ToString(item).Trim());
                    }
                }
            }
            else if (Convert.ToString(value).Trim() != "")
            {
                names.Add(Convert.ToString(value).Trim());
            }
            if (names.Count == 0)
            {
                throw new ArgumentException("'" + name + "' is empty");
            }
            return names;
        }

        // Ảnh phủ >= 90% slide = ảnh nền của mẫu (logo, dải màu): xoá là mất nhận diện.
        private static bool IsBackgroundPicture(dynamic shape, double slideWidth, double slideHeight)
        {
            try
            {
                if (Convert.ToInt32(shape.Type) != 13)
                {
                    return false;
                }
                double left = Math.Max(0, Convert.ToDouble(shape.Left));
                double top = Math.Max(0, Convert.ToDouble(shape.Top));
                double right = Math.Min(slideWidth, Convert.ToDouble(shape.Left) + Convert.ToDouble(shape.Width));
                double bottom = Math.Min(slideHeight, Convert.ToDouble(shape.Top) + Convert.ToDouble(shape.Height));
                return right > left && bottom > top && (right - left) * (bottom - top) >= 0.9 * slideWidth * slideHeight;
            }
            catch
            {
                return false;
            }
        }

        private static Dictionary<string, object> WppDeleteShapes(IAppHost host, Dictionary<string, object> p)
        {
            List<string> names = ParamNames(p, "shapes");
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            double width = Convert.ToDouble(pres.PageSetup.SlideWidth);
            double height = Convert.ToDouble(pres.PageSetup.SlideHeight);
            var deleted = new List<string>();
            var notFound = new List<string>();
            var refused = new List<string>();
            foreach (string name in names)
            {
                dynamic shape = FindShape(slide, name);
                if (shape == null)
                {
                    notFound.Add(name);
                    continue;
                }
                if (IsBackgroundPicture(shape, width, height))
                {
                    refused.Add(name);
                    continue;
                }
                deleted.Add(Convert.ToString(shape.Name));
                shape.Delete();
            }
            var result = new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "deleted", deleted },
                { "remaining", Convert.ToInt32(slide.Shapes.Count) }
            };
            if (notFound.Count > 0)
            {
                result["notFound"] = notFound;
                result["shapes"] = ShapeNames(slide, false);
            }
            if (refused.Count > 0)
            {
                result["refused"] = refused;
                result["hint"] = "the template background picture (logo, color bands) is never deleted";
            }
            return result;
        }

        // Bài nguồn đang mở trong PowerPoint (so đường dẫn đầy đủ, rồi tên tệp).
        private static dynamic FindPresentation(dynamic app, string path)
        {
            string full = Path.GetFullPath(path);
            int count = Convert.ToInt32(app.Presentations.Count);
            dynamic byName = null;
            for (int i = 1; i <= count; i++)
            {
                dynamic pres = app.Presentations[i];
                string fullName = Convert.ToString(pres.FullName);
                if (string.Equals(fullName, full, StringComparison.OrdinalIgnoreCase))
                {
                    return pres;
                }
                if (byName == null && string.Equals(Convert.ToString(pres.Name), Path.GetFileName(full), StringComparison.OrdinalIgnoreCase))
                {
                    byName = pres;
                }
            }
            return byName;
        }

        private static bool IsPicture(dynamic shape)
        {
            try
            {
                int type = Convert.ToInt32(shape.Type);
                // 13 ảnh, 11 ảnh liên kết, 7/10 OLE nhúng/liên kết (sơ đồ của bài cũ: "Object 4" ở slide 4 Y4.4-GAN).
                if (type == 13 || type == 11 || type == 7 || type == 10)
                {
                    return true;
                }
                return type == 14 && Convert.ToInt32(shape.PlaceholderFormat.ContainedType) == 13;
            }
            catch
            {
                return false;
            }
        }

        private static Dictionary<string, object> WppCopyPicture(IAppHost host, Dictionary<string, object> p)
        {
            string from = RequireExistingFile(p, "from");
            int fromSlide = ParamInt(p, "fromSlide", 0);
            dynamic app = host.Application;
            dynamic target = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(target, ParamInt(p, "slide", 0));
            // Mở CÓ cửa sổ, chỉ đọc, rồi trả quyền cho bài đang làm: Close bài không cửa sổ làm POWERPNT.EXE sập
            // (RPC_E_SERVERFAULT, đo 13/10/2026) nên không mở ngầm; bài nguồn để mở cho người dùng tự đóng.
            dynamic source = OpenSource(app, from);
            int sourceCount = Convert.ToInt32(source.Slides.Count);
            if (fromSlide < 1 || fromSlide > sourceCount)
            {
                throw new ArgumentException("'fromSlide' must be 1.." + sourceCount);
            }
            dynamic sourceSlide = source.Slides[fromSlide];
            string name = ParamString(p, "shape", null);
            dynamic picture = null;
            if (!string.IsNullOrWhiteSpace(name))
            {
                picture = FindShape(sourceSlide, name);
                if (picture == null)
                {
                    throw new ArgumentException("slide " + fromSlide + " of the source has no shape '" + name + "'; shapes: " + ShapeNames(sourceSlide, false));
                }
            }
            else
            {
                double best = 0;
                int shapeCount = Convert.ToInt32(sourceSlide.Shapes.Count);
                for (int i = 1; i <= shapeCount; i++)
                {
                    dynamic shape = sourceSlide.Shapes[i];
                    double area = Convert.ToDouble(shape.Width) * Convert.ToDouble(shape.Height);
                    if (IsPicture(shape) && area > best)
                    {
                        best = area;
                        picture = shape;
                    }
                }
                if (picture == null)
                {
                    throw new ArgumentException("slide " + fromSlide + " of the source has no picture; shapes: " + ShapeNames(sourceSlide, false));
                }
            }

            double slideWidth = Convert.ToDouble(target.PageSetup.SlideWidth);
            double slideHeight = Convert.ToDouble(target.PageSetup.SlideHeight);
            double left = ParamDouble(p, "left", 36);
            double top = ParamDouble(p, "top", 36);
            double width = ParamDouble(p, "width", slideWidth - left - 36);
            double height = ParamDouble(p, "height", slideHeight - top - 36);
            if (width <= 0 || height <= 0)
            {
                throw new ArgumentException("'width' and 'height' must be positive");
            }

            bool jpeg = string.Equals(ParamString(p, "format", "png"), "jpg", StringComparison.OrdinalIgnoreCase)
                || string.Equals(ParamString(p, "format", "png"), "jpeg", StringComparison.OrdinalIgnoreCase);
            string temp = Path.Combine(Path.GetTempPath(), "axiom-picture-" + Guid.NewGuid().ToString("N") + (jpeg ? ".jpg" : ".png"));
            dynamic added;
            try
            {
                List<dynamic> overlays = ParamBool(p, "overlays", false) ? Overlays(sourceSlide, picture) : new List<dynamic>();
                if (overlays.Count > 0)
                {
                    ExportRegion(source, sourceSlide, picture, overlays, temp, jpeg);
                }
                else
                {
                    // Shape.Export (ẩn trong thư viện kiểu): PNG đúng số điểm ảnh gốc, không đụng clipboard.
                    picture.Export(temp, jpeg ? 1 : 2);
                }
                added = slide.Shapes.AddPicture(temp, 0, -1, left, top, -1, -1);
            }
            finally
            {
                try
                {
                    File.Delete(temp);
                }
                catch
                {
                }
            }
            added.LockAspectRatio = -1;
            double scale = Math.Min(width / Convert.ToDouble(added.Width), height / Convert.ToDouble(added.Height));
            added.Width = Convert.ToDouble(added.Width) * scale;
            added.Left = left + (width - Convert.ToDouble(added.Width)) / 2;
            added.Top = top + (height - Convert.ToDouble(added.Height)) / 2;
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "shape", Convert.ToString(added.Name) },
                { "source", Convert.ToString(picture.Name) },
                { "box", new[] { Math.Round(Convert.ToDouble(added.Left), 1), Math.Round(Convert.ToDouble(added.Top), 1),
                    Math.Round(Convert.ToDouble(added.Width), 1), Math.Round(Convert.ToDouble(added.Height), 1) } }
            };
        }
    }
}

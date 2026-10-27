using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;

namespace AxiomOffice.Bridge
{
    internal static partial class CommandDispatcher
    {
        // PowerPoint / WPS Presentation (AppKind "wpp").
        private static IEnumerable<CommandInfo> PresentationCommands()
        {
            return new[]
            {
                Command("wpp.newPresentation", "wpp", WppNewPresentation, "Tạo bản trình chiếu mới"),
                Command("wpp.open", "wpp", WppOpen, "Mở .pptx", Req("path")),
                Command("wpp.listSlides", "wpp", WppListSlides, "Số slide + text từng slide", Opt("presentation", "full path of the presentation to work on; default: the active one"), Opt("detail", "true = kem layout/master/notes")).ForAgent(),
                Command("wpp.addSlide", "wpp", WppAddSlide, "Thêm slide cuối; `layout` mặc định 12 = trống (1 = tiêu đề, 2 = tiêu đề + nội dung, 11 = chỉ tiêu đề)",
                    Opt("layout", "number: 1 title, 2 title+content, 11 title only, 12 blank"),
                    Opt("index", "position of the new slide (1 = first); default: after the last slide")).ForAgent(),
                Command("wpp.addText", "wpp", WppAddText, "Textbox có định dạng (`color` dạng `#RRGGBB`, `align` left/center/right)",
                    Req("text"), Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height"), Opt("fontSize"), Opt("bold"), Opt("color"), Opt("align"),
                    Opt("presentation", "full path of the presentation to work on; default: the active one")).ForAgent(),
                Command("wpp.addTextBox", "wpp", WppAddTextBox, "Textbox", Req("text"), Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height")),
                Command("wpp.addImage", "wpp", WppAddImage, "Chèn ảnh (kích thước gốc nếu bỏ trống `width`/`height`)",
                    Req("path"), Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height")).ForAgent(),
                Command("wpp.addTable", "wpp", WppAddTable, "Bảng; `rows`/`cols` tự suy ra/nới theo `values`",
                    Opt("rows"), Opt("cols"), Opt("values", "2D array of rows"), Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height")).ForAgent(),
                Command("wpp.setNotes", "wpp", WppSetNotes, "Ghi chú thuyết trình", Req("text"), Opt("slide"), Opt("presentation", "full path of the presentation to work on; default: the active one")).ForAgent(),
                Command("wpp.deleteSlide", "wpp", WppDeleteSlide, "Xoá slide (mặc định slide cuối)", Opt("slide")).ForAgent(),
                Command("wpp.exportPdf", "wpp", WppExportPdf, "Xuất PDF", Req("path")).ForAgent(),
                Command("wpp.save", "wpp", (host, p) => SaveDocument(host, "wpp", null, ParamString(p, "presentation", null)), "Lưu",
                    Opt("presentation", "full path of the presentation to work on; default: the active one")).ForAgent(),
                Command("wpp.saveAs", "wpp", (host, p) => SaveDocument(host, "wpp", ParamString(p, "path", null)), "Lưu thành file mới", Req("path")).ForAgent()
            };
        }

        private static Dictionary<string, object> WppListSlides(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            int count = Convert.ToInt32(pres.Slides.Count);
            var slides = new List<object>();
            for (int i = 1; i <= count; i++)
            {
                dynamic slide = pres.Slides[i];
                var texts = new List<string>();
                int shapeCount = Convert.ToInt32(slide.Shapes.Count);
                for (int s = 1; s <= shapeCount; s++)
                {
                    try
                    {
                        AddShapeTexts(slide.Shapes[s], texts);
                    }
                    catch
                    {
                    }
                }
                var entry = new Dictionary<string, object>
                {
                    { "index", i },
                    { "shapeTexts", texts }
                };
                // Slide dựng lại từ slide nguồn số này (wpp.addSlideFromTemplate {tag}).
                if (ParamBool(p, "detail", false))
                {
                    try { entry["layout"] = System.Convert.ToString(slide.Layout); } catch { }
                    try { entry["master"] = System.Convert.ToString(slide.Master.Name); } catch { }
                    try
                    {
                        var notes = new System.Collections.Generic.List<string>();
                        foreach (dynamic shape in slide.NotesPage.Shapes)
                        {
                            try { if (System.Convert.ToInt32(shape.HasTextFrame) == -1) notes.Add(System.Convert.ToString(shape.TextFrame.TextRange.Text)); }
                            catch { }
                        }
                        if (notes.Count > 0) entry["notes"] = notes;
                    }
                    catch { }
                }
                string tag = SlideTag(slide);
                if (tag != null)
                {
                    entry["tag"] = tag;
                }
                slides.Add(entry);
            }
            return new Dictionary<string, object>
            {
                { "presentation", Convert.ToString(pres.Name) },
                { "slideCount", count },
                { "slides", slides }
            };
        }

        // Chữ của một shape cho wpp.listSlides: bảng (mỗi hàng một dòng, ô cách " | "), nhóm (đệ quy), tiêu đề biểu
        // đồ, còn lại là TextFrame. Trước 25/10/2026 chỉ đọc TextFrame nên agent không thấy bảng của chính mình và
        // chèn lại lần hai (docs/plans/19-h0 H3). Cùng định dạng với impress.py và commands.js.
        internal static void AddShapeTexts(dynamic shape, List<string> texts)
        {
            if (ComFlag(() => shape.HasTable))
            {
                dynamic table = shape.Table;
                int rows = Convert.ToInt32(table.Rows.Count);
                int cols = Convert.ToInt32(table.Columns.Count);
                var lines = new List<string> { "[table " + rows + "x" + cols + "]" };
                for (int r = 1; r <= rows; r++)
                {
                    var cells = new List<string>();
                    for (int c = 1; c <= cols; c++)
                    {
                        string cell = "";
                        try
                        {
                            cell = Convert.ToString(table.Cell(r, c).Shape.TextFrame.TextRange.Text);
                        }
                        catch
                        {
                        }
                        cells.Add(cell.Replace("\r", " ").Replace("\n", " ").Trim());
                    }
                    lines.Add(string.Join(" | ", cells));
                }
                texts.Add(string.Join("\n", lines));
                return;
            }
            if (ComFlag(() => Convert.ToInt32(shape.Type) == 6))
            {
                dynamic items = shape.GroupItems;
                int count = Convert.ToInt32(items.Count);
                for (int i = 1; i <= count; i++)
                {
                    try
                    {
                        AddShapeTexts(items.Item(i), texts);
                    }
                    catch
                    {
                    }
                }
                return;
            }
            if (ComFlag(() => shape.HasChart))
            {
                texts.Add(ChartText(shape.Chart));
                return;
            }
            if (Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.TextFrame.HasText) == -1)
            {
                texts.Add(Convert.ToString(shape.TextFrame.TextRange.Text));
            }
        }

        // Cờ COM (MsoTriState -1 / bool) đọc an toàn: shape nào không có thuộc tính thì coi là false, không bỏ cả shape.
        private static bool ComFlag(Func<object> read)
        {
            try
            {
                object value = read();
                return value is bool flag ? flag : Convert.ToInt32(value) == -1;
            }
            catch
            {
                return false;
            }
        }

        // Số COM đọc an toàn: không có thuộc tính thì 0.
        private static double ComDouble(Func<object> read)
        {
            try
            {
                return Convert.ToDouble(read());
            }
            catch
            {
                return 0;
            }
        }

        private static Dictionary<string, object> WppNewPresentation(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = app.Presentations.Add();
            TrackPresentation(pres);
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(pres.Name) }
            };
        }

        private static Dictionary<string, object> WppOpen(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            // Đang mở rồi thì chỉ đưa cửa sổ của nó lên (dựng theo mẫu nhiều lô quay lại bài mới giữa chừng).
            dynamic open = FindPresentation(app, path);
            if (open != null)
            {
                // Activate() có lúc trả về ngay mà ActivePresentation vẫn là bài khác (flake 28/10/2026: reply
                // alreadyOpen nhưng lượt sau sửa nhầm from-template.pptx). Chờ tới khi PowerPoint đổi thật.
                string wanted = Convert.ToString(open.FullName) ?? "";
                for (int attempt = 0; attempt < 50; attempt++)
                {
                    // Bài có thể có nhiều cửa sổ (document + slide show + notes): Activate() window 1 không
                    // chuyển ActivePresentation nếu đó không phải cửa sổ tài liệu. Kích hoạt mọi cửa sổ tài liệu.
                    try
                    {
                        int windows = Convert.ToInt32(open.Windows.Count);
                        for (int w = 1; w <= windows; w++)
                        {
                            dynamic window = open.Windows[w];
                            int type = 1;
                            try { type = Convert.ToInt32(window.Type); } catch (Exception) { }
                            if (type == 1) // ppWindowDocument
                            {
                                window.Activate();
                            }
                        }
                    }
                    catch (Exception) { }
                    try
                    {
                        dynamic active = app.ActivePresentation;
                        if (active != null && string.Equals(Convert.ToString(active.FullName) ?? "", wanted,
                                StringComparison.OrdinalIgnoreCase))
                        {
                            break;
                        }
                    }
                    catch (Exception)
                    {
                    }
                    System.Threading.Thread.Sleep(100);
                }
                TrackPresentation(open);
                return new Dictionary<string, object>
                {
                    { "name", Convert.ToString(open.Name) },
                    { "fullName", Convert.ToString(open.FullName) },
                    { "alreadyOpen", true }
                };
            }
            dynamic pres = app.Presentations.Open(path);
            TrackPresentation(pres);
            return new Dictionary<string, object>
            {
                { "name", Convert.ToString(pres.Name) },
                { "fullName", Convert.ToString(pres.FullName) }
            };
        }

        private static dynamic WppGetSlide(dynamic pres, int slideIndex)
        {
            int count = Convert.ToInt32(pres.Slides.Count);
            if (count == 0)
            {
                throw new InvalidOperationException("presentation has no slides; call wpp.addSlide first");
            }
            if (slideIndex < 1 || slideIndex > count)
            {
                slideIndex = count;
            }
            return pres.Slides[slideIndex];
        }

        private static Dictionary<string, object> WppAddSlide(IAppHost host, Dictionary<string, object> p)
        {
            int layout = ParamInt(p, "layout", 12);
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            int count = Convert.ToInt32(pres.Slides.Count);
            // index: chen vao vi tri (thay dung cho mot slide da xoa - slides_from_brief "replace"); ngoai 1..count+1 thi cuoi bai.
            int index = ParamInt(p, "index", 0);
            if (index < 1 || index > count + 1)
            {
                index = count + 1;
            }
            pres.Slides.Add(index, layout);
            return new Dictionary<string, object> { { "slide", index }, { "layout", layout } };
        }

        private static Dictionary<string, object> WppAddText(IAppHost host, Dictionary<string, object> p)
        {
            string text = ParamString(p, "text", "");
            int left = ParamInt(p, "left", 60);
            int top = ParamInt(p, "top", 60);
            int width = ParamInt(p, "width", 540);
            int height = ParamInt(p, "height", 120);
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic shape = slide.Shapes.AddTextbox(1, left, top, width, height);
            // Hộp giữ đúng chiều cao được đặt (AddTextbox mặc định tự giãn theo chữ): chữ dài thì checkLayout báo
            // `overflow`, thay vì hộp lặng lẽ dài xuống đè hộp dưới - tiêu đề 3 dòng đè nội dung 9pt mà không ai
            // báo (đo trên PowerPoint 16, 13/10/2026). Hộp chữ không viền, không nền nên nhìn không khác.
            bool fixedHeight = false;
            try
            {
                shape.TextFrame.AutoSize = 0;
                fixedHeight = true;
            }
            catch (Exception)
            {
            }
            dynamic textRange = shape.TextFrame.TextRange;
            textRange.Text = text;
            dynamic font = textRange.Font;
            if (HasParam(p, "fontSize"))
            {
                font.Size = ParamInt(p, "fontSize", 18);
            }
            if (HasParam(p, "bold"))
            {
                font.Bold = ParamBool(p, "bold", false) ? -1 : 0;
            }
            int? color = ParseBgrColor(ParamString(p, "color", null));
            if (color.HasValue)
            {
                font.Color = color.Value;
            }
            if (HasParam(p, "align"))
            {
                string align = ParamString(p, "align", "left").ToLowerInvariant();
                textRange.ParagraphFormat.Alignment = align == "center" ? 2 : (align == "right" ? 3 : 1);
            }
            if (fixedHeight)
            {
                // Đề phòng bản Office/WPS đã co giãn hộp trước khi tắt tự giãn: trả lại đúng chiều cao đã đặt.
                shape.Height = height;
            }
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "shape", Convert.ToString(shape.Name) }
            };
        }

        private static Dictionary<string, object> WppAddImage(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            int left = ParamInt(p, "left", 60);
            int top = ParamInt(p, "top", 60);
            int width = ParamInt(p, "width", -1);
            int height = ParamInt(p, "height", -1);
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            // Chỉ một chiều thì co theo tỉ lệ ảnh: AddPicture coi -1 là "giữ cỡ gốc" cho TỪNG chiều, nên
            // truyền width=491, height=-1 làm ảnh 16:9 thành 491x445.5 (đo trên PowerPoint 16, 13/10/2026).
            bool keepAspect = (width > 0) != (height > 0);
            dynamic shape = slide.Shapes.AddPicture(path, 0, -1, left, top, keepAspect ? -1 : width, keepAspect ? -1 : height);
            if (keepAspect)
            {
                shape.LockAspectRatio = -1;
                if (width > 0)
                {
                    shape.Width = width;
                }
                else
                {
                    shape.Height = height;
                }
            }
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "shape", Convert.ToString(shape.Name) }
            };
        }

        private static Dictionary<string, object> WppAddTable(IAppHost host, Dictionary<string, object> p)
        {
            List<IList> valueRows = ParamMatrix(p, "values", false);
            int rows = ParamInt(p, "rows", 0);
            int cols = ParamInt(p, "cols", 0);
            if (valueRows != null && valueRows.Count > 0)
            {
                // Bảng phải chứa đủ dữ liệu: suy ra/nới rows, cols theo values thay vì cắt bớt.
                rows = Math.Max(rows, valueRows.Count);
                cols = Math.Max(cols, valueRows.Max(row => row.Count));
            }
            if (rows <= 0 || cols <= 0)
            {
                throw new ArgumentException("'rows' and 'cols' are required (or pass 'values' as a 2D array to size the table)");
            }
            int left = ParamInt(p, "left", 60);
            int top = ParamInt(p, "top", 120);
            int width = ParamInt(p, "width", 600);
            int height = ParamInt(p, "height", 200);
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic tableShape = slide.Shapes.AddTable(rows, cols, left, top, width, height);
            dynamic table = tableShape.Table;
            int filled = 0;
            if (valueRows != null)
            {
                for (int r = 0; r < valueRows.Count && r < rows; r++)
                {
                    IList row = valueRows[r] as IList;
                    if (row == null)
                    {
                        continue;
                    }
                    for (int c = 0; c < row.Count && c < cols; c++)
                    {
                        table.Cell(r + 1, c + 1).Shape.TextFrame.TextRange.Text = Convert.ToString(row[c]);
                        filled++;
                    }
                }
            }
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "rows", rows },
                { "cols", cols },
                { "filled", filled },
                // Tên shape: Core ghi sổ đối tượng của lượt để xoá lại không phải hỏi (docs/plans/19-h0 H4).
                { "shape", Convert.ToString(tableShape.Name) }
            };
        }

        private static Dictionary<string, object> WppSetNotes(IAppHost host, Dictionary<string, object> p)
        {
            string text = ParamString(p, "text", "");
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic notesPage = slide.NotesPage;
            dynamic target = null;
            int shapeCount = Convert.ToInt32(notesPage.Shapes.Count);
            for (int i = 1; i <= shapeCount; i++)
            {
                dynamic shape = notesPage.Shapes[i];
                try
                {
                    if (Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.PlaceholderFormat.Type) == 2)
                    {
                        target = shape;
                        break;
                    }
                }
                catch
                {
                }
            }
            if (target == null)
            {
                for (int i = 1; i <= shapeCount; i++)
                {
                    dynamic shape = notesPage.Shapes[i];
                    try
                    {
                        if (Convert.ToInt32(shape.HasTextFrame) == -1)
                        {
                            target = shape;
                            break;
                        }
                    }
                    catch
                    {
                    }
                }
            }
            if (target == null)
            {
                throw new InvalidOperationException("notes text placeholder not found");
            }
            target.TextFrame.TextRange.Text = text;
            return new Dictionary<string, object> { { "slide", Convert.ToInt32(slide.SlideIndex) }, { "notes", text.Length } };
        }

        private static Dictionary<string, object> WppDeleteSlide(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            int index = Convert.ToInt32(slide.SlideIndex);
            slide.Delete();
            return new Dictionary<string, object>
            {
                { "deleted", index },
                { "slideCount", Convert.ToInt32(pres.Slides.Count) }
            };
        }

        private static Dictionary<string, object> WppExportPdf(IAppHost host, Dictionary<string, object> p)
        {
            string path = ParamString(p, "path", null);
            if (string.IsNullOrEmpty(path))
            {
                throw new InvalidOperationException("'path' is required");
            }
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            try
            {
                pres.ExportAsFixedFormat(path, 2);
            }
            catch
            {
                pres.SaveAs(path, 32);
            }
            return new Dictionary<string, object> { { "exported", path } };
        }

        private static Dictionary<string, object> WppAddTextBox(IAppHost host, Dictionary<string, object> p)
        {
            string text = ParamString(p, "text", null);
            int slideIndex = ParamInt(p, "slide", 0);
            int left = ParamInt(p, "left", 60);
            int top = ParamInt(p, "top", 60);
            int width = ParamInt(p, "width", 540);
            int height = ParamInt(p, "height", 120);
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            int count = Convert.ToInt32(pres.Slides.Count);
            if (count == 0)
            {
                throw new InvalidOperationException("presentation has no slides; call wpp.addSlide first");
            }
            if (slideIndex < 1 || slideIndex > count)
            {
                slideIndex = count;
            }
            dynamic slide = pres.Slides[slideIndex];
            dynamic shape = slide.Shapes.AddTextbox(1, left, top, width, height);
            if (text != null)
            {
                shape.TextFrame.TextRange.Text = text;
            }
            return new Dictionary<string, object>
            {
                { "slide", slideIndex },
                { "shape", Convert.ToString(shape.Name) }
            };
        }
    }
}

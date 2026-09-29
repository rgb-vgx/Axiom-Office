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
                Command("wpp.listSlides", "wpp", WppListSlides, "Số slide + text từng slide").ForAgent(),
                Command("wpp.addSlide", "wpp", WppAddSlide, "Thêm slide cuối; `layout` mặc định 12 = trống (1 = tiêu đề, 2 = tiêu đề + nội dung, 11 = chỉ tiêu đề)",
                    Opt("layout", "number: 1 title, 2 title+content, 11 title only, 12 blank")).ForAgent(),
                Command("wpp.addText", "wpp", WppAddText, "Textbox có định dạng (`color` dạng `#RRGGBB`, `align` left/center/right)",
                    Req("text"), Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height"), Opt("fontSize"), Opt("bold"), Opt("color"), Opt("align")).ForAgent(),
                Command("wpp.addTextBox", "wpp", WppAddTextBox, "Textbox", Req("text"), Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height")),
                Command("wpp.addImage", "wpp", WppAddImage, "Chèn ảnh (kích thước gốc nếu bỏ trống `width`/`height`)",
                    Req("path"), Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height")).ForAgent(),
                Command("wpp.addTable", "wpp", WppAddTable, "Bảng; `rows`/`cols` tự suy ra/nới theo `values`",
                    Opt("rows"), Opt("cols"), Opt("values", "2D array of rows"), Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height")).ForAgent(),
                Command("wpp.setNotes", "wpp", WppSetNotes, "Ghi chú thuyết trình", Req("text"), Opt("slide")).ForAgent(),
                Command("wpp.deleteSlide", "wpp", WppDeleteSlide, "Xoá slide (mặc định slide cuối)", Opt("slide")).ForAgent(),
                Command("wpp.exportPdf", "wpp", WppExportPdf, "Xuất PDF", Req("path")).ForAgent(),
                Command("wpp.save", "wpp", (host, p) => SaveDocument(host, "wpp", null), "Lưu").ForAgent(),
                Command("wpp.saveAs", "wpp", (host, p) => SaveDocument(host, "wpp", ParamString(p, "path", null)), "Lưu thành file mới", Req("path")).ForAgent()
            };
        }

        private static Dictionary<string, object> WppListSlides(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
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
                        dynamic shape = slide.Shapes[s];
                        if (Convert.ToInt32(shape.HasTextFrame) == -1 && Convert.ToInt32(shape.TextFrame.HasText) == -1)
                        {
                            texts.Add(Convert.ToString(shape.TextFrame.TextRange.Text));
                        }
                    }
                    catch
                    {
                    }
                }
                slides.Add(new Dictionary<string, object>
                {
                    { "index", i },
                    { "shapeTexts", texts }
                });
            }
            return new Dictionary<string, object>
            {
                { "presentation", Convert.ToString(pres.Name) },
                { "slideCount", count },
                { "slides", slides }
            };
        }

        private static Dictionary<string, object> WppNewPresentation(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = app.Presentations.Add();
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
            dynamic pres = app.Presentations.Open(path);
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
            dynamic pres = app.ActivePresentation;
            int index = Convert.ToInt32(pres.Slides.Count) + 1;
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
            dynamic pres = app.ActivePresentation;
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic shape = slide.Shapes.AddTextbox(1, left, top, width, height);
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
            dynamic pres = app.ActivePresentation;
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic shape = slide.Shapes.AddPicture(path, 0, -1, left, top, width, height);
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
            dynamic pres = app.ActivePresentation;
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
                { "filled", filled }
            };
        }

        private static Dictionary<string, object> WppSetNotes(IAppHost host, Dictionary<string, object> p)
        {
            string text = ParamString(p, "text", "");
            dynamic app = host.Application;
            dynamic pres = app.ActivePresentation;
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
            dynamic pres = app.ActivePresentation;
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
            dynamic pres = app.ActivePresentation;
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
            dynamic pres = app.ActivePresentation;
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

using System;
using System.Collections.Generic;

namespace AxiomOffice.Bridge
{
    // Hình khối và đường nối cho slide (docs/plans/18-s0 P2): primitive thuần - vẽ một hình hoặc một đường nối, không
    // có bố cục (vị trí do Core tính ở internal/slidelayout - docs/plans/19-h0 H6). Chỉ tool slides_from_brief gọi;
    // model không thấy (không ForAgent). Mọi đối tượng là shape gốc của PowerPoint, sửa được bằng tay.
    internal static partial class CommandDispatcher
    {
        private static IEnumerable<CommandInfo> ShapeCommands()
        {
            return new[]
            {
                Command("wpp.addShape", "wpp", WppAddShape,
                    "Hình khối có chữ (chữ nhật, bo góc, tròn, mũi tên chevron) - nền, viền, cỡ chữ, màu chữ",
                    Opt("slide"), Opt("left"), Opt("top"), Opt("width"), Opt("height"),
                    Opt("shape", "rect, rounded, oval, chevron, line (default rect)"), Opt("text"),
                    Opt("fontSize"), Opt("bold"), Opt("boldFirst", "true = only the first line bold (heading of the box)"), Opt("color", "#RRGGBB text"), Opt("fill", "#RRGGBB fill; empty = none"),
                    Opt("line", "#RRGGBB outline; empty = none"), Opt("font", "font name"), Opt("anchor", "top or middle (default)"),
                    Opt("presentation", "full path of the presentation to work on; default: the active one")),
                Command("wpp.renderSlide", "wpp", WppRenderSlide,
                    "Ảnh PNG của một slide theo số (soát bằng mắt, benchmark) - base64, không đổi cửa sổ đang xem",
                    Req("slide"), Opt("maxWidth", "pixels (default 1280)"),
                    Opt("presentation", "full path of the presentation to work on; default: the active one")),
                Command("wpp.addConnector", "wpp", WppAddConnector,
                    "Đường nối có mũi tên giữa hai hình (bám theo hình khi kéo)",
                    Req("from", "shape name"), Req("to", "shape name"), Opt("slide"),
                    Opt("elbow", "true = elbow connector (default straight)"), Opt("color", "#RRGGBB"), Opt("weight", "points"),
                    Opt("presentation", "full path of the presentation to work on; default: the active one"))
            };
        }

        // Slide bất kỳ (app.screenshot chỉ chụp slide đang hiện) - docs/plans/18-s0 P3.
        private static Dictionary<string, object> WppRenderSlide(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            int count = Convert.ToInt32(pres.Slides.Count);
            int index = ParamInt(p, "slide", 0);
            if (index < 1 || index > count)
            {
                throw new ArgumentException("'slide' must be between 1 and " + count + ", got " + index);
            }
            return ExportSlide(pres, index, count, Math.Max(320, Math.Min(ParamInt(p, "maxWidth", 1280), 2560)));
        }

        private static Dictionary<string, object> WppAddShape(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            // MsoAutoShapeType: 1 chữ nhật, 5 bo góc, 9 elip, 52 chevron.
            string kind = ParamString(p, "shape", "rect");
            int type;
            switch (kind)
            {
                case "rounded": type = 5; break;
                case "oval": type = 9; break;
                case "chevron": type = 52; break;
                case "rect":
                case "line": type = 1; break;
                default: throw new ArgumentException("'shape' must be rect, rounded, oval, chevron or line, got " + kind);
            }
            dynamic shape = slide.Shapes.AddShape(type, ParamDouble(p, "left", 40), ParamDouble(p, "top", 130),
                ParamDouble(p, "width", 200), ParamDouble(p, "height", 80));
            int? fill = ParseBgrColor(ParamString(p, "fill", null));
            if (fill.HasValue)
            {
                shape.Fill.Visible = -1;
                shape.Fill.Solid();
                shape.Fill.ForeColor.RGB = fill.Value;
            }
            else
            {
                shape.Fill.Visible = 0;
            }
            int? line = ParseBgrColor(ParamString(p, "line", null));
            if (line.HasValue)
            {
                shape.Line.Visible = -1;
                shape.Line.ForeColor.RGB = line.Value;
                shape.Line.Weight = 1.25f;
            }
            else
            {
                shape.Line.Visible = 0;
            }
            string text = ParamString(p, "text", null);
            if (!string.IsNullOrEmpty(text))
            {
                dynamic frame = shape.TextFrame;
                frame.WordWrap = -1;
                frame.AutoSize = 0;
                frame.MarginLeft = 6;
                frame.MarginRight = 6;
                frame.VerticalAnchor = string.Equals(ParamString(p, "anchor", "middle"), "top", StringComparison.OrdinalIgnoreCase) ? 1 : 3;
                dynamic range = frame.TextRange;
                // "\n" của PowerPoint là ngắt dòng TRONG đoạn: boldFirst sẽ đậm cả khối. Mỗi dòng một đoạn ("\r").
                range.Text = text.Replace("\r\n", "\r").Replace("\n", "\r");
                range.ParagraphFormat.Alignment = 2; // giữa
                range.Font.Size = (float)ParamDouble(p, "fontSize", 18);
                range.Font.Bold = ParamBool(p, "bold", false) ? -1 : 0;
                if (ParamBool(p, "boldFirst", false))
                {
                    range.Paragraphs(1).Font.Bold = -1;
                }
                string font = ParamString(p, "font", null);
                if (!string.IsNullOrWhiteSpace(font))
                {
                    range.Font.Name = font;
                }
                int? color = ParseBgrColor(ParamString(p, "color", "#262626"));
                if (color.HasValue)
                {
                    range.Font.Color.RGB = color.Value;
                }
            }
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "shape", Convert.ToString(shape.Name) }
            };
        }

        private static Dictionary<string, object> WppAddConnector(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            dynamic slide = WppGetSlide(pres, ParamInt(p, "slide", 0));
            dynamic from = ConnectEnd(slide, ParamString(p, "from", null));
            dynamic to = ConnectEnd(slide, ParamString(p, "to", null));
            // MsoConnectorType: 1 thẳng, 2 gấp khúc. Nối vào điểm 1 rồi RerouteConnections chọn điểm gần nhất.
            dynamic connector = slide.Shapes.AddConnector(ParamBool(p, "elbow", false) ? 2 : 1, 0, 0, 10, 10);
            connector.ConnectorFormat.BeginConnect(from, 1);
            connector.ConnectorFormat.EndConnect(to, 1);
            connector.RerouteConnections();
            connector.Line.EndArrowheadStyle = 2; // msoArrowheadTriangle
            connector.Line.Weight = (float)ParamDouble(p, "weight", 1.75);
            int? color = ParseBgrColor(ParamString(p, "color", "#595959"));
            if (color.HasValue)
            {
                connector.Line.ForeColor.RGB = color.Value;
            }
            return new Dictionary<string, object>
            {
                { "slide", Convert.ToInt32(slide.SlideIndex) },
                { "shape", Convert.ToString(connector.Name) }
            };
        }

        private static dynamic ConnectEnd(dynamic slide, string name)
        {
            if (string.IsNullOrWhiteSpace(name))
            {
                throw new ArgumentException("'from' and 'to' are required (shape names returned by wpp.addShape)");
            }
            dynamic shape = FindShape(slide, name);
            if (shape == null)
            {
                throw new ArgumentException("no shape named '" + name + "' on slide " + Convert.ToInt32(slide.SlideIndex));
            }
            return shape;
        }
    }
}

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
                    Opt("elbow", "true = elbow connector (default straight)"),
                    Opt("direction", "down, up, right or left: leave 'from' on that side and enter 'to' on the opposite side; default: nearest sides"),
                    Opt("fromSide", "top, bottom, left or right side of 'from' (overrides direction)"),
                    Opt("toSide", "top, bottom, left or right side of 'to' (overrides direction); same side as fromSide + elbow = a loop around the row"),
                    Opt("offset", "loop only: how far (pt) the loop goes beyond the farther end (default: PowerPoint's choice)"),
                    Opt("color", "#RRGGBB"), Opt("weight", "points"),
                    Opt("presentation", "full path of the presentation to work on; default: the active one")),
                ChartCommand()
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
            // MsoConnectorType: 1 thẳng, 2 gấp khúc. Có hướng thì nối đúng cạnh (đáy cha -> đỉnh con): RerouteConnections
            // chọn "điểm gần nhất" có thể cho đường gấp khúc cắt ngang hộp bên cạnh (benchmark s17, docs/plans/18-s0 9.3).
            string direction = ParamString(p, "direction", null);
            string fromSide = ParamString(p, "fromSide", null), toSide = ParamString(p, "toSide", null);
            int begin = string.IsNullOrWhiteSpace(fromSide) ? ConnectSite(from, direction, false) : SideSite(from, fromSide, "fromSide");
            int end = string.IsNullOrWhiteSpace(toSide) ? ConnectSite(to, direction, true) : SideSite(to, toSide, "toSide");
            dynamic connector = slide.Shapes.AddConnector(ParamBool(p, "elbow", false) ? 2 : 1, 0, 0, 10, 10);
            connector.ConnectorFormat.BeginConnect(from, begin > 0 ? begin : 1);
            connector.ConnectorFormat.EndConnect(to, end > 0 ? end : 1);
            if (begin == 0 || end == 0)
            {
                connector.RerouteConnections();
            }
            // Đường vòng (cùng một cạnh ở hai đầu, gấp khúc): đặt độ sâu để hai đường vòng của cùng hàng không trùng nhau.
            double offset = ParamDouble(p, "offset", 0);
            if (offset > 0 && ParamBool(p, "elbow", false) && !string.IsNullOrWhiteSpace(fromSide)
                && string.Equals(fromSide.Trim(), (toSide ?? "").Trim(), StringComparison.OrdinalIgnoreCase))
            {
                SetLoopOffset(connector, fromSide.Trim().ToLowerInvariant(), offset);
            }
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

        // Điểm nối của hình theo cạnh. Hình chữ nhật, bo góc, elip có 4k điểm đều nhau, đánh số ngược chiều kim đồng hồ
        // từ đỉnh: đỉnh 1, trái 1+k, đáy 1+2k, phải 1+3k. Hình khác hoặc không có hướng: 0 (để PowerPoint tự chọn).
        private static int ConnectSite(dynamic shape, string direction, bool entering)
        {
            if (string.IsNullOrWhiteSpace(direction))
            {
                return 0;
            }
            int side;
            switch (direction.Trim().ToLowerInvariant())
            {
                case "down": side = entering ? 0 : 2; break;
                case "up": side = entering ? 2 : 0; break;
                case "right": side = entering ? 1 : 3; break;
                case "left": side = entering ? 3 : 1; break;
                default: throw new ArgumentException("'direction' must be down, up, right or left, got " + direction);
            }
            int count = Convert.ToInt32(shape.ConnectionSiteCount);
            return count > 0 && count % 4 == 0 ? 1 + side * count / 4 : 0;
        }

        // Điểm nối ở giữa một cạnh (top/left/bottom/right) - cùng cách đánh số với ConnectSite.
        private static int SideSite(dynamic shape, string side, string param)
        {
            int index;
            switch (side.Trim().ToLowerInvariant())
            {
                case "top": index = 0; break;
                case "left": index = 1; break;
                case "bottom": index = 2; break;
                case "right": index = 3; break;
                default: throw new ArgumentException("'" + param + "' must be top, bottom, left or right, got " + side);
            }
            int count = Convert.ToInt32(shape.ConnectionSiteCount);
            return count > 0 && count % 4 == 0 ? 1 + index * count / 4 : 0;
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

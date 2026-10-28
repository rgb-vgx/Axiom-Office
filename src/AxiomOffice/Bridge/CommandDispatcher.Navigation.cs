using System;
using System.Collections.Generic;

namespace AxiomOffice.Bridge
{
    // Nhảy tới chỗ được trích dẫn trong câu trả lời ([§12] = đoạn 12 theo writer.outline, [Data!C42], [slide 3]): pane gọi
    // khi người dùng bấm vào trích dẫn (docs/research/tinh-nang-con-thieu-so-voi-claude-m365.md W1). Chỉ chọn + cuộn, không
    // sửa tài liệu, không để lại bước hoàn tác. Không cho agent (agent=false): model chỉ viết trích dẫn, không cần lệnh này
    // trong mô tả tool.
    internal static partial class CommandDispatcher
    {
        private static IEnumerable<CommandInfo> NavigationCommands()
        {
            return new[]
            {
                Command("writer.goto", "wps", WriterGoto,
                    "Chọn và cuộn tới một đoạn theo số của writer.outline (trích dẫn [§n] trong câu trả lời)",
                    Req("paragraph", "paragraph number from writer.outline"), Opt("text", "first words of the paragraph, to find it if numbering moved")),
                Command("et.goto", "et", EtGoto,
                    "Chọn và cuộn tới một ô/vùng (trích dẫn [Sheet!A1] trong câu trả lời)",
                    Req("range", "e.g. 'C42' or 'Data!C42'"), Opt("sheet")),
                Command("wpp.goto", "wpp", WppGoto,
                    "Chuyển tới một slide (trích dẫn [slide n] trong câu trả lời)",
                    Req("slide", "slide number")),
            };
        }

        private static Dictionary<string, object> WriterGoto(IAppHost host, Dictionary<string, object> p)
        {
            int index = ParamInt(p, "paragraph", 0);
            string hint = (ParamString(p, "text", null) ?? "").Trim();
            dynamic app = host.Application;
            dynamic doc = app.ActiveDocument;
            dynamic paragraphs = doc.Paragraphs;
            int count = Convert.ToInt32(paragraphs.Count);
            if (index < 1 || index > count)
            {
                throw new ArgumentException("'paragraph' must be between 1 and " + count);
            }
            // Dò theo chữ đầu đoạn và đọc chữ trả về trên bản CUỐI (ẩn revision) - cùng bản agent thấy ở writer.outline.
            string text;
            using (new HiddenRevisions(doc))
            {
                dynamic range = paragraphs.Item(index).Range;
                // Tài liệu đã đổi sau câu trả lời (thêm/xoá đoạn): số đoạn lệch. Có chữ đầu đoạn thì dò quanh đó (±40 đoạn).
                if (hint.Length > 0 && !StartsWithHint(Convert.ToString(range.Text), hint))
                {
                    for (int offset = 1; offset <= 40; offset++)
                    {
                        foreach (int candidate in new[] { index - offset, index + offset })
                        {
                            if (candidate < 1 || candidate > count)
                            {
                                continue;
                            }
                            dynamic other = paragraphs.Item(candidate).Range;
                            if (StartsWithHint(Convert.ToString(other.Text), hint))
                            {
                                range = other;
                                index = candidate;
                                offset = 41;
                                break;
                            }
                        }
                    }
                }
                range.Select();
                try
                {
                    app.ActiveWindow.ScrollIntoView(range, true);
                }
                catch
                {
                }
                text = (Convert.ToString(range.Text) ?? "").TrimEnd('\r', '\a').Trim();
            }
            return new Dictionary<string, object> { { "paragraph", index }, { "text", Shorten(text, 120) } };
        }

        private static bool StartsWithHint(string paragraphText, string hint)
        {
            string text = (paragraphText ?? "").Trim();
            string probe = hint.Length > 30 ? hint.Substring(0, 30) : hint;
            return text.StartsWith(probe, StringComparison.CurrentCultureIgnoreCase);
        }

        private static Dictionary<string, object> EtGoto(IAppHost host, Dictionary<string, object> p)
        {
            string address = ParamString(p, "range", null);
            if (string.IsNullOrWhiteSpace(address))
            {
                throw new ArgumentException("'range' is required, e.g. 'C42' or 'Data!C42'");
            }
            dynamic app = host.Application;
            dynamic wb = EnsureWorkbook(app);
            dynamic sheet;
            dynamic range = SheetRange(wb, ParamString(p, "sheet", null), address.Trim(), "'range'", out sheet);
            sheet.Activate();
            try
            {
                app.Goto(range, true);
            }
            catch
            {
                range.Select();
            }
            return new Dictionary<string, object>
            {
                { "sheet", Convert.ToString(sheet.Name) }, { "range", Convert.ToString(range.Address(false, false)) }
            };
        }

        private static Dictionary<string, object> WppGoto(IAppHost host, Dictionary<string, object> p)
        {
            int slide = ParamInt(p, "slide", 0);
            dynamic app = host.Application;
            dynamic pres = TargetPresentation(app, p);
            int count = Convert.ToInt32(pres.Slides.Count);
            if (slide < 1 || slide > count)
            {
                throw new ArgumentException("'slide' must be between 1 and " + count);
            }
            app.ActiveWindow.View.GotoSlide(slide);
            return new Dictionary<string, object> { { "slide", slide } };
        }
    }
}

using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Runtime.InteropServices;
using System.Text.RegularExpressions;
using System.Windows.Forms;

namespace WpsAiBridge.Ai
{
    // Design tokens của Ask AI pane. Số đo là px ở 96 DPI; dùng Px() khi đặt kích thước.
    internal static class PaneTheme
    {
        // Surfaces
        public static readonly Color PaneBg = Hex(0xFFFFFF);
        public static readonly Color Divider = Hex(0xE5E7EB);
        // Text
        public static readonly Color TextPrimary = Hex(0x111827);
        public static readonly Color TextSecondary = Hex(0x4B5563);
        public static readonly Color TextMuted = Hex(0x6B7280);
        // Bubbles
        public static readonly Color AiBg = Hex(0xF9FAFB);
        public static readonly Color AiBorder = Hex(0xE5E7EB);
        public static readonly Color UserBg = Hex(0xE0E7FF);
        public static readonly Color UserFg = Hex(0x1E1B4B);
        // Accent / action. Accent (#6366F1) chỉ để trang trí: trắng trên nó chỉ đạt 4.47:1.
        public static readonly Color Accent = Hex(0x6366F1);
        public static readonly Color AccentFg = Hex(0x4F46E5);
        public static readonly Color ActionBg = Hex(0x4F46E5);
        public static readonly Color ActionHover = Hex(0x4338CA);
        public static readonly Color ActionPressed = Hex(0x3730A3);
        public static readonly Color OnAction = Hex(0xFFFFFF);
        public static readonly Color DisabledBg = Hex(0xF3F4F6);
        public static readonly Color DisabledFg = Hex(0x9CA3AF);
        public static readonly Color InputBorder = Hex(0x8C93A0);
        public static readonly Color InputDisabled = Hex(0xF9FAFB);
        public static readonly Color Focus = Hex(0x4F46E5);
        // Chip
        public static readonly Color ChipBorder = Hex(0xC7D2FE);
        public static readonly Color ChipHover = Hex(0xEEF2FF);
        public static readonly Color ChipPressed = Hex(0xE0E7FF);
        public static readonly Color ChipFg = Hex(0x4338CA);
        // Status
        public static readonly Color Danger = Hex(0xDC2626);
        public static readonly Color DangerFg = Hex(0xB91C1C);
        public static readonly Color DangerBg = Hex(0xFEF2F2);
        public static readonly Color DangerBorder = Hex(0xFECACA);
        public static readonly Color Success = Hex(0x047857);

        // Fonts: tạo một lần, không dispose (sống cùng AppDomain).
        public static readonly Font Title = new Font("Segoe UI Semibold", 10f);
        public static readonly Font EmptyTitle = new Font("Segoe UI Semibold", 11f);
        public static readonly Font Body = new Font("Segoe UI", 9.5f);
        public static readonly Font Small = new Font("Segoe UI", 9f);
        public static readonly Font SmallBold = new Font("Segoe UI Semibold", 9f);
        public static readonly Font Tool = new Font("Segoe UI", 8.5f);
        public static readonly Font ToolId = new Font("Consolas", 8f);
        public static readonly Font Caption = new Font("Segoe UI", 8.25f);
        public static readonly Font CaptionBold = new Font("Segoe UI Semibold", 8.25f);

        public const int PadX = 12;
        public const int GapMessage = 12;
        public const int GapBeforeTools = 8;
        public const int GapToolLine = 2;
        public const int GapChip = 6;
        public const int RadiusBubble = 10;
        public const int RadiusTail = 4;
        public const int RadiusControl = 8;
        public const int RadiusComposer = 10;
        public const int BubblePadX = 12;
        public const int BubblePadY = 8;
        public const float BubbleMaxUser = 0.80f;
        public const float BubbleMaxAi = 0.92f;
        public const int HeaderH = 48;
        public const int FooterH = 28;
        public const int ComposerH = 98;
        public const int ComposerMaxH = 158;
        public const int ComposerTextMinH = 36;
        public const int AskW = 64;
        public const int AskH = 30;
        public const int ChipH = 32;
        public const int FocusStroke = 2;
        public const int FocusGap = 2;

        public const TextFormatFlags WrapFlags = TextFormatFlags.WordBreak | TextFormatFlags.NoPrefix | TextFormatFlags.TextBoxControl;
        public const TextFormatFlags LineFlags = TextFormatFlags.SingleLine | TextFormatFlags.NoPrefix | TextFormatFlags.EndEllipsis | TextFormatFlags.VerticalCenter;

        private static readonly float DpiScale = DetectScale();

        public static int Px(int value)
        {
            return (int)Math.Round(value * DpiScale);
        }

        public static float PxF(float value)
        {
            return value * DpiScale;
        }

        // Tắt animation khi người dùng tắt hiệu ứng hệ thống hoặc bật High Contrast.
        public static bool AnimationsEnabled
        {
            get
            {
                if (SystemInformation.HighContrast)
                {
                    return false;
                }
                try
                {
                    bool enabled = true;
                    if (SystemParametersInfo(0x1042, 0, ref enabled, 0))
                    {
                        return enabled;
                    }
                }
                catch
                {
                }
                return true;
            }
        }

        public static GraphicsPath RoundedPath(RectangleF rect, float radius)
        {
            return RoundedPath(rect, radius, radius, radius, radius);
        }

        // Bo từng góc riêng (tl, tr, br, bl) để vẽ bubble có "đuôi".
        public static GraphicsPath RoundedPath(RectangleF rect, float tl, float tr, float br, float bl)
        {
            float max = Math.Min(rect.Width, rect.Height) / 2f;
            tl = Clamp(tl, max);
            tr = Clamp(tr, max);
            br = Clamp(br, max);
            bl = Clamp(bl, max);
            var path = new GraphicsPath();
            path.AddArc(rect.X, rect.Y, tl * 2, tl * 2, 180, 90);
            path.AddArc(rect.Right - tr * 2, rect.Y, tr * 2, tr * 2, 270, 90);
            path.AddArc(rect.Right - br * 2, rect.Bottom - br * 2, br * 2, br * 2, 0, 90);
            path.AddArc(rect.X, rect.Bottom - bl * 2, bl * 2, bl * 2, 90, 90);
            path.CloseFigure();
            return path;
        }

        public static void ClearToParent(Control control, Graphics graphics)
        {
            graphics.Clear(control.Parent != null ? control.Parent.BackColor : PaneBg);
        }

        public static Label MakeLabel(string text, Font font, Color color)
        {
            return new Label
            {
                Text = text,
                Font = font,
                ForeColor = color,
                BackColor = PaneBg,
                AutoSize = false,
                UseMnemonic = false,
                UseCompatibleTextRendering = false,
            };
        }

        public static LinkLabel MakeLink(string text, Font font)
        {
            var link = new LinkLabel
            {
                Text = text,
                Font = font,
                AutoSize = true,
                BackColor = PaneBg,
                LinkColor = AccentFg,
                ActiveLinkColor = ActionHover,
                VisitedLinkColor = AccentFg,
                LinkBehavior = LinkBehavior.HoverUnderline,
                UseMnemonic = false,
                UseCompatibleTextRendering = false,
            };
            return link;
        }

        public static int MeasureHeight(string text, Font font, int width)
        {
            if (string.IsNullOrEmpty(text))
            {
                text = " ";
            }
            return TextRenderer.MeasureText(text, font, new Size(Math.Max(1, width), int.MaxValue), WrapFlags).Height;
        }

        private static float Clamp(float radius, float max)
        {
            if (radius < 0.5f)
            {
                radius = 0.5f;
            }
            return radius > max ? max : radius;
        }

        private static Color Hex(int rgb)
        {
            return Color.FromArgb(255, (rgb >> 16) & 0xFF, (rgb >> 8) & 0xFF, rgb & 0xFF);
        }

        private static float DetectScale()
        {
            try
            {
                using (var graphics = Graphics.FromHwnd(IntPtr.Zero))
                {
                    float scale = graphics.DpiY / 96f;
                    return scale > 0.5f ? scale : 1f;
                }
            }
            catch
            {
                return 1f;
            }
        }

        [DllImport("user32.dll")]
        private static extern bool SystemParametersInfo(uint action, uint param, ref bool value, uint winIni);
    }

    internal sealed class ChatBubble : Control
    {
        private readonly string _text;
        private readonly bool _isUser;
        private readonly ContextMenuStrip _menu;

        public ChatBubble(string text, bool isUser)
        {
            _text = text ?? "";
            _isUser = isUser;
            Font = PaneTheme.Body;
            TabStop = true;
            AccessibleRole = AccessibleRole.ListItem;
            AccessibleName = (isUser ? "Bạn: " : "AI: ") + _text;
            SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.Selectable, true);
            _menu = new ContextMenuStrip();
            _menu.Items.Add("Sao chép", null, delegate { CopyText(); });
            ContextMenuStrip = _menu;
        }

        public string MessageText
        {
            get { return _text; }
        }

        public bool IsUserBubble
        {
            get { return _isUser; }
        }

        public void Measure(int maxWidth)
        {
            int padX = PaneTheme.Px(PaneTheme.BubblePadX);
            int padY = PaneTheme.Px(PaneTheme.BubblePadY);
            maxWidth = Math.Max(PaneTheme.Px(100), maxWidth);
            Size measured = TextRenderer.MeasureText(_text, Font, new Size(maxWidth - 2 * padX, int.MaxValue), PaneTheme.WrapFlags);
            int width = Math.Min(maxWidth, measured.Width + 2 * padX + 2);
            measured = TextRenderer.MeasureText(_text, Font, new Size(width - 2 * padX, int.MaxValue), PaneTheme.WrapFlags);
            Size = new Size(width, measured.Height + 2 * padY);
        }

        protected override void OnPaintBackground(PaintEventArgs e)
        {
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            PaneTheme.ClearToParent(this, g);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            float big = PaneTheme.PxF(PaneTheme.RadiusBubble);
            float tail = PaneTheme.PxF(PaneTheme.RadiusTail);
            var rect = new RectangleF(0.5f, 0.5f, Width - 1.5f, Height - 1.5f);
            using (GraphicsPath path = _isUser
                ? PaneTheme.RoundedPath(rect, big, big, tail, big)
                : PaneTheme.RoundedPath(rect, tail, big, big, big))
            {
                using (var brush = new SolidBrush(_isUser ? PaneTheme.UserBg : PaneTheme.AiBg))
                {
                    g.FillPath(brush, path);
                }
                if (Focused)
                {
                    using (var pen = new Pen(PaneTheme.Focus, PaneTheme.PxF(PaneTheme.FocusStroke)))
                    {
                        pen.Alignment = PenAlignment.Inset;
                        g.DrawPath(pen, path);
                    }
                }
                else if (!_isUser)
                {
                    using (var pen = new Pen(PaneTheme.AiBorder))
                    {
                        g.DrawPath(pen, path);
                    }
                }
            }
            int padX = PaneTheme.Px(PaneTheme.BubblePadX);
            int padY = PaneTheme.Px(PaneTheme.BubblePadY);
            TextRenderer.DrawText(g, _text, Font, new Rectangle(padX, padY, Width - 2 * padX, Height - 2 * padY),
                _isUser ? PaneTheme.UserFg : PaneTheme.TextPrimary, PaneTheme.WrapFlags);
        }

        protected override void OnMouseDown(MouseEventArgs e)
        {
            base.OnMouseDown(e);
            if (e.Button == MouseButtons.Left && !Focused)
            {
                Focus();
            }
        }

        protected override void OnKeyDown(KeyEventArgs e)
        {
            base.OnKeyDown(e);
            if (e.Control && e.KeyCode == Keys.C)
            {
                CopyText();
                e.Handled = true;
            }
        }

        protected override void OnGotFocus(EventArgs e)
        {
            base.OnGotFocus(e);
            Invalidate();
        }

        protected override void OnLostFocus(EventArgs e)
        {
            base.OnLostFocus(e);
            Invalidate();
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing)
            {
                _menu.Dispose();
            }
            base.Dispose(disposing);
        }

        private void CopyText()
        {
            if (_text.Length == 0)
            {
                return;
            }
            try
            {
                Clipboard.SetText(_text);
            }
            catch (ExternalException)
            {
            }
        }
    }

    internal enum ToolState
    {
        Ok,
        Error,
        Info
    }

    // Một dòng hoạt động của agent: "Thêm tiêu đề   writer.heading".
    internal sealed class ToolLine : Control
    {
        private static readonly Regex ActionPattern = new Regex("\"action\"\\s*:\\s*\"([^\"]+)\"");
        private static readonly Regex FailPattern = new Regex("\"ok\"\\s*:\\s*false");
        private static readonly Regex ErrorPattern = new Regex("\"error\"\\s*:\\s*\"((?:[^\"\\\\]|\\\\.)*)");

        private static readonly Dictionary<string, string> Labels = new Dictionary<string, string>
        {
            { "writer.getText", "Đọc nội dung tài liệu" },
            { "writer.selection", "Đọc vùng chọn" },
            { "writer.newDocument", "Tạo tài liệu mới" },
            { "writer.open", "Mở tài liệu" },
            { "writer.typeText", "Gõ văn bản" },
            { "writer.appendText", "Viết thêm nội dung" },
            { "writer.replaceAll", "Thay thế văn bản" },
            { "writer.insertStyledText", "Chèn đoạn văn" },
            { "writer.formatSelection", "Định dạng vùng chọn" },
            { "writer.setParagraphAlignment", "Căn lề đoạn văn" },
            { "writer.insertTable", "Chèn bảng" },
            { "writer.insertPageBreak", "Ngắt trang" },
            { "writer.insertImage", "Chèn ảnh" },
            { "writer.insertHyperlink", "Chèn liên kết" },
            { "writer.heading", "Thêm tiêu đề" },
            { "writer.closeAll", "Đóng tài liệu" },
            { "et.listSheets", "Đọc danh sách sheet" },
            { "et.newWorkbook", "Tạo bảng tính mới" },
            { "et.open", "Mở bảng tính" },
            { "et.readRange", "Đọc vùng dữ liệu" },
            { "et.writeRange", "Ghi dữ liệu vào ô" },
            { "et.formatRange", "Định dạng ô" },
            { "et.activateSheet", "Chuyển sheet" },
            { "wpp.listSlides", "Đọc danh sách slide" },
            { "wpp.newPresentation", "Tạo bản trình chiếu mới" },
            { "wpp.open", "Mở bản trình chiếu" },
            { "wpp.addSlide", "Thêm slide" },
            { "wpp.addTextBox", "Thêm hộp văn bản" },
            { "wpp.addText", "Thêm chữ vào slide" },
            { "wpp.addImage", "Chèn ảnh vào slide" },
            { "wpp.addTable", "Thêm bảng vào slide" },
            { "wpp.setNotes", "Ghi chú thuyết trình" },
            { "wpp.deleteSlide", "Xoá slide" },
        };

        private static readonly Dictionary<string, string> SuffixLabels = new Dictionary<string, string>
        {
            { "save", "Lưu tệp" },
            { "saveAs", "Lưu tệp" },
            { "exportPdf", "Xuất PDF" },
            { "undo", "Hoàn tác thao tác trước" },
        };

        private readonly string _label;
        private readonly string _actionId;
        private readonly ToolState _state;
        private int _labelHeight;

        public ToolLine(string label, string actionId, ToolState state)
        {
            _label = label ?? "";
            _actionId = actionId ?? "";
            _state = state;
            AccessibleRole = AccessibleRole.ListItem;
            AccessibleName = _label + (state == ToolState.Ok ? ", xong" : state == ToolState.Error ? ", lỗi" : "");
            SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer, true);
        }

        public bool IsToolCall
        {
            get { return _state != ToolState.Info; }
        }

        // Dòng progress từ LlmClient có dạng: office_action {"action":"writer.heading",...} -> {"ok":true,...}
        public static ToolLine FromProgress(string line)
        {
            line = line ?? "";
            int arrow = line.IndexOf(" -> ", StringComparison.Ordinal);
            if (arrow < 0)
            {
                return new ToolLine(line.Trim('(', ')', ' '), "", ToolState.Info);
            }
            string call = line.Substring(0, arrow);
            string outcome = line.Substring(arrow + 4);
            int space = call.IndexOf(' ');
            string toolName = space > 0 ? call.Substring(0, space) : call;
            Match action = ActionPattern.Match(call);
            string actionId = action.Success ? action.Groups[1].Value : toolName;
            string label = action.Success ? LabelFor(actionId) : toolName;
            if (!FailPattern.IsMatch(outcome))
            {
                return new ToolLine(label, actionId, ToolState.Ok);
            }
            Match error = ErrorPattern.Match(outcome);
            if (error.Success)
            {
                string message = error.Groups[1].Value;
                try
                {
                    message = Regex.Unescape(message);
                }
                catch (ArgumentException)
                {
                }
                if (message.Length > 120)
                {
                    message = message.Substring(0, 120) + "…";
                }
                label += ": " + message;
            }
            return new ToolLine(label, actionId, ToolState.Error);
        }

        public static string LabelFor(string actionId)
        {
            string label;
            if (Labels.TryGetValue(actionId, out label))
            {
                return label;
            }
            int dot = actionId.LastIndexOf('.');
            if (dot >= 0 && SuffixLabels.TryGetValue(actionId.Substring(dot + 1), out label))
            {
                return label;
            }
            return "Thao tác trên tài liệu";
        }

        public void Measure(int width)
        {
            int textWidth = Math.Max(PaneTheme.Px(40), width - LabelLeft - IdWidth() - PaneTheme.Px(8));
            _labelHeight = PaneTheme.MeasureHeight(_label, PaneTheme.Tool, textWidth);
            Size = new Size(width, Math.Max(PaneTheme.Px(20), _labelHeight + 2));
        }

        protected override void OnPaintBackground(PaintEventArgs e)
        {
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            PaneTheme.ClearToParent(this, g);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            int rowH = PaneTheme.Px(20);
            float d = PaneTheme.PxF(12);
            float gx = PaneTheme.PxF(2);
            float gy = (rowH - d) / 2f;
            float stroke = PaneTheme.PxF(1.5f);
            if (_state == ToolState.Info)
            {
                using (var pen = new Pen(PaneTheme.TextMuted, stroke))
                {
                    g.DrawEllipse(pen, gx + stroke / 2, gy + stroke / 2, d - stroke, d - stroke);
                }
            }
            else
            {
                using (var brush = new SolidBrush(_state == ToolState.Ok ? PaneTheme.Success : PaneTheme.Danger))
                {
                    g.FillEllipse(brush, gx, gy, d, d);
                }
                using (var pen = new Pen(PaneTheme.PaneBg, stroke))
                {
                    pen.StartCap = LineCap.Round;
                    pen.EndCap = LineCap.Round;
                    if (_state == ToolState.Ok)
                    {
                        g.DrawLines(pen, new[]
                        {
                            new PointF(gx + d * 0.28f, gy + d * 0.52f),
                            new PointF(gx + d * 0.44f, gy + d * 0.68f),
                            new PointF(gx + d * 0.73f, gy + d * 0.36f),
                        });
                    }
                    else
                    {
                        g.DrawLine(pen, gx + d * 0.34f, gy + d * 0.34f, gx + d * 0.66f, gy + d * 0.66f);
                        g.DrawLine(pen, gx + d * 0.66f, gy + d * 0.34f, gx + d * 0.34f, gy + d * 0.66f);
                    }
                }
            }

            int idWidth = IdWidth();
            int labelWidth = Width - LabelLeft - idWidth - PaneTheme.Px(8);
            Color labelColor = _state == ToolState.Error ? PaneTheme.DangerFg : PaneTheme.TextSecondary;
            int top = _labelHeight < rowH ? (rowH - _labelHeight) / 2 : 1;
            TextRenderer.DrawText(g, _label, PaneTheme.Tool, new Rectangle(LabelLeft, top, labelWidth, Height - top),
                labelColor, PaneTheme.WrapFlags);
            if (idWidth > 0)
            {
                TextRenderer.DrawText(g, _actionId, PaneTheme.ToolId, new Rectangle(Width - idWidth, 0, idWidth, rowH),
                    PaneTheme.TextMuted, PaneTheme.LineFlags | TextFormatFlags.Right);
            }
        }

        private static int LabelLeft
        {
            get { return PaneTheme.Px(22); }
        }

        private int IdWidth()
        {
            if (_actionId.Length == 0)
            {
                return 0;
            }
            int natural = TextRenderer.MeasureText(_actionId, PaneTheme.ToolId, Size.Empty, TextFormatFlags.NoPrefix | TextFormatFlags.SingleLine).Width;
            return Math.Min(natural, PaneTheme.Px(120));
        }
    }

    internal sealed class TypingBubble : Control
    {
        private readonly Timer _timer;
        private int _phase;

        public TypingBubble()
        {
            Size = new Size(PaneTheme.Px(52), PaneTheme.Px(32));
            AccessibleRole = AccessibleRole.Animation;
            AccessibleName = "AI đang xử lý";
            SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer, true);
            _timer = new Timer { Interval = 350 };
            _timer.Tick += delegate
            {
                _phase = (_phase + 1) % 3;
                Invalidate();
            };
            if (PaneTheme.AnimationsEnabled)
            {
                _timer.Start();
            }
            else
            {
                _phase = -1;
            }
        }

        public void StopAnimation()
        {
            _timer.Stop();
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing)
            {
                _timer.Dispose();
            }
            base.Dispose(disposing);
        }

        protected override void OnPaintBackground(PaintEventArgs e)
        {
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            PaneTheme.ClearToParent(this, g);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            float big = PaneTheme.PxF(PaneTheme.RadiusBubble);
            float tail = PaneTheme.PxF(PaneTheme.RadiusTail);
            using (var path = PaneTheme.RoundedPath(new RectangleF(0.5f, 0.5f, Width - 1.5f, Height - 1.5f), tail, big, big, big))
            {
                using (var brush = new SolidBrush(PaneTheme.AiBg))
                {
                    g.FillPath(brush, path);
                }
                using (var pen = new Pen(PaneTheme.AiBorder))
                {
                    g.DrawPath(pen, path);
                }
            }
            float dot = PaneTheme.PxF(6);
            float step = PaneTheme.PxF(10);
            float x = (Width - (2 * step + dot)) / 2f;
            float y = (Height - dot) / 2f;
            for (int i = 0; i < 3; i++)
            {
                int alpha = _phase < 0 ? 180 : (i == _phase ? 255 : 80);
                using (var brush = new SolidBrush(Color.FromArgb(alpha, PaneTheme.Accent)))
                {
                    g.FillEllipse(brush, x + i * step, y, dot, dot);
                }
            }
        }
    }

    // Button thật (giữ UIA Invoke, Enter/Space, Tab) nhưng tự vẽ để bo góc có anti-alias.
    internal abstract class PaneButton : Button
    {
        protected bool Hovered;
        protected bool Pressed;

        protected PaneButton()
        {
            SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer, true);
            UseMnemonic = false;
            UseVisualStyleBackColor = false;
            FlatStyle = FlatStyle.Flat;
            FlatAppearance.BorderSize = 0;
            Cursor = Cursors.Hand;
            BackColor = PaneTheme.PaneBg;
        }

        protected override void OnMouseEnter(EventArgs e)
        {
            Hovered = true;
            Invalidate();
            base.OnMouseEnter(e);
        }

        protected override void OnMouseLeave(EventArgs e)
        {
            Hovered = false;
            Pressed = false;
            Invalidate();
            base.OnMouseLeave(e);
        }

        protected override void OnMouseDown(MouseEventArgs e)
        {
            if (e.Button == MouseButtons.Left)
            {
                Pressed = true;
                Invalidate();
            }
            base.OnMouseDown(e);
        }

        protected override void OnMouseUp(MouseEventArgs e)
        {
            Pressed = false;
            Invalidate();
            base.OnMouseUp(e);
        }

        protected override void OnEnabledChanged(EventArgs e)
        {
            Cursor = Enabled ? Cursors.Hand : Cursors.Default;
            Invalidate();
            base.OnEnabledChanged(e);
        }

        protected override void OnGotFocus(EventArgs e)
        {
            Invalidate();
            base.OnGotFocus(e);
        }

        protected override void OnLostFocus(EventArgs e)
        {
            Invalidate();
            base.OnLostFocus(e);
        }

        protected override void OnPaintBackground(PaintEventArgs e)
        {
        }
    }

    internal sealed class ChipButton : PaneButton
    {
        public ChipButton(string text)
        {
            Text = text;
            Font = PaneTheme.Small;
            AccessibleDescription = "Điền gợi ý vào ô nhập";
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            PaneTheme.ClearToParent(this, g);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            Color fill = Pressed ? PaneTheme.ChipPressed : (Hovered ? PaneTheme.ChipHover : PaneTheme.PaneBg);
            var rect = new RectangleF(0.5f, 0.5f, Width - 1.5f, Height - 1.5f);
            using (var path = PaneTheme.RoundedPath(rect, PaneTheme.PxF(PaneTheme.RadiusControl)))
            {
                using (var brush = new SolidBrush(fill))
                {
                    g.FillPath(brush, path);
                }
                // Focus: viền 2px màu Focus thay cho viền chip, vẽ trong bounds nên không cần chừa lề.
                using (var pen = Focused
                    ? new Pen(PaneTheme.Focus, PaneTheme.PxF(PaneTheme.FocusStroke)) { Alignment = PenAlignment.Inset }
                    : new Pen(PaneTheme.ChipBorder))
                {
                    g.DrawPath(pen, path);
                }
            }
            int pad = PaneTheme.Px(12);
            TextRenderer.DrawText(g, Text, Font, new Rectangle(pad, 0, Width - 2 * pad, Height), PaneTheme.ChipFg, PaneTheme.LineFlags);
        }
    }

    internal sealed class SendButton : PaneButton
    {
        public SendButton()
        {
            Text = "Ask";
            Font = PaneTheme.SmallBold;
            AccessibleDescription = "Gửi yêu cầu cho AI";
            Size = new Size(PaneTheme.Px(PaneTheme.AskW) + 2 * Inset, PaneTheme.Px(PaneTheme.AskH) + 2 * Inset);
        }

        // Lề trong bounds để chứa focus ring 2px cách mặt nút 2px.
        public static int Inset
        {
            get { return PaneTheme.Px(PaneTheme.FocusStroke + PaneTheme.FocusGap); }
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            PaneTheme.ClearToParent(this, g);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            int inset = Inset;
            var face = new RectangleF(inset, inset, Width - 2 * inset, Height - 2 * inset);
            Color fill = !Enabled ? PaneTheme.DisabledBg
                : Pressed ? PaneTheme.ActionPressed
                : Hovered ? PaneTheme.ActionHover
                : PaneTheme.ActionBg;
            using (var path = PaneTheme.RoundedPath(face, PaneTheme.PxF(PaneTheme.RadiusControl)))
            using (var brush = new SolidBrush(fill))
            {
                g.FillPath(brush, path);
            }
            if (Focused && Enabled)
            {
                float stroke = PaneTheme.PxF(PaneTheme.FocusStroke);
                RectangleF ring = RectangleF.Inflate(face, PaneTheme.PxF(PaneTheme.FocusGap) + stroke / 2, PaneTheme.PxF(PaneTheme.FocusGap) + stroke / 2);
                using (var path = PaneTheme.RoundedPath(ring, PaneTheme.PxF(PaneTheme.RadiusControl + PaneTheme.FocusGap)))
                using (var pen = new Pen(PaneTheme.Focus, stroke))
                {
                    g.DrawPath(pen, path);
                }
            }
            TextRenderer.DrawText(g, Text, Font, Rectangle.Round(face), Enabled ? PaneTheme.OnAction : PaneTheme.DisabledFg,
                TextFormatFlags.HorizontalCenter | TextFormatFlags.VerticalCenter | TextFormatFlags.NoPrefix | TextFormatFlags.SingleLine);
        }
    }

    // Edit multiline của Win32 không phát EN_CHANGE khi text được đặt bằng WM_SETTEXT (UIA ValuePattern,
    // automation), nên TextChanged không chạy và nút Ask không bật. Tự báo lại sau WM_SETTEXT.
    internal sealed class PromptBox : TextBox
    {
        private const int WmSetText = 0x000C;

        protected override void WndProc(ref Message m)
        {
            base.WndProc(ref m);
            if (m.Msg == WmSetText)
            {
                OnTextChanged(EventArgs.Empty);
            }
        }
    }

    // Khung ô nhập: vẽ viền bo góc quanh TextBox thật.
    internal sealed class ComposerPanel : Panel
    {
        private bool _focusedLook;
        private bool _blocked;

        public ComposerPanel()
        {
            BackColor = PaneTheme.PaneBg;
            Cursor = Cursors.IBeam;
            SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw, true);
        }

        public bool FocusedLook
        {
            get { return _focusedLook; }
            set
            {
                if (_focusedLook != value)
                {
                    _focusedLook = value;
                    Invalidate();
                }
            }
        }

        public bool Blocked
        {
            get { return _blocked; }
            set
            {
                if (_blocked != value)
                {
                    _blocked = value;
                    Cursor = value ? Cursors.Default : Cursors.IBeam;
                    Invalidate();
                }
            }
        }

        public Rectangle BoxBounds
        {
            get
            {
                int left = PaneTheme.Px(PaneTheme.PadX);
                int top = PaneTheme.Px(8);
                return new Rectangle(left, top, ClientSize.Width - 2 * left, ClientSize.Height - top);
            }
        }

        protected override void OnPaintBackground(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            g.Clear(BackColor);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            Rectangle box = BoxBounds;
            var rect = new RectangleF(box.X + 0.5f, box.Y + 0.5f, box.Width - 1.5f, box.Height - 1.5f);
            using (var path = PaneTheme.RoundedPath(rect, PaneTheme.PxF(PaneTheme.RadiusComposer)))
            {
                if (_blocked)
                {
                    using (var brush = new SolidBrush(PaneTheme.InputDisabled))
                    {
                        g.FillPath(brush, path);
                    }
                    using (var pen = new Pen(PaneTheme.Divider))
                    {
                        g.DrawPath(pen, path);
                    }
                }
                else if (_focusedLook)
                {
                    using (var pen = new Pen(PaneTheme.Focus, PaneTheme.PxF(PaneTheme.FocusStroke)) { Alignment = PenAlignment.Inset })
                    {
                        g.DrawPath(pen, path);
                    }
                }
                else
                {
                    using (var pen = new Pen(PaneTheme.InputBorder))
                    {
                        g.DrawPath(pen, path);
                    }
                }
            }
        }
    }

    // Chấm tròn accent: header (8px) và chỉ báo "đang chạy" ở footer (6px).
    internal sealed class AccentDot : Control
    {
        private readonly int _diameter;

        public AccentDot(int diameter)
        {
            _diameter = diameter;
            Size = new Size(diameter, diameter);
            AccessibleRole = AccessibleRole.Graphic;
            SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw, true);
        }

        protected override void OnPaintBackground(PaintEventArgs e)
        {
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            PaneTheme.ClearToParent(this, e.Graphics);
            e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
            using (var brush = new SolidBrush(PaneTheme.Accent))
            {
                e.Graphics.FillEllipse(brush, 0, (Height - _diameter) / 2f, _diameter, _diameter);
            }
        }
    }

    // Panel có đường kẻ 1px ở đáy (header).
    internal sealed class DividerPanel : Panel
    {
        public DividerPanel()
        {
            BackColor = PaneTheme.PaneBg;
            SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw, true);
        }

        protected override void OnPaintBackground(PaintEventArgs e)
        {
            e.Graphics.Clear(BackColor);
            using (var pen = new Pen(PaneTheme.Divider))
            {
                e.Graphics.DrawLine(pen, 0, Height - 1, Width, Height - 1);
            }
        }
    }

    // Empty state: tiêu đề + mô tả + danh sách chip gợi ý xếp dọc.
    internal sealed class EmptyState : Panel
    {
        private readonly Label _title;
        private readonly Label _description;
        private readonly Label _section;
        private readonly List<ChipButton> _chips = new List<ChipButton>();

        public event Action<string> SuggestionClicked;

        public EmptyState(string title, string description, IEnumerable<string> suggestions)
        {
            BackColor = PaneTheme.PaneBg;
            AccessibleRole = AccessibleRole.Grouping;
            AccessibleName = title;
            _title = PaneTheme.MakeLabel(title, PaneTheme.EmptyTitle, PaneTheme.TextPrimary);
            _description = PaneTheme.MakeLabel(description, PaneTheme.Body, PaneTheme.TextSecondary);
            _section = PaneTheme.MakeLabel("GỢI Ý", PaneTheme.CaptionBold, PaneTheme.TextMuted);
            Controls.Add(_title);
            Controls.Add(_description);
            Controls.Add(_section);
            foreach (string suggestion in suggestions)
            {
                string captured = suggestion;
                var chip = new ChipButton(suggestion);
                chip.Click += delegate
                {
                    Action<string> handler = SuggestionClicked;
                    if (handler != null)
                    {
                        handler(captured);
                    }
                };
                _chips.Add(chip);
                Controls.Add(chip);
            }
        }

        public void Measure(int width)
        {
            int y = PaneTheme.Px(4);
            int h = PaneTheme.MeasureHeight(_title.Text, _title.Font, width);
            _title.SetBounds(0, y, width, h);
            y += h + PaneTheme.Px(4);
            h = PaneTheme.MeasureHeight(_description.Text, _description.Font, width);
            _description.SetBounds(0, y, width, h);
            y += h + PaneTheme.Px(16);
            h = PaneTheme.MeasureHeight(_section.Text, _section.Font, width);
            _section.SetBounds(0, y, width, h);
            y += h + PaneTheme.Px(8);
            foreach (ChipButton chip in _chips)
            {
                chip.SetBounds(0, y, width, PaneTheme.Px(PaneTheme.ChipH));
                y += PaneTheme.Px(PaneTheme.ChipH) + PaneTheme.Px(PaneTheme.GapChip);
            }
            Size = new Size(width, y);
        }
    }

    // Thẻ lỗi: dải đỏ bên trái + tiêu đề + nội dung + link "Thử lại" / "Mở Cài đặt".
    internal sealed class ErrorCard : Panel
    {
        private readonly Label _title;
        private readonly Label _message;
        private readonly LinkLabel _retry;
        private readonly LinkLabel _settings;

        public event EventHandler RetryClicked;
        public event EventHandler SettingsClicked;

        public ErrorCard(string title, string message)
        {
            BackColor = PaneTheme.DangerBg;
            AccessibleRole = AccessibleRole.Alert;
            AccessibleName = title + ". " + message;
            SetStyle(ControlStyles.UserPaint | ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.ResizeRedraw, true);
            _title = PaneTheme.MakeLabel(title, PaneTheme.SmallBold, PaneTheme.DangerFg);
            _message = PaneTheme.MakeLabel(message, PaneTheme.Small, PaneTheme.TextPrimary);
            _retry = PaneTheme.MakeLink("Thử lại", PaneTheme.SmallBold);
            _settings = PaneTheme.MakeLink("Mở Cài đặt", PaneTheme.SmallBold);
            foreach (Control control in new Control[] { _title, _message, _retry, _settings })
            {
                control.BackColor = PaneTheme.DangerBg;
                Controls.Add(control);
            }
            _retry.LinkClicked += delegate { Raise(RetryClicked); };
            _settings.LinkClicked += delegate { Raise(SettingsClicked); };
        }

        public void Measure(int width)
        {
            int left = PaneTheme.Px(3 + 12);
            int right = PaneTheme.Px(12);
            int inner = Math.Max(PaneTheme.Px(60), width - left - right);
            int y = PaneTheme.Px(8);
            int h = PaneTheme.MeasureHeight(_title.Text, _title.Font, inner);
            _title.SetBounds(left, y, inner, h);
            y += h + PaneTheme.Px(2);
            h = PaneTheme.MeasureHeight(_message.Text, _message.Font, inner);
            _message.SetBounds(left, y, inner, h);
            y += h + PaneTheme.Px(6);
            _retry.Location = new Point(left, y);
            _settings.Location = new Point(_retry.Right + PaneTheme.Px(12), y);
            y += Math.Max(_retry.Height, _settings.Height) + PaneTheme.Px(8);
            Size = new Size(width, y);
        }

        protected override void OnPaintBackground(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            PaneTheme.ClearToParent(this, g);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            var rect = new RectangleF(0.5f, 0.5f, Width - 1.5f, Height - 1.5f);
            using (var path = PaneTheme.RoundedPath(rect, PaneTheme.PxF(PaneTheme.RadiusControl)))
            {
                using (var brush = new SolidBrush(PaneTheme.DangerBg))
                {
                    g.FillPath(brush, path);
                }
                GraphicsState state = g.Save();
                g.SetClip(path);
                using (var stripe = new SolidBrush(PaneTheme.Danger))
                {
                    g.FillRectangle(stripe, 0, 0, PaneTheme.PxF(3), Height);
                }
                g.Restore(state);
                using (var pen = new Pen(PaneTheme.DangerBorder))
                {
                    g.DrawPath(pen, path);
                }
            }
        }

        private void Raise(EventHandler handler)
        {
            if (handler != null)
            {
                handler(this, EventArgs.Empty);
            }
        }
    }

    internal sealed class ChatList : FlowLayoutPanel
    {
        private readonly ToolTip _toolTip = new ToolTip();
        private TypingBubble _typing;
        private bool _realigning;
        private bool _realignAgain;

        public ChatList()
        {
            FlowDirection = FlowDirection.TopDown;
            WrapContents = false;
            AutoScroll = true;
            BackColor = PaneTheme.PaneBg;
            int pad = PaneTheme.Px(PaneTheme.PadX);
            Padding = new Padding(pad, pad, pad, pad);
            // AutoScroll của WinForms bỏ qua Padding đáy khi tính vùng cuộn; bù bằng AutoScrollMargin.
            AutoScrollMargin = new Size(0, pad);
            AccessibleRole = AccessibleRole.List;
            AccessibleName = "Hội thoại";
            SetStyle(ControlStyles.OptimizedDoubleBuffer | ControlStyles.AllPaintingInWmPaint, true);
        }

        public ChatBubble AddMessage(string text, bool isUser)
        {
            var bubble = new ChatBubble(text, isUser);
            AddBlock(bubble);
            return bubble;
        }

        public ToolLine AddProgress(string progressLine)
        {
            ToolLine line = ToolLine.FromProgress(progressLine);
            _toolTip.SetToolTip(line, progressLine);
            AddBlock(line);
            return line;
        }

        public ToolLine AddInfo(string text)
        {
            var line = new ToolLine(text, "", ToolState.Info);
            AddBlock(line);
            return line;
        }

        public void AddTyping()
        {
            RemoveTyping();
            _typing = new TypingBubble();
            AddBlock(_typing);
        }

        public void RemoveTyping()
        {
            if (_typing != null)
            {
                TypingBubble typing = _typing;
                _typing = null;
                RemoveItem(typing);
            }
        }

        // Thêm một khối bất kỳ (bubble, empty state, thẻ lỗi...). Luôn nằm trên typing indicator.
        public void AddBlock(Control control)
        {
            Controls.Add(control);
            if (_typing != null && control != _typing && Controls.Contains(_typing))
            {
                Controls.SetChildIndex(control, Controls.GetChildIndex(_typing));
            }
            Realign();
            ScrollToBottom();
        }

        public void RemoveItem(Control control)
        {
            if (control == null)
            {
                return;
            }
            var typing = control as TypingBubble;
            if (typing != null)
            {
                typing.StopAnimation();
            }
            Controls.Remove(control);
            control.Dispose();
            Realign();
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing)
            {
                _toolTip.Dispose();
            }
            base.Dispose(disposing);
        }

        protected override void OnClientSizeChanged(EventArgs e)
        {
            base.OnClientSizeChanged(e);
            Realign();
        }

        // Click vào bubble để focus (Ctrl+C) không được làm danh sách nhảy vị trí cuộn.
        protected override Point ScrollToControl(Control activeControl)
        {
            if (Control.MouseButtons != MouseButtons.None)
            {
                return DisplayRectangle.Location;
            }
            return base.ScrollToControl(activeControl);
        }

        private void Realign()
        {
            if (_realigning)
            {
                _realignAgain = true;
                return;
            }
            _realigning = true;
            // Đang ở cuối danh sách thì sau khi đo lại (scrollbar hiện ra, pane đổi rộng) vẫn giữ ở cuối.
            bool stickToBottom = IsAtBottom();
            try
            {
                int passes = 0;
                do
                {
                    _realignAgain = false;
                    RealignOnce();
                }
                while (_realignAgain && ++passes < 3);
            }
            finally
            {
                _realigning = false;
            }
            if (stickToBottom)
            {
                ScrollToBottom();
            }
        }

        private bool IsAtBottom()
        {
            if (!VerticalScroll.Visible)
            {
                return true;
            }
            return -AutoScrollPosition.Y + ClientSize.Height >= DisplayRectangle.Height - PaneTheme.Px(4);
        }

        private void RealignOnce()
        {
            // ClientSize đã trừ scrollbar khi nó đang hiện. Nếu nội dung sắp tràn mà scrollbar chưa hiện,
            // chừa sẵn chỗ để không phát sinh scrollbar ngang.
            int available = Math.Max(PaneTheme.Px(120), ClientSize.Width - Padding.Horizontal);
            SuspendLayout();
            try
            {
                int total = MeasureAll(available);
                if (!VerticalScroll.Visible && total > ClientSize.Height)
                {
                    MeasureAll(Math.Max(PaneTheme.Px(120), available - SystemInformation.VerticalScrollBarWidth));
                }
            }
            finally
            {
                ResumeLayout(true);
            }
        }

        private int MeasureAll(int available)
        {
            int total = Padding.Vertical;
            int count = Controls.Count;
            for (int i = 0; i < count; i++)
            {
                Control control = Controls[i];
                Control next = i + 1 < count ? Controls[i + 1] : null;
                bool nextIsTool = next is ToolLine;
                int left = 0;
                int bottom;

                var bubble = control as ChatBubble;
                var line = control as ToolLine;
                var empty = control as EmptyState;
                var card = control as ErrorCard;
                if (bubble != null)
                {
                    bubble.Measure((int)(available * (bubble.IsUserBubble ? PaneTheme.BubbleMaxUser : PaneTheme.BubbleMaxAi)));
                    if (bubble.IsUserBubble)
                    {
                        left = Math.Max(0, available - bubble.Width);
                    }
                    bottom = nextIsTool ? PaneTheme.GapBeforeTools : PaneTheme.GapMessage;
                }
                else if (line != null)
                {
                    line.Measure(available);
                    bottom = nextIsTool ? PaneTheme.GapToolLine : PaneTheme.GapMessage;
                }
                else if (empty != null)
                {
                    empty.Measure(available);
                    bottom = PaneTheme.GapMessage;
                }
                else if (card != null)
                {
                    card.Measure(available);
                    bottom = PaneTheme.GapMessage;
                }
                else
                {
                    bottom = nextIsTool ? PaneTheme.GapBeforeTools : PaneTheme.GapMessage;
                }
                int bottomPx = next == null ? 0 : PaneTheme.Px(bottom);
                control.Margin = new Padding(left, 0, 0, bottomPx);
                total += control.Height + bottomPx;
            }
            return total;
        }

        private void ScrollToBottom()
        {
            if (Controls.Count == 0)
            {
                return;
            }
            try
            {
                Control last = Controls[Controls.Count - 1];
                if (last.Height > ClientSize.Height)
                {
                    // Câu trả lời dài hơn khung nhìn: hiện phần đầu để đọc từ trên xuống.
                    ScrollControlIntoView(last);
                }
                else
                {
                    AutoScrollPosition = new Point(0, DisplayRectangle.Height);
                }
            }
            catch (Exception)
            {
            }
        }
    }
}

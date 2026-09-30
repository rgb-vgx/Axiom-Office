using System;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Windows.Forms;

namespace AxiomOffice.Ai
{
    // Control riêng cho wizard thiết lập. Lý do tồn tại: TextBox/ComboBox/CheckBox của WinForms vẽ theo kiểu hệ
    // thống (viền vuông, tick 3D) trong khi nút của Axiom là phẳng bo góc - trộn hai ngôn ngữ thị giác thì nhìn
    // như hai sản phẩm ghép lại. Các control dưới đây vẽ lại cho khớp bảng màu/kích thước của PaneTheme.
    // Đặt ở file riêng để không đụng vào PaneControls.cs (pane đang dùng chung).

    // Khung viền bo góc cho một control nhập: TextBox bên trong đặt BorderStyle.None, khung này vẽ viền 1px
    // (InputBorder, khi focus thì đổi màu Focus). Dùng cho ô Địa chỉ máy chủ / Khoá truy cập / Model.
    internal sealed class FieldBox : Panel
    {
        private bool _focused;

        public FieldBox(Control inner, int height = 30, int innerHeight = 22)
        {
            BackColor = PaneTheme.PaneBg;
            Height = height;
            Inner = inner;

            var textBox = inner as TextBoxBase;
            if (textBox != null)
            {
                textBox.BorderStyle = BorderStyle.None;
            }

            inner.AutoSize = false;
            inner.BackColor = PaneTheme.PaneBg;
            try
            {
                inner.Height = innerHeight;      // ComboBox DropDownList tu choi doi chieu cao -> bo qua
            }
            catch (Exception)
            {
            }

            inner.Location = new Point(10, Math.Max(1, (height - innerHeight) / 2));
            inner.Width = Math.Max(20, Width - 20);
            inner.GotFocus += delegate { _focused = true; Invalidate(); };
            inner.LostFocus += delegate { _focused = false; Invalidate(); };
            inner.SizeChanged += delegate { ResizeInner(); };
            SizeChanged += delegate { ResizeInner(); };
            Controls.Add(inner);
            SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer, true);
        }

        public Control Inner { get; private set; }

        // Ten khac Control.Layout (co san cua WinForms) de khong che khuat thanh vien ke thua.
        private void ResizeInner()
        {
            Inner.Width = Math.Max(20, Width - 20);
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;
            var face = new RectangleF(0.5f, 0.5f, Width - 1.5f, Height - 1.5f);
            Color border = !Enabled ? PaneTheme.DisabledBg : _focused ? PaneTheme.Focus : PaneTheme.InputBorder;
            using (var path = PaneTheme.RoundedPath(face, 6))
            using (var pen = new Pen(border, _focused ? 2f : 1f))
            {
                g.DrawPath(pen, path);
            }
        }
    }

    // Checkbox phẳng: hình vuông bo góc 1px (đã chọn thì nền indigo + dấu ✓ trắng), nhãn bên phải, ghi chú mờ
    // phía dưới - thay cho CheckBox của WinForms.
    internal sealed class ToggleBox : Control
    {
        private bool _checked;
        private bool _hover;

        public event EventHandler CheckedChanged;

        public ToggleBox()
        {
            SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.UserPaint, true);
            Font = PaneTheme.SmallBold;
            Height = 46;
            Cursor = Cursors.Hand;
            TabStop = true;
        }

        public bool Checked
        {
            get { return _checked; }
            set
            {
                if (_checked == value)
                {
                    return;
                }

                _checked = value;
                Invalidate();
                EventHandler handler = CheckedChanged;
                if (handler != null)
                {
                    handler(this, EventArgs.Empty);
                }
            }
        }

        // Ghi chu duoi nhan (mot dong hoac hai dong).
        public string Hint { get; set; }

        public override string Text
        {
            get { return base.Text; }
            set { base.Text = value; Invalidate(); }
        }

        protected override void OnMouseEnter(EventArgs e)
        {
            _hover = true;
            Invalidate();
            base.OnMouseEnter(e);
        }

        protected override void OnMouseLeave(EventArgs e)
        {
            _hover = false;
            Invalidate();
            base.OnMouseLeave(e);
        }

        protected override void OnClick(EventArgs e)
        {
            Focus();
            Checked = !Checked;
            base.OnClick(e);
        }

        protected override bool IsInputKey(Keys keyData)
        {
            return keyData == Keys.Space || base.IsInputKey(keyData);
        }

        protected override void OnKeyDown(KeyEventArgs e)
        {
            if (e.KeyCode == Keys.Space)
            {
                Checked = !Checked;
                e.Handled = true;
            }

            base.OnKeyDown(e);
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

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            PaneTheme.ClearToParent(this, g);
            g.SmoothingMode = SmoothingMode.AntiAlias;

            const int boxSize = 18;
            var box = new RectangleF(0.5f, 2.5f, boxSize - 1f, boxSize - 1f);
            using (var path = PaneTheme.RoundedPath(box, 4))
            {
                if (Checked)
                {
                    using (var brush = new SolidBrush(PaneTheme.ActionBg))
                    {
                        g.FillPath(brush, path);
                    }
                }
                else
                {
                    using (var brush = new SolidBrush(_hover ? PaneTheme.ChipHover : PaneTheme.PaneBg))
                    {
                        g.FillPath(brush, path);
                    }
                }

                using (var pen = new Pen(Focused ? PaneTheme.Focus : Checked ? PaneTheme.ActionBg : PaneTheme.InputBorder,
                    Focused ? 2f : 1f))
                {
                    g.DrawPath(pen, path);
                }
            }

            if (Checked)
            {
                using (var pen = new Pen(PaneTheme.OnAction, 2f))
                {
                    pen.StartCap = LineCap.Round;
                    pen.EndCap = LineCap.Round;
                    g.DrawLines(pen, new[]
                    {
                        new PointF(5.5f, 11.5f),
                        new PointF(8.5f, 14.5f),
                        new PointF(13.5f, 7.5f),
                    });
                }
            }

            int textLeft = boxSize + 10;
            int textWidth = Math.Max(40, Width - textLeft);
            int labelHeight = TextRenderer.MeasureText(Text, Font, new Size(textWidth, 0)).Height;
            TextRenderer.DrawText(g, Text, Font, new Rectangle(textLeft, 0, textWidth, labelHeight), PaneTheme.TextPrimary,
                TextFormatFlags.NoPrefix | TextFormatFlags.WordBreak);
            if (!string.IsNullOrEmpty(Hint))
            {
                TextRenderer.DrawText(g, Hint, PaneTheme.Small, new Rectangle(textLeft, labelHeight + 2, textWidth, 34),
                    PaneTheme.TextMuted, TextFormatFlags.NoPrefix | TextFormatFlags.WordBreak);
            }
        }
    }

    // Huy hiệu trạng thái: hình tròn đặc + dấu (✓ / ! / ✗) màu trắng - thay cho ký tự ✓/✗ trần trong danh sách
    // kiểm tra (ký tự trần nhìn như lỗi gõ, không như một chỉ báo).
    internal sealed class StatusBadge : Control
    {
        private string _glyph = "✓";
        private bool _warn;
        private bool _danger;
        private Color _custom = Color.Empty;

        public StatusBadge()
        {
            SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.UserPaint, true);
            Size = new Size(20, 20);
        }

        public void Set(bool ok, bool warn, bool danger)
        {
            _warn = warn;
            _danger = danger;
            _glyph = danger ? "✗" : warn ? "!" : "✓";
            _custom = Color.Empty;
            Invalidate();
        }

        // Huy hieu dung cho muc khac (vd so thu tu buoc): hinh tron mau `fill` + chu `glyph`.
        public void SetGlyph(string glyph, Color fill)
        {
            _glyph = glyph;
            _custom = fill;
            _warn = false;
            _danger = false;
            Invalidate();
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            PaneTheme.ClearToParent(this, g);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            Color fill = _custom != Color.Empty ? _custom
                : _danger ? PaneTheme.Danger
                : _warn ? PaneTheme.TextMuted
                : PaneTheme.Success;
            using (var brush = new SolidBrush(fill))
            {
                g.FillEllipse(brush, new RectangleF(0, 0, Width - 1, Height - 1));
            }

            using (var font = new Font("Segoe UI", 9f, FontStyle.Bold))
            using (var brush = new SolidBrush(PaneTheme.OnAction))
            using (var format = new StringFormat { Alignment = StringAlignment.Center, LineAlignment = StringAlignment.Center })
            {
                g.DrawString(_glyph, font, brush, new RectangleF(0, 0, Width, Height - 1), format);
            }
        }
    }

    // Thẻ bo góc cho một dòng/nhóm nội dung: viền 1px + nền nhạt theo sắc thái (trung tính / thành công / cảnh báo
    // / lỗi). Dùng cho các dòng ở bước "Kiểm tra máy" và thẻ kết quả thử kết nối.
    internal sealed class CardBox : Panel
    {
        private string _tone = "neutral";

        public CardBox()
        {
            SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer, true);
            BackColor = PaneTheme.PaneBg;
        }

        // "neutral" | "success" | "warn" | "danger"
        public string Tone
        {
            get { return _tone; }
            set
            {
                _tone = value ?? "neutral";
                Invalidate();
            }
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            g.SmoothingMode = SmoothingMode.AntiAlias;
            Color fill = Tone == "danger" ? PaneTheme.DangerBg
                : Tone == "warn" ? PaneTheme.AiBg
                : Tone == "success" ? PaneTheme.AiBg
                : PaneTheme.PaneBg;
            Color border = Tone == "danger" ? PaneTheme.DangerBorder
                : Tone == "warn" ? PaneTheme.ChipBorder
                : Tone == "success" ? PaneTheme.ChipBorder
                : PaneTheme.Divider;
            var face = new RectangleF(0.5f, 0.5f, Width - 1.5f, Height - 1.5f);
            using (var path = PaneTheme.RoundedPath(face, 8))
            {
                using (var brush = new SolidBrush(fill))
                {
                    g.FillPath(brush, path);
                }

                using (var pen = new Pen(border, 1f))
                {
                    g.DrawPath(pen, path);
                }
            }
        }
    }

    // Chỉ báo bước: 5 đoạn bo góc (đã qua / đang ở = indigo, còn lại = xám). Chấm ●○○○○ ở cỡ 8.25pt khó thấy và
    // trùng nghĩa với chữ "bước 2/5" bên cạnh; thanh đoạn đọc được ngay cả khi cửa sổ nhỏ.
    internal sealed class StepBar : Control
    {
        private int _step;

        public StepBar()
        {
            SetStyle(ControlStyles.AllPaintingInWmPaint | ControlStyles.OptimizedDoubleBuffer | ControlStyles.UserPaint, true);
            Size = new Size(5 * (18 + 6), 6);
        }

        public int Step
        {
            get { return _step; }
            set
            {
                _step = value;
                Invalidate();
            }
        }

        protected override void OnPaint(PaintEventArgs e)
        {
            Graphics g = e.Graphics;
            PaneTheme.ClearToParent(this, g);
            g.SmoothingMode = SmoothingMode.AntiAlias;
            const int segment = 18;
            const int gap = 6;
            for (int index = 0; index < 5; index++)
            {
                var rect = new RectangleF(index * (segment + gap), 0, segment, Height - 1);
                using (var path = PaneTheme.RoundedPath(rect, 3))
                using (var brush = new SolidBrush(index <= _step ? PaneTheme.ActionBg : PaneTheme.Divider))
                {
                    g.FillPath(brush, path);
                }
            }
        }
    }
}

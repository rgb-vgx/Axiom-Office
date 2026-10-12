using System;
using System.Collections.Generic;
using System.Drawing;
using System.Text;
using System.Text.RegularExpressions;
using System.Windows.Forms;

namespace AxiomOffice.Ai
{
    // Markdown trong bong bóng trả lời của AI (pane Windows). Model trả **đậm**, ## tiêu đề, - danh sách, `mã`…;
    // vẽ nguyên văn thì người dùng thấy ký hiệu. Chỉ phần model hay dùng: đậm, nghiêng, mã trong dòng, link (chỉ
    // tô màu, không bấm), tiêu đề, danh sách (lồng theo thụt lề), khối mã, trích dẫn, đường kẻ. Bảng hiện bằng chữ
    // đều (Consolas) cho thẳng cột - pane hẹp, không dựng lưới.
    internal enum MdBlockKind
    {
        Paragraph,
        Heading,
        Bullet,
        Numbered,
        Code,
        Quote,
        Table,
        Rule,
        Blank
    }

    internal sealed class MdRun
    {
        public string Text;
        public bool Bold;
        public bool Italic;
        public bool Code;
        public bool Link;
    }

    internal sealed class MdBlock
    {
        public MdBlockKind Kind;
        public int Level;            // tiêu đề: 1..6; danh sách: mức lồng (0 = ngoài cùng)
        public string Marker;        // "1." cho danh sách số
        public List<MdRun> Runs = new List<MdRun>();
        public List<string> Lines;   // khối mã / bảng: giữ nguyên từng dòng
    }

    internal static class Markdown
    {
        private static readonly Regex Heading = new Regex(@"^\s{0,3}(#{1,6})\s+(.*?)\s*#*\s*$");
        private static readonly Regex Bullet = new Regex(@"^(\s*)[-*+•]\s+(.*)$");
        private static readonly Regex Numbered = new Regex(@"^(\s*)(\d{1,3})[.)]\s+(.*)$");
        private static readonly Regex Rule = new Regex(@"^\s{0,3}([-*_])(\s*\1){2,}\s*$");
        private static readonly Regex Fence = new Regex(@"^\s{0,3}(```|~~~)");
        private static readonly Regex Quote = new Regex(@"^\s{0,3}>\s?(.*)$");
        private static readonly Regex TableRow = new Regex(@"^\s*\|.*\|\s*$");
        private static readonly Regex TableSeparator = new Regex(@"^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$");
        private static readonly Regex LinkPattern = new Regex(@"^\[([^\]]+)\]\(([^)\s]+)\)");

        public static List<MdBlock> Parse(string text)
        {
            var blocks = new List<MdBlock>();
            string[] lines = (text ?? "").Replace("\r\n", "\n").Replace('\r', '\n').Split('\n');
            for (int i = 0; i < lines.Length; i++)
            {
                string line = lines[i];
                if (Fence.IsMatch(line))
                {
                    var code = new MdBlock { Kind = MdBlockKind.Code, Lines = new List<string>() };
                    for (i++; i < lines.Length && !Fence.IsMatch(lines[i]); i++)
                    {
                        code.Lines.Add(lines[i].Replace("\t", "    "));
                    }
                    blocks.Add(code);
                    continue;
                }
                if (line.Trim().Length == 0)
                {
                    if (blocks.Count > 0 && blocks[blocks.Count - 1].Kind != MdBlockKind.Blank)
                    {
                        blocks.Add(new MdBlock { Kind = MdBlockKind.Blank });
                    }
                    continue;
                }
                if (TableRow.IsMatch(line))
                {
                    MdBlock table = blocks.Count > 0 && blocks[blocks.Count - 1].Kind == MdBlockKind.Table
                        ? blocks[blocks.Count - 1]
                        : null;
                    if (table == null)
                    {
                        table = new MdBlock { Kind = MdBlockKind.Table, Lines = new List<string>() };
                        blocks.Add(table);
                    }
                    if (!TableSeparator.IsMatch(line))
                    {
                        table.Lines.Add(line.Trim());
                    }
                    continue;
                }
                Match match;
                if (Rule.IsMatch(line))
                {
                    blocks.Add(new MdBlock { Kind = MdBlockKind.Rule });
                }
                else if ((match = Heading.Match(line)).Success)
                {
                    blocks.Add(new MdBlock { Kind = MdBlockKind.Heading, Level = match.Groups[1].Value.Length, Runs = Inline(match.Groups[2].Value) });
                }
                else if ((match = Bullet.Match(line)).Success)
                {
                    blocks.Add(new MdBlock { Kind = MdBlockKind.Bullet, Level = IndentLevel(match.Groups[1].Value), Runs = Inline(match.Groups[2].Value) });
                }
                else if ((match = Numbered.Match(line)).Success)
                {
                    blocks.Add(new MdBlock
                    {
                        Kind = MdBlockKind.Numbered,
                        Level = IndentLevel(match.Groups[1].Value),
                        Marker = match.Groups[2].Value + ".",
                        Runs = Inline(match.Groups[3].Value)
                    });
                }
                else if ((match = Quote.Match(line)).Success)
                {
                    blocks.Add(new MdBlock { Kind = MdBlockKind.Quote, Runs = Inline(match.Groups[1].Value) });
                }
                else
                {
                    blocks.Add(new MdBlock { Kind = MdBlockKind.Paragraph, Runs = Inline(line.Trim()) });
                }
            }
            while (blocks.Count > 0 && blocks[blocks.Count - 1].Kind == MdBlockKind.Blank)
            {
                blocks.RemoveAt(blocks.Count - 1);
            }
            foreach (MdBlock block in blocks)
            {
                if (block.Kind == MdBlockKind.Table)
                {
                    block.Lines = AlignTable(block.Lines);
                }
            }
            return blocks;
        }

        // "| a | bb |" -> các cột đệm cho thẳng (chữ đều Consolas), bỏ ký hiệu đậm/mã trong ô.
        private static List<string> AlignTable(List<string> rows)
        {
            var cells = new List<string[]>();
            int columns = 0;
            foreach (string row in rows)
            {
                string[] parts = row.Trim().Trim('|').Split('|');
                for (int i = 0; i < parts.Length; i++)
                {
                    var plain = new StringBuilder();
                    foreach (MdRun run in Inline(parts[i].Trim()))
                    {
                        plain.Append(run.Text);
                    }
                    parts[i] = plain.ToString();
                }
                cells.Add(parts);
                columns = Math.Max(columns, parts.Length);
            }
            var widths = new int[columns];
            foreach (string[] row in cells)
            {
                for (int i = 0; i < row.Length; i++)
                {
                    widths[i] = Math.Max(widths[i], row[i].Length);
                }
            }
            var lines = new List<string>();
            foreach (string[] row in cells)
            {
                var line = new StringBuilder();
                for (int i = 0; i < columns; i++)
                {
                    string cell = i < row.Length ? row[i] : "";
                    line.Append(cell.PadRight(widths[i]));
                    if (i < columns - 1)
                    {
                        line.Append("  ");
                    }
                }
                lines.Add(line.ToString().TrimEnd());
            }
            return lines;
        }

        private static int IndentLevel(string indent)
        {
            int spaces = indent.Replace("\t", "    ").Length;
            return Math.Min(4, spaces / 2);
        }

        // **đậm** / __đậm__, *nghiêng* / _nghiêng_ (gạch dưới chỉ khi không dính chữ: ten_bien giữ nguyên),
        // `mã` (không định dạng bên trong), [chữ](url), \* thoát ký tự.
        public static List<MdRun> Inline(string text)
        {
            var runs = new List<MdRun>();
            var buffer = new StringBuilder();
            bool bold = false;
            bool italic = false;
            int i = 0;
            Action flush = () =>
            {
                if (buffer.Length > 0)
                {
                    runs.Add(new MdRun { Text = buffer.ToString(), Bold = bold, Italic = italic });
                    buffer.Clear();
                }
            };
            while (i < text.Length)
            {
                char c = text[i];
                if (c == '\\' && i + 1 < text.Length && "\\`*_[]()#+-.!|>".IndexOf(text[i + 1]) >= 0)
                {
                    buffer.Append(text[i + 1]);
                    i += 2;
                    continue;
                }
                if (c == '`')
                {
                    int close = text.IndexOf('`', i + 1);
                    if (close > i + 1)
                    {
                        flush();
                        runs.Add(new MdRun { Text = text.Substring(i + 1, close - i - 1), Code = true, Bold = bold, Italic = italic });
                        i = close + 1;
                        continue;
                    }
                }
                if (c == '[')
                {
                    Match link = LinkPattern.Match(text.Substring(i));
                    if (link.Success)
                    {
                        flush();
                        runs.Add(new MdRun { Text = link.Groups[1].Value, Link = true, Bold = bold, Italic = italic });
                        i += link.Length;
                        continue;
                    }
                }
                if ((c == '*' || c == '_') && i + 1 < text.Length && text[i + 1] == c)
                {
                    if (c == '*' || IsBoundary(text, i, 2))
                    {
                        if (bold || text.IndexOf(new string(c, 2), i + 2, StringComparison.Ordinal) > i + 2)
                        {
                            flush();
                            bold = !bold;
                            i += 2;
                            continue;
                        }
                    }
                }
                else if (c == '*' || c == '_')
                {
                    bool opening = !italic && i + 1 < text.Length && !char.IsWhiteSpace(text[i + 1])
                        && text.IndexOf(c, i + 1) > i + 1;
                    bool closing = italic && i > 0 && !char.IsWhiteSpace(text[i - 1]);
                    if ((opening || closing) && (c == '*' || IsBoundary(text, i, 1)))
                    {
                        flush();
                        italic = !italic;
                        i++;
                        continue;
                    }
                }
                buffer.Append(c);
                i++;
            }
            flush();
            return runs;
        }

        // Dấu _ chỉ là định dạng khi không nằm giữa hai chữ/số (ten_bien_dai giữ nguyên).
        private static bool IsBoundary(string text, int index, int length)
        {
            bool before = index > 0 && char.IsLetterOrDigit(text[index - 1]);
            bool after = index + length < text.Length && char.IsLetterOrDigit(text[index + length]);
            return !(before && after);
        }

        // Chữ thuần (sao chép, đọc màn hình): bỏ ký hiệu, giữ dấu đầu dòng.
        public static string ToPlainText(string text)
        {
            var sb = new StringBuilder();
            foreach (MdBlock block in Parse(text))
            {
                switch (block.Kind)
                {
                    case MdBlockKind.Blank:
                        sb.AppendLine();
                        continue;
                    case MdBlockKind.Rule:
                        sb.AppendLine("—");
                        continue;
                    case MdBlockKind.Code:
                    case MdBlockKind.Table:
                        foreach (string line in block.Lines)
                        {
                            sb.AppendLine(line);
                        }
                        continue;
                }
                if (block.Kind == MdBlockKind.Bullet || block.Kind == MdBlockKind.Numbered)
                {
                    sb.Append(new string(' ', block.Level * 2)).Append(block.Kind == MdBlockKind.Bullet ? "• " : block.Marker + " ");
                }
                foreach (MdRun run in block.Runs)
                {
                    sb.Append(run.Text);
                }
                sb.AppendLine();
            }
            return sb.ToString().TrimEnd();
        }
    }

    // Kết quả xếp chữ: các mảnh chữ đã đặt vị trí + nền khối mã, vạch trích dẫn, đường kẻ.
    internal sealed class MdLayout
    {
        internal sealed class Piece
        {
            public string Text;
            public Font Font;
            public Color Color;
            public Point At;
            public bool CodeBack;
            public Size Size;
        }

        public readonly List<Piece> Pieces = new List<Piece>();
        public readonly List<Rectangle> CodeBlocks = new List<Rectangle>();
        public readonly List<Rectangle> QuoteBars = new List<Rectangle>();
        public readonly List<int> Rules = new List<int>();
        public int Width;
        public int Height;

        private const TextFormatFlags Flags = TextFormatFlags.NoPadding | TextFormatFlags.NoPrefix | TextFormatFlags.SingleLine;
        private static readonly Regex Tokens = new Regex(@"\s+|\S+");

        private static readonly Font Bold = new Font(PaneTheme.Body, FontStyle.Bold);
        private static readonly Font Italic = new Font(PaneTheme.Body, FontStyle.Italic);
        private static readonly Font BoldItalic = new Font(PaneTheme.Body, FontStyle.Bold | FontStyle.Italic);
        private static readonly Font Mono = new Font("Consolas", 9f);
        private static readonly Font H1 = new Font("Segoe UI Semibold", 11.5f);
        private static readonly Font H2 = new Font("Segoe UI Semibold", 10.5f);
        private static readonly Font H3 = new Font("Segoe UI Semibold", 9.5f);
        private static readonly Color CodeBg = Color.FromArgb(0xEE, 0xF0, 0xF3);
        private static readonly Color QuoteBar = Color.FromArgb(0xC7, 0xD2, 0xFE);

        private int _x;
        private int _y;
        private int _lineHeight;
        private readonly List<Piece> _line = new List<Piece>();

        public static MdLayout Build(List<MdBlock> blocks, int width, Color textColor)
        {
            var layout = new MdLayout();
            layout.Run(blocks, Math.Max(PaneTheme.Px(60), width), textColor);
            return layout;
        }

        private void Run(List<MdBlock> blocks, int width, Color textColor)
        {
            int indentStep = PaneTheme.Px(18);   // mục lồng thẳng với chữ của mục cha ("1." + khoảng cách)
            int gapParagraph = PaneTheme.Px(6);
            int gapItem = PaneTheme.Px(2);
            MdBlockKind previous = MdBlockKind.Blank;
            bool first = true;
            foreach (MdBlock block in blocks)
            {
                if (block.Kind == MdBlockKind.Blank)
                {
                    _y += gapParagraph;
                    previous = MdBlockKind.Blank;
                    continue;
                }
                if (!first)
                {
                    bool listContinues = (block.Kind == MdBlockKind.Bullet || block.Kind == MdBlockKind.Numbered)
                        && (previous == MdBlockKind.Bullet || previous == MdBlockKind.Numbered);
                    _y += block.Kind == MdBlockKind.Heading || block.Kind == MdBlockKind.Code || block.Kind == MdBlockKind.Table
                        ? gapParagraph
                        : listContinues ? gapItem : PaneTheme.Px(3);
                }
                first = false;
                previous = block.Kind;
                switch (block.Kind)
                {
                    case MdBlockKind.Rule:
                        _y += PaneTheme.Px(4);
                        Rules.Add(_y);
                        _y += PaneTheme.Px(5);
                        break;
                    case MdBlockKind.Code:
                    case MdBlockKind.Table:
                        Monospace(block, width, textColor);
                        break;
                    case MdBlockKind.Heading:
                        Flow(block.Runs, 0, width, textColor, block.Level == 1 ? H1 : block.Level == 2 ? H2 : H3);
                        break;
                    case MdBlockKind.Quote:
                    {
                        int top = _y;
                        Flow(block.Runs, PaneTheme.Px(10), width, PaneTheme.TextSecondary, null);
                        QuoteBars.Add(new Rectangle(0, top, PaneTheme.Px(3), _y - top));
                        break;
                    }
                    case MdBlockKind.Bullet:
                    case MdBlockKind.Numbered:
                    {
                        int markerLeft = block.Level * indentStep;
                        string marker = block.Kind == MdBlockKind.Bullet ? (block.Level == 0 ? "•" : "◦") : block.Marker;
                        Size markerSize = Measure(marker, PaneTheme.Body);
                        int textLeft = markerLeft + Math.Max(indentStep, markerSize.Width + PaneTheme.Px(5));
                        Add(new Piece { Text = marker, Font = PaneTheme.Body, Color = PaneTheme.TextMuted, At = new Point(markerLeft, _y), Size = markerSize });
                        Flow(block.Runs, textLeft, width, textColor, null);
                        break;
                    }
                    default:
                        Flow(block.Runs, 0, width, textColor, null);
                        break;
                }
            }
            Height = _y;
            foreach (Piece piece in Pieces)
            {
                Width = Math.Max(Width, piece.At.X + piece.Size.Width);
            }
            // Nền khối mã kéo hết bề ngang nội dung (sau khi biết bề ngang thật).
            for (int i = 0; i < CodeBlocks.Count; i++)
            {
                Rectangle rect = CodeBlocks[i];
                CodeBlocks[i] = new Rectangle(rect.X, rect.Y, Math.Max(rect.Width, Width - rect.X), rect.Height);
            }
        }

        private void Monospace(MdBlock block, int width, Color textColor)
        {
            int pad = PaneTheme.Px(6);
            int top = _y;
            _y += pad;
            foreach (string line in block.Lines.Count == 0 ? new List<string> { "" } : block.Lines)
            {
                // Dòng dài thì cắt theo ký tự (mã/bảng không có chỗ ngắt từ hợp lý).
                string rest = line.Length == 0 ? " " : line;
                while (rest.Length > 0)
                {
                    int take = Fit(rest, Mono, width - 2 * pad);
                    string part = rest.Substring(0, take);
                    Size size = Measure(part, Mono);
                    Pieces.Add(new Piece { Text = part, Font = Mono, Color = textColor, At = new Point(pad, _y), Size = size, CodeBack = true });
                    _y += Math.Max(size.Height, Mono.Height);
                    rest = rest.Substring(take);
                }
            }
            _y += pad;
            CodeBlocks.Add(new Rectangle(0, top, 0, _y - top));
        }

        // Xếp các đoạn chữ thành dòng trong [left, width), xuống dòng ở khoảng trắng; từ dài hơn cả dòng thì cắt.
        // Đo CẢ CỤM chữ cùng định dạng trên dòng (không đo từng từ rồi cộng): TextRenderer cộng lề cho mỗi lần đo,
        // cộng từng từ thì khoảng cách giữa các chữ rộng gấp mấy lần (đo 13/10/2026).
        private Piece _segment;

        private void Flow(List<MdRun> runs, int left, int width, Color textColor, Font heading)
        {
            _x = left;
            _lineHeight = 0;
            _segment = null;
            foreach (MdRun run in runs)
            {
                Font font = heading ?? (run.Code ? Mono : run.Bold && run.Italic ? BoldItalic : run.Bold ? Bold : run.Italic ? Italic : PaneTheme.Body);
                Color color = run.Link ? PaneTheme.AccentFg : textColor;
                foreach (Match token in Tokens.Matches(run.Text ?? ""))
                {
                    string text = token.Value;
                    if (char.IsWhiteSpace(text[0]))
                    {
                        if (_x == left && (_segment == null || _segment.Text.Trim().Length == 0))
                        {
                            continue;
                        }
                        Append(" ", font, color, run.Code);
                        continue;
                    }
                    if (StartX(font, color, run.Code) + TextWidth(Pending(font, color, run.Code) + text, font) > width && LineHasText())
                    {
                        NewLine(left);
                    }
                    // Từ dài hơn cả dòng: cắt theo ký tự.
                    while (left + TextWidth(text, font) > width && text.Length > 1)
                    {
                        int room = width - StartX(font, color, run.Code) - TextWidth(Pending(font, color, run.Code), font);
                        int take = Math.Max(1, Fit(text, font, room));
                        Append(text.Substring(0, take), font, color, run.Code);
                        NewLine(left);
                        text = text.Substring(take);
                    }
                    Append(text, font, color, run.Code);
                }
            }
            if (!LineHasText() && _line.Count == 0)
            {
                _lineHeight = (heading ?? PaneTheme.Body).Height;
            }
            NewLine(left);
        }

        private bool SameStyle(Font font, Color color, bool code)
        {
            return _segment != null && _segment.Font == font && _segment.Color == color && _segment.CodeBack == code;
        }

        private string Pending(Font font, Color color, bool code)
        {
            return SameStyle(font, color, code) ? _segment.Text : "";
        }

        // Toạ độ x mà cụm chữ kiểu này bắt đầu: cụm đang mở cùng kiểu thì từ đầu cụm, khác kiểu thì sau cụm đó.
        private int StartX(Font font, Color color, bool code)
        {
            if (_segment == null)
            {
                return _x;
            }
            return SameStyle(font, color, code) ? _segment.At.X : _segment.At.X + TextWidth(_segment.Text, _segment.Font);
        }

        private bool LineHasText()
        {
            return _line.Count > 0 || (_segment != null && _segment.Text.Trim().Length > 0);
        }

        private void Append(string text, Font font, Color color, bool code)
        {
            if (!SameStyle(font, color, code))
            {
                CloseSegment();
                _segment = new Piece { Text = "", Font = font, Color = color, At = new Point(_x, _y), CodeBack = code };
            }
            _segment.Text += text;
        }

        private void CloseSegment()
        {
            if (_segment == null)
            {
                return;
            }
            Piece piece = _segment;
            _segment = null;
            int width = TextWidth(piece.Text, piece.Font);
            _x = piece.At.X + width;
            string shown = piece.Text.TrimEnd();
            if (shown.Length == 0)
            {
                return;
            }
            piece.Text = shown;
            piece.Size = new Size(TextWidth(shown, piece.Font), Measure(shown, piece.Font).Height);
            _line.Add(piece);
            _lineHeight = Math.Max(_lineHeight, Math.Max(piece.Size.Height, piece.Font.Height));
        }

        // Bề ngang chữ kể cả dấu cách cuối (TextRenderer bỏ qua khoảng trắng cuối khi đo).
        private static int TextWidth(string text, Font font)
        {
            if (string.IsNullOrEmpty(text))
            {
                return 0;
            }
            string trimmed = text.TrimEnd();
            int trailing = text.Length - trimmed.Length;
            return (trimmed.Length > 0 ? Math.Max(1, Measure(trimmed, font).Width - Overhead(font)) : 0) + trailing * SpaceWidth(font);
        }

        // Mỗi lần đo TextRenderer cộng một khoản lề cố định (kể cả NoPadding): Measure("x")*2 - Measure("xx").
        // Không trừ thì chỗ đổi kiểu chữ ("**26 lỗi** trình") hở thêm một khe.
        private static readonly Dictionary<Font, int> Overheads = new Dictionary<Font, int>();

        private static int Overhead(Font font)
        {
            int overhead;
            if (!Overheads.TryGetValue(font, out overhead))
            {
                overhead = Math.Max(0, 2 * Measure("x", font).Width - Measure("xx", font).Width);
                Overheads[font] = overhead;
            }
            return overhead;
        }

        private static readonly Dictionary<Font, int> SpaceWidths = new Dictionary<Font, int>();

        private static int SpaceWidth(Font font)
        {
            int width;
            if (!SpaceWidths.TryGetValue(font, out width))
            {
                width = Math.Max(1, Measure("x x", font).Width - Measure("xx", font).Width);
                SpaceWidths[font] = width;
            }
            return width;
        }

        // Chốt một dòng: căn đáy các mảnh khác cỡ chữ (tiêu đề lẫn mã trong dòng) theo dòng cao nhất.
        private void NewLine(int left)
        {
            CloseSegment();
            foreach (Piece piece in _line)
            {
                int height = Math.Max(piece.Size.Height, piece.Font.Height);
                piece.At = new Point(piece.At.X, _y + (_lineHeight - height));
                Pieces.Add(piece);
            }
            if (_line.Count > 0 || _lineHeight > 0)
            {
                _y += _lineHeight;
            }
            _line.Clear();
            _x = left;
            _lineHeight = 0;
        }

        private void Add(Piece piece)
        {
            Pieces.Add(piece);
        }

        private static int Fit(string text, Font font, int width)
        {
            int count = 1;
            while (count < text.Length && Measure(text.Substring(0, count + 1), font).Width <= width)
            {
                count++;
            }
            return count;
        }

        private static Size Measure(string text, Font font)
        {
            return TextRenderer.MeasureText(text, font, new Size(int.MaxValue, int.MaxValue), Flags);
        }

        public void Paint(Graphics g, Point origin)
        {
            using (var back = new SolidBrush(CodeBg))
            {
                foreach (Rectangle rect in CodeBlocks)
                {
                    g.FillRectangle(back, rect.X + origin.X, rect.Y + origin.Y, rect.Width, rect.Height);
                }
                foreach (Piece piece in Pieces)
                {
                    if (piece.CodeBack && piece.Font == Mono && !InCodeBlock(piece))
                    {
                        g.FillRectangle(back, piece.At.X + origin.X - 1, piece.At.Y + origin.Y, piece.Size.Width + 2, piece.Size.Height);
                    }
                }
            }
            using (var bar = new SolidBrush(QuoteBar))
            {
                foreach (Rectangle rect in QuoteBars)
                {
                    g.FillRectangle(bar, rect.X + origin.X, rect.Y + origin.Y, rect.Width, rect.Height);
                }
            }
            using (var pen = new Pen(PaneTheme.Divider))
            {
                foreach (int y in Rules)
                {
                    g.DrawLine(pen, origin.X, origin.Y + y, origin.X + Width, origin.Y + y);
                }
            }
            foreach (Piece piece in Pieces)
            {
                TextRenderer.DrawText(g, piece.Text, piece.Font, new Point(piece.At.X + origin.X, piece.At.Y + origin.Y), piece.Color, Flags);
            }
        }

        private bool InCodeBlock(Piece piece)
        {
            foreach (Rectangle rect in CodeBlocks)
            {
                if (piece.At.Y >= rect.Y && piece.At.Y < rect.Bottom)
                {
                    return true;
                }
            }
            return false;
        }
    }
}

using System;
using System.Collections.Generic;
using System.Drawing;
using System.Linq;
using System.Threading;
using System.Windows.Forms;
using AxiomOffice.Bridge;

namespace AxiomOffice.Ai
{
    [System.Runtime.InteropServices.ComVisible(true)]
    [System.Runtime.InteropServices.Guid("8001B0D7-F189-443A-B3CB-6EB98038C72E")]
    [System.Runtime.InteropServices.ProgId("AxiomOffice.AskAiPane")]
    [System.Runtime.InteropServices.ClassInterface(System.Runtime.InteropServices.ClassInterfaceType.AutoDispatch)]
    public class AskAiPane : UserControl
    {
        // Chữ giao diện theo ngôn ngữ đang chọn (Ui.T) - thuộc tính, không phải hằng, để đổi ngôn ngữ là đổi ngay.
        private static string PromptPlaceholder { get { return Ui.T("Nhập yêu cầu cho tài liệu này…"); } }
        private static string PlanPlaceholder { get { return Ui.T("Mô tả việc cần làm - AI lập kế hoạch trước, chưa sửa gì…"); } }
        // Nói rõ dùng phương án mặc định cho câu hỏi chưa trả lời (Core: PlanModeRule bắt kế hoạch ghi mặc định an
        // toàn). Lượt 13/10/2026 bấm thực hiện khi còn 3 câu hỏi bỏ ngỏ, model tự chọn phạm vi lớn nhất.
        // Câu gửi cho AI theo ngôn ngữ giao diện: AI trả lời theo ngôn ngữ của câu hỏi.
        private static string ExecutePlanPrompt { get { return Ui.T("Thực hiện kế hoạch ở trên. Câu hỏi nào tôi chưa trả lời thì dùng phương án mặc định đã ghi trong kế hoạch."); } }
        private const int MinPaneWidth = 300;
        private const int TargetPaneWidth = 360;

        internal static Connect CurrentHost;

        private readonly ChatList _chat;
        private readonly ComposerPanel _composer;
        private readonly PromptBox _prompt;
        private readonly Label _placeholder;
        private readonly Label _hint;
        private readonly SendButton _send;
        private readonly Label _headerSubtitle;
        private readonly LinkLabel _settingsLink;
        private readonly LinkLabel _moreLink;
        private readonly Label _status;
        private readonly AccentDot _busyDot;
        private readonly LinkLabel _insertLink;
        private readonly LinkLabel _stopLink;
        private readonly LinkLabel _newChatLink;
        private readonly LinkLabel _undoLink;
        // Plan mode: lượt chạy chỉ đọc, trả về kế hoạch; thẻ dưới kế hoạch cho chạy thật (options.mode của Core).
        private readonly LinkLabel _planToggle;
        private bool _planMode;
        private bool _lastPlan;
        private PlanCard _planCard;
        // Đính kèm tệp Office: Core đọc bằng công cụ MCP đọc tệp của server "office" theo đường dẫn đầy đủ.
        private const int MaxAttachments = 3;
        private readonly List<string> _attachments = new List<string>();
        private readonly LinkLabel _attachLink;
        private readonly FlowLayoutPanel _attachStrip;
        private List<string> _lastAttachments = new List<string>();
        private int _runEdits;                       // so thao tac writer.* sua tai lieu thanh cong trong luot (Word)
        private int _lastRunEdits;
        private EmptyState _empty;
        private string _conversationId;              // hoi thoai ben Agent Core, theo tai lieu dang mo
        private bool _freshConversation;             // nguoi dung vua bam "Cuoc tro chuyen moi"
        private bool _lastRunViaCore;                // luot vua roi da chay qua Core
        private string _lastReply = "";
        private string _lastPrompt = "";
        private string _blockedReason;
        private bool _busy;
        private int _toolCount;
        private CancellationTokenSource _run;
        private int _widthChecks;

        public AskAiPane() : this(null)
        {
        }

        internal AskAiPane(Connect connect)
        {
            if (connect != null)
            {
                CurrentHost = connect;
            }
            Text = "AxiomOfficePane";
            Dock = DockStyle.Fill;
            Font = PaneTheme.Body;
            BackColor = PaneTheme.PaneBg;

            _chat = new ChatList { Dock = DockStyle.Fill };

            _composer = new ComposerPanel { Dock = DockStyle.Bottom, Height = PaneTheme.Px(PaneTheme.ComposerH) };
            _prompt = new PromptBox
            {
                Multiline = true,
                AcceptsReturn = true,
                BorderStyle = BorderStyle.None,
                ScrollBars = ScrollBars.None,
                Font = PaneTheme.Body,
                BackColor = PaneTheme.PaneBg,
                ForeColor = PaneTheme.TextPrimary,
                AccessibleName = Ui.T("Yêu cầu cho AI"),
            };
            _placeholder = PaneTheme.MakeLabel(PromptPlaceholder, PaneTheme.Body, PaneTheme.TextMuted);
            _placeholder.Cursor = Cursors.IBeam;
            _placeholder.AccessibleRole = AccessibleRole.StaticText;
            _hint = PaneTheme.MakeLabel(Ui.T("Enter để gửi · Shift+Enter xuống dòng"), PaneTheme.Caption, PaneTheme.TextMuted);
            _hint.AutoEllipsis = true;
            _hint.TextAlign = ContentAlignment.MiddleLeft;
            _send = new SendButton();
            _composer.Controls.Add(_placeholder);
            _composer.Controls.Add(_prompt);
            _composer.Controls.Add(_hint);
            // Công tắc lập kế hoạch đứng chỗ dòng gợi ý phím (pane hẹp không đủ chỗ cho cả hai); gợi ý vào tooltip.
            _planToggle = PaneTheme.MakeLink("", PaneTheme.Caption);
            _composer.Controls.Add(_planToggle);
            _hint.Visible = false;
            _attachLink = PaneTheme.MakeLink(Ui.T("+ Đính kèm"), PaneTheme.Caption);
            _attachLink.LinkColor = PaneTheme.TextMuted;
            _attachLink.AccessibleName = Ui.T("Đính kèm tệp Word, Excel hoặc PowerPoint cho AI đọc");
            _composer.Controls.Add(_attachLink);
            _attachStrip = new FlowLayoutPanel
            {
                Dock = DockStyle.Bottom,
                Height = PaneTheme.Px(24),
                BackColor = PaneTheme.PaneBg,
                Padding = new Padding(PaneTheme.Px(PaneTheme.PadX), 0, PaneTheme.Px(PaneTheme.PadX), 0),
                WrapContents = false,
                AutoScroll = false,
                Visible = false,
            };
            _composer.Controls.Add(_send);

            var footer = new Panel
            {
                Dock = DockStyle.Bottom,
                Height = PaneTheme.Px(PaneTheme.FooterH),
                BackColor = PaneTheme.PaneBg,
                Padding = new Padding(PaneTheme.Px(PaneTheme.PadX), 0, PaneTheme.Px(PaneTheme.PadX), 0),
            };
            _status = PaneTheme.MakeLabel("", PaneTheme.Caption, PaneTheme.TextMuted);
            _status.Dock = DockStyle.Fill;
            _status.AutoEllipsis = true;
            _status.TextAlign = ContentAlignment.MiddleLeft;
            _status.AccessibleName = Ui.T("Trạng thái");
            _busyDot = new AccentDot(PaneTheme.Px(6)) { Dock = DockStyle.Left, Width = PaneTheme.Px(12), Visible = false };
            _insertLink = MakeFooterLink(Ui.T("Chèn trả lời"));
            _stopLink = MakeFooterLink(Ui.T("Dừng"));
            _undoLink = MakeFooterLink(Ui.T("Hoàn tác lượt này"));
            _undoLink.AccessibleName = Ui.T("Hoàn tác toàn bộ thay đổi của lượt AI vừa rồi");
            footer.Controls.Add(_status);
            footer.Controls.Add(_busyDot);
            footer.Controls.Add(_insertLink);
            footer.Controls.Add(_stopLink);
            footer.Controls.Add(_undoLink);

            var header = new DividerPanel { Dock = DockStyle.Top, Height = PaneTheme.Px(PaneTheme.HeaderH) };
            var dot = new AccentDot(PaneTheme.Px(8));
            dot.Location = new Point(PaneTheme.Px(PaneTheme.PadX), (header.Height - dot.Height) / 2);
            var title = PaneTheme.MakeLabel("Axiom Office", PaneTheme.Title, PaneTheme.TextPrimary);
            title.AutoSize = true;
            title.Location = new Point(PaneTheme.Px(28), PaneTheme.Px(7));
            _headerSubtitle = PaneTheme.MakeLabel("", PaneTheme.Caption, PaneTheme.TextMuted);
            _headerSubtitle.AutoEllipsis = true;
            _settingsLink = PaneTheme.MakeLink(Ui.T("Thiết lập"), PaneTheme.Caption);
            _settingsLink.AccessibleName = Ui.T("Mở wizard thiết lập Axiom Office");
            // docs/architecture/core.md mục 9.2: link ở header (trước đây ở footer, người dùng không thấy trong Word).
            _newChatLink = PaneTheme.MakeLink(Ui.T("Trò chuyện mới"), PaneTheme.Caption);
            _newChatLink.AccessibleName = Ui.T("Bắt đầu cuộc trò chuyện mới cho tài liệu này");
            // Hội thoại trước / nhật ký hoạt động: thuộc về khung chat, không đặt trên ribbon (docs/architecture/ribbon-ui.md).
            _moreLink = PaneTheme.MakeLink("⋯", PaneTheme.Caption);
            _moreLink.AccessibleName = Ui.T("Hội thoại trước và nhật ký hoạt động");
            header.Controls.Add(dot);
            header.Controls.Add(title);
            header.Controls.Add(_headerSubtitle);
            header.Controls.Add(_settingsLink);
            header.Controls.Add(_newChatLink);
            header.Controls.Add(_moreLink);
            header.Resize += delegate { LayoutHeader(header); };

            Controls.Add(_chat);
            Controls.Add(_attachStrip);   // thêm trước composer: nằm ngay TRÊN ô nhập
            Controls.Add(_composer);
            Controls.Add(footer);
            Controls.Add(header);

            _send.Click += OnSend;
            _prompt.KeyDown += OnPromptKeyDown;
            _prompt.TextChanged += delegate { OnPromptChanged(); };
            _prompt.Enter += delegate { _composer.FocusedLook = true; };
            _prompt.Leave += delegate { _composer.FocusedLook = false; };
            _placeholder.Click += delegate { OnComposerClick(); };
            _composer.Click += delegate { OnComposerClick(); };
            _insertLink.LinkClicked += OnInsert;
            _stopLink.LinkClicked += delegate { OnStop(); };
            _newChatLink.LinkClicked += delegate { StartNewConversation(); };
            _undoLink.LinkClicked += delegate { UndoLastRun(); };
            _settingsLink.LinkClicked += delegate { OpenSetup(); };
            _planToggle.LinkClicked += delegate { SetPlanMode(!_planMode); };
            _attachLink.LinkClicked += delegate { PickAttachments(); };
            _promptTip.SetToolTip(_prompt, Ui.T("Enter để gửi · Shift+Enter xuống dòng"));
            Ui.Changed += OnUiLanguageChanged;
            ChatBubble.CitationClicked += OnCitationClicked;
            SetPlanMode(false);
            _moreLink.LinkClicked += delegate { ShowMoreMenu(); };
            _composer.Resize += delegate { LayoutComposer(); };
            Load += delegate
            {
                LayoutHeader(header);
                LayoutComposer();
                RefreshState();
                OfferSetupOnce();
                _prompt.Focus();
                ScheduleWidthCheck();
            };

            AddGreeting();
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing)
            {
                Ui.Changed -= OnUiLanguageChanged;
                ChatBubble.CitationClicked -= OnCitationClicked;
                CommandDispatcher.UnlockSelection();
            }
            if (disposing && _run != null)
            {
                _run.Cancel();
            }
            base.Dispose(disposing);
        }

        // Bấm trích dẫn trong câu trả lời ([§12], [Data!C42], [slide 3]): chọn + cuộn tới chỗ đó (writer.goto / et.goto /
        // wpp.goto - chỉ đọc, gọi thẳng bridge như "Hoàn tác lượt này"). Event là static: chỉ nhận bong bóng của pane này.
        private void OnCitationClicked(ChatBubble bubble, string target)
        {
            Connect host = CurrentHost;
            if (host == null || _busy || bubble == null || !_chat.Contains(bubble) || string.IsNullOrWhiteSpace(target))
            {
                return;
            }
            string text = target.Trim();
            string action;
            var parameters = new Dictionary<string, object>();
            int number;
            if (text.StartsWith("§", StringComparison.Ordinal))
            {
                if (!int.TryParse(text.Substring(1).Trim(), out number))
                {
                    return;
                }
                action = "writer.goto";
                parameters["paragraph"] = number;
            }
            else if (text.StartsWith("slide", StringComparison.OrdinalIgnoreCase))
            {
                if (!int.TryParse(text.Substring(5).Trim(), out number))
                {
                    return;
                }
                action = "wpp.goto";
                parameters["slide"] = number;
            }
            else
            {
                action = "et.goto";
                parameters["range"] = text;
            }
            try
            {
                object reply = CommandDispatcher.Execute(host, action, parameters);
                Logger.Info("AskAiPane: citation " + text + " -> " + action + " " + Truncate(new System.Web.Script.Serialization.JavaScriptSerializer().Serialize(reply), 160));
            }
            catch (Exception ex)
            {
                Logger.Error("AskAiPane: citation " + text + " failed", ex);
            }
        }

        private readonly ToolTip _promptTip = new ToolTip();

        // Đổi ngôn ngữ giao diện (ribbon, wizard; có thể từ luồng khác): đặt lại chữ các nút và nhãn cố định. Tin nhắn
        // đã có trong khung chat giữ nguyên ngôn ngữ lúc hiện.
        private void OnUiLanguageChanged(object sender, EventArgs e)
        {
            PostToUi(ApplyLanguage);
        }

        private void ApplyLanguage()
        {
            _prompt.AccessibleName = Ui.T("Yêu cầu cho AI");
            _hint.Text = Ui.T("Enter để gửi · Shift+Enter xuống dòng");
            _promptTip.SetToolTip(_prompt, _hint.Text);
            _attachLink.AccessibleName = Ui.T("Đính kèm tệp Word, Excel hoặc PowerPoint cho AI đọc");
            _status.AccessibleName = Ui.T("Trạng thái");
            SetFooterText(_insertLink, Ui.T("Chèn trả lời"));
            SetFooterText(_stopLink, Ui.T("Dừng"));
            SetFooterText(_undoLink, Ui.T("Hoàn tác lượt này"));
            _undoLink.AccessibleName = Ui.T("Hoàn tác toàn bộ thay đổi của lượt AI vừa rồi");
            _settingsLink.Text = Ui.T("Thiết lập");
            _settingsLink.AccessibleName = Ui.T("Mở wizard thiết lập Axiom Office");
            _newChatLink.Text = Ui.T("Trò chuyện mới");
            _newChatLink.AccessibleName = Ui.T("Bắt đầu cuộc trò chuyện mới cho tài liệu này");
            _moreLink.AccessibleName = Ui.T("Hội thoại trước và nhật ký hoạt động");
            SetPlanMode(_planMode);
            RefreshAttachments();
            // Chưa gửi gì (còn màn chào): vẽ lại màn chào và gợi ý bằng ngôn ngữ mới.
            if (_empty != null)
            {
                _chat.ClearAll();
                AddGreeting();
            }
            RefreshState();
            LayoutComposer();
        }

        private static void SetFooterText(LinkLabel link, string text)
        {
            link.Text = text;
            link.Width = TextRenderer.MeasureText(text, PaneTheme.Caption).Width + PaneTheme.Px(8);
        }

        private LinkLabel MakeFooterLink(string text)
        {
            LinkLabel link = PaneTheme.MakeLink(text, PaneTheme.Caption);
            link.AutoSize = false;
            link.Dock = DockStyle.Right;
            link.TextAlign = ContentAlignment.MiddleRight;
            link.Width = TextRenderer.MeasureText(text, PaneTheme.Caption).Width + PaneTheme.Px(8);
            link.Visible = false;
            return link;
        }

        private void LayoutHeader(Control header)
        {
            int padX = PaneTheme.Px(PaneTheme.PadX);
            _settingsLink.Location = new Point(header.ClientSize.Width - padX - _settingsLink.Width, (header.ClientSize.Height - _settingsLink.Height) / 2);
            _moreLink.Location = new Point(_settingsLink.Left - PaneTheme.Px(10) - _moreLink.Width, _settingsLink.Top);
            _newChatLink.Location = new Point(_moreLink.Left - PaneTheme.Px(10) - _newChatLink.Width, _settingsLink.Top);
            int right = _newChatLink.Visible ? _newChatLink.Left : _moreLink.Left;
            int left = PaneTheme.Px(28);
            _headerSubtitle.SetBounds(left, PaneTheme.Px(26), Math.Max(PaneTheme.Px(40), right - PaneTheme.Px(8) - left), PaneTheme.Px(16));
        }

        private void LayoutComposer()
        {
            int width = _composer.ClientSize.Width;
            int height = _composer.ClientSize.Height;
            Rectangle box = _composer.BoxBounds;
            int innerLeft = box.Left + PaneTheme.Px(12);
            int innerRight = box.Right - PaneTheme.Px(8);
            int inset = SendButton.Inset;
            int faceX = innerRight - PaneTheme.Px(PaneTheme.AskW);
            int faceY = height - PaneTheme.Px(8) - PaneTheme.Px(PaneTheme.AskH);
            _send.Location = new Point(faceX - inset, faceY - inset);
            int textTop = box.Top + PaneTheme.Px(10);
            int textHeight = Math.Max(PaneTheme.Px(18), faceY - PaneTheme.Px(6) - textTop);
            _prompt.SetBounds(innerLeft, textTop, Math.Max(PaneTheme.Px(40), innerRight - innerLeft), textHeight);
            _placeholder.SetBounds(innerLeft + 1, textTop, Math.Max(PaneTheme.Px(40), innerRight - innerLeft - 1), textHeight);
            _hint.SetBounds(innerLeft, faceY, Math.Max(0, faceX - inset - PaneTheme.Px(4) - innerLeft), PaneTheme.Px(PaneTheme.AskH));
            _planToggle.Location = new Point(innerLeft, faceY + (PaneTheme.Px(PaneTheme.AskH) - _planToggle.Height) / 2);
            _attachLink.Location = new Point(_planToggle.Right + PaneTheme.Px(10), _planToggle.Top);
        }

        // Ô nhập cao theo nội dung từ 2 đến 5 dòng; quá 5 dòng thì TextBox tự cuộn theo con trỏ.
        private void UpdateComposerHeight()
        {
            string text = _prompt.Text;
            if (text.Length == 0 || text.EndsWith("\n", StringComparison.Ordinal))
            {
                text += " ";
            }
            int needed = PaneTheme.MeasureHeight(text, PaneTheme.Body, Math.Max(1, _prompt.ClientSize.Width - 4));
            int chrome = PaneTheme.Px(PaneTheme.ComposerH) - PaneTheme.Px(PaneTheme.ComposerTextMinH);
            int textHeight = Math.Max(PaneTheme.Px(PaneTheme.ComposerTextMinH), Math.Min(needed, PaneTheme.Px(PaneTheme.ComposerMaxH) - chrome));
            int height = chrome + textHeight;
            if (_composer.Height != height)
            {
                _composer.Height = height;
            }
        }

        private void OnPromptChanged()
        {
            UpdatePlaceholder();
            UpdateSendEnabled();
            UpdateComposerHeight();
        }

        private void OnComposerClick()
        {
            if (_blockedReason != null)
            {
                if (CurrentHost != null)
                {
                    OpenSetup();
                }
                return;
            }
            _prompt.Focus();
        }

        private void UpdatePlaceholder()
        {
            _placeholder.Visible = _prompt.TextLength == 0;
        }

        private void UpdateSendEnabled()
        {
            _send.Enabled = !_busy && _blockedReason == null && _prompt.Text.Trim().Length > 0;
        }

        // Khoá composer khi chắc chắn gửi sẽ thất bại (chưa cấu hình AI, mất host); ngược lại mở.
        private void RefreshState()
        {
            string model = (Config.LlmModel ?? "").Trim();
            string endpoint = (Config.LlmEndpoint ?? "").Trim();
            _headerSubtitle.Text = HostLabel() + " · " + (model.Length > 0 ? model : Ui.T("Chưa chọn model"));
            _newChatLink.Visible = CoreClient.Instance.Enabled;
            if (_newChatLink.Parent != null)
            {
                LayoutHeader(_newChatLink.Parent);
            }

            string reason = null;
            if (CurrentHost == null)
            {
                reason = Ui.T("Mất kết nối với ứng dụng. Mở lại tài liệu để tiếp tục.");
            }
            else if (endpoint.Length == 0 || model.Length == 0)
            {
                reason = Ui.T("Chưa thiết lập AI — bấm Thiết lập ở trên");
            }
            _blockedReason = reason;
            bool blocked = reason != null;
            _composer.Blocked = blocked;
            _prompt.Enabled = !blocked;
            Color surface = blocked ? PaneTheme.InputDisabled : PaneTheme.PaneBg;
            _prompt.BackColor = surface;
            _placeholder.BackColor = surface;
            _placeholder.Text = blocked ? reason : (_planMode ? PlanPlaceholder : PromptPlaceholder);
            _placeholder.Cursor = blocked ? Cursors.Hand : Cursors.IBeam;
            _planToggle.Visible = !blocked;
            _attachLink.Visible = !blocked && CoreClient.Instance.Enabled;
            UpdatePlaceholder();
            UpdateSendEnabled();
        }

        private static string HostKind()
        {
            return CurrentHost != null ? CurrentHost.AppKind : "wps";
        }

        private static string HostLabel()
        {
            string kind = HostKind();
            return kind == "et" ? Ui.T("Bảng tính") : kind == "wpp" ? Ui.T("Trình chiếu") : Ui.T("Văn bản");
        }

        private void AddGreeting()
        {
            string kind = HostKind();
            string noun = kind == "et" ? Ui.T("bảng tính") : kind == "wpp" ? Ui.T("bản trình chiếu") : Ui.T("văn bản");
            _empty = new EmptyState(
                Ui.T("Bạn muốn làm gì với ") + noun + Ui.T(" này?"),
                Ui.T("Tôi đọc và chỉnh sửa trực tiếp tài liệu đang mở. Thay đổi hiện ngay trên trang khi tôi làm."),
                GetSuggestions(kind));
            _empty.SuggestionClicked += delegate(string suggestion)
            {
                if (_blockedReason != null)
                {
                    return;
                }
                _prompt.Text = suggestion;
                _prompt.Focus();
                _prompt.SelectionStart = _prompt.TextLength;
            };
            _chat.AddBlock(_empty);
        }

        private static string[] GetSuggestions(string kind)
        {
            if (kind == "et")
            {
                return new[] { Ui.T("Tạo bảng điểm 5 học sinh, có cột trung bình"), Ui.T("Thêm cột Tổng có công thức cho bảng này"), Ui.T("Tô đậm và kẻ khung dòng tiêu đề") };
            }
            if (kind == "wpp")
            {
                return new[] { Ui.T("Tạo 5 slide giới thiệu công ty"), Ui.T("Thêm slide kế hoạch quý 4"), Ui.T("Viết ghi chú thuyết trình cho từng slide") };
            }
            return new[] { Ui.T("Soạn đơn xin việc vị trí kế toán"), Ui.T("Tóm tắt tài liệu này thành 5 ý"), Ui.T("Chèn bảng lịch họp tuần 3 cột") };
        }

        private void OnPromptKeyDown(object sender, KeyEventArgs e)
        {
            if (e.KeyCode == Keys.Enter && !e.Shift)
            {
                e.SuppressKeyPress = true;
                OnSend(sender, EventArgs.Empty);
            }
        }

        private void OnSend(object sender, EventArgs e)
        {
            if (_busy)
            {
                return;
            }
            RefreshState();
            if (_blockedReason != null)
            {
                return;
            }
            string promptText = (_prompt.Text ?? "").Trim();
            if (promptText.Length == 0)
            {
                return;
            }
            _prompt.Clear();
            StartRun(promptText, _planMode, new List<string>(_attachments));
            _attachments.Clear();
            RefreshAttachments();
        }

        private void PickAttachments()
        {
            using (var dialog = new OpenFileDialog
            {
                Title = Ui.T("Đính kèm tệp cho AI đọc"),
                Filter = Ui.T("Tài liệu Office (*.docx;*.xlsx;*.pptx)|*.docx;*.xlsx;*.pptx"),
                Multiselect = true,
                CheckFileExists = true,
            })
            {
                if (dialog.ShowDialog(this) != DialogResult.OK)
                {
                    return;
                }
                foreach (string path in dialog.FileNames)
                {
                    if (_attachments.Count >= MaxAttachments)
                    {
                        SetStatus(Ui.T("Tối đa ") + MaxAttachments + Ui.T(" tệp mỗi lượt"), true);
                        break;
                    }
                    if (!_attachments.Contains(path, StringComparer.OrdinalIgnoreCase))
                    {
                        _attachments.Add(path);
                    }
                }
            }
            Logger.Info("AskAiPane: attachments=" + _attachments.Count);
            RefreshAttachments();
            _prompt.Focus();
        }

        // Hàng tệp đính kèm trên ô nhập: "📄 tên ✕", bấm để bỏ tệp đó.
        private void RefreshAttachments()
        {
            _attachStrip.SuspendLayout();
            foreach (Control old in _attachStrip.Controls.Cast<Control>().ToList())
            {
                _attachStrip.Controls.Remove(old);
                old.Dispose();
            }
            foreach (string path in _attachments)
            {
                string captured = path;
                var chip = PaneTheme.MakeLink("📄 " + System.IO.Path.GetFileName(path) + "  ✕", PaneTheme.Caption);
                chip.AccessibleName = Ui.T("Bỏ tệp đính kèm ") + System.IO.Path.GetFileName(path);
                chip.Margin = new Padding(0, PaneTheme.Px(4), PaneTheme.Px(10), 0);
                new ToolTip().SetToolTip(chip, path);
                chip.LinkClicked += delegate
                {
                    _attachments.Remove(captured);
                    RefreshAttachments();
                };
                _attachStrip.Controls.Add(chip);
            }
            _attachStrip.Visible = _attachments.Count > 0;
            _attachStrip.ResumeLayout(true);
            _attachLink.Text = _attachments.Count > 0 ? Ui.T("+ Đính kèm (") + _attachments.Count + ")" : Ui.T("+ Đính kèm");
        }

        // Câu gửi Core: lời người dùng + đường dẫn đầy đủ. Model trước đây gọi ppt_profile bằng tên tệp trần và
        // nhận "file not found" (13/10/2026) - nên nói rõ dùng NGUYÊN đường dẫn.
        private static string WithAttachments(string promptText, List<string> files)
        {
            if (files == null || files.Count == 0)
            {
                return promptText;
            }
            var lines = new List<string> { promptText, "", "[Attached files - read them with the office file tools " +
                "(doc_get_text, excel_profile, excel_read, ppt_get_text; ppt_profile gives a template's size, colors, " +
                "fonts and slide kinds for wpp.newFromTemplate) using the full path exactly as written:]" };
            foreach (string path in files)
            {
                lines.Add("- " + path);
            }
            return string.Join("\n", lines.ToArray());
        }

        private void SetPlanMode(bool on)
        {
            _planMode = on;
            _planToggle.Text = (on ? "● " : "○ ") + Ui.T("Lập kế hoạch trước");
            _planToggle.LinkColor = on ? PaneTheme.AccentFg : PaneTheme.TextMuted;
            _planToggle.AccessibleName = Ui.T("Lập kế hoạch trước khi sửa: ") + (on ? Ui.T("đang bật") : Ui.T("đang tắt"));
            if (_blockedReason == null)
            {
                _placeholder.Text = on ? PlanPlaceholder : PromptPlaceholder;
            }
            Logger.Info("AskAiPane: plan mode " + (on ? "on" : "off"));
        }

        private void StartRun(string promptText, bool plan)
        {
            StartRun(promptText, plan, new List<string>());
        }

        private void StartRun(string promptText, bool plan, List<string> files)
        {
            Connect host = CurrentHost;
            if (host == null)
            {
                RefreshState();
                return;
            }
            if (_empty != null)
            {
                _chat.RemoveItem(_empty);
                _empty = null;
            }

            var run = new CancellationTokenSource();
            _run = run;
            _busy = true;
            _toolCount = 0;
            _runEdits = 0;
            _reasoningBlock = null;   // mỗi lượt một khối suy luận riêng
            _undoLink.Visible = false;
            _lastPrompt = promptText;
            _lastPlan = plan;
            _lastAttachments = files;
            string corePrompt = WithAttachments(promptText, files);
            if (_planCard != null && _planCard.IsOpen)
            {
                _planCard.Close(promptText == ExecutePlanPrompt && !plan ? Ui.T("Đang thực hiện kế hoạch…") : Ui.T("Đã gửi yêu cầu mới"));
            }
            _insertLink.Visible = false;
            _stopLink.Visible = true;
            _busyDot.Visible = true;
            UpdateSendEnabled();
            _chat.AddMessage(files.Count == 0 ? promptText
                : promptText + "\n📎 " + string.Join(", ", files.Select(f => System.IO.Path.GetFileName(f)).ToArray()), true);
            if (plan)
            {
                _chat.AddInfo(Ui.T("Lập kế hoạch: AI chỉ đọc tài liệu, chưa sửa gì"));
            }
            // Đang bôi đen trong Word: AI chỉ được sửa vùng đó tới hết lượt (CommandDispatcher.WriterScope.cs).
            int locked = 0;
            if (host.AppKind == "wps")
            {
                try
                {
                    locked = CommandDispatcher.LockSelection(host);
                }
                catch (Exception ex)
                {
                    Logger.Error("AskAiPane: lock selection failed", ex);
                }
            }
            if (locked > 0)
            {
                _chat.AddInfo(Ui.T("Chỉ sửa trong vùng đang bôi đen (") + locked + Ui.T(" ký tự). Muốn AI sửa cả tài liệu thì bỏ bôi đen rồi hỏi lại."));
            }
            _chat.AddTyping();
            SetStatus(Ui.T("Đang xử lý…"), false);
            Logger.Info("AskAiPane: prompt=" + promptText + (plan ? " (plan)" : "") + (files.Count > 0 ? " files=" + files.Count : ""));

            Action<string> progress = delegate(string line)
            {
                Logger.Info("AskAiPane progress: " + line);
                PostToUi(delegate { OnProgress(run, line); });
            };

            ThreadPool.QueueUserWorkItem(delegate
            {
                LlmResult result;
                try
                {
                    // Moi luot chay qua Agent Core (docs/architecture/core.md muc 9); Core khong dung duoc thi bao loi ro.
                    // Khong co agent thu hai trong add-in: moi luot phai qua policy/xac nhan/audit/memory cua Core.
                    result = RunViaCore(host, corePrompt, run, progress, plan);
                    if (result == null)
                    {
                        result = new LlmResult();
                        result.Error = Ui.T("Agent Core không chạy được (")
                            + (CoreClient.Instance.LastError ?? Ui.T("không rõ lý do")) + ").";
                    }
                }
                catch (Exception ex)
                {
                    result = new LlmResult();
                    result.Error = ex.GetType().Name + ": " + ex.Message;
                    Logger.Error("AskAiPane agent crashed", ex);
                }
                // Ghi log kết quả ngay trên worker: vẫn có dấu vết kể cả khi không trả được về UI.
                if (result.Ok)
                {
                    Logger.Info("AskAiPane: ok in " + result.Seconds.ToString("0.0") + "s, " + result.Transcript.Count + " tool calls, " + result.Rounds + " rounds");
                }
                else if (result.Cancelled)
                {
                    Logger.Info("AskAiPane: cancelled after " + result.Seconds.ToString("0.0") + "s, " + result.Transcript.Count + " tool calls");
                }
                else
                {
                    Logger.Error("AskAiPane failed after " + result.Seconds.ToString("0.0") + "s: " + result.Error, null);
                }
                if (!PostToUi(delegate { FinishRun(run, host, result, plan); }))
                {
                    Logger.Error("AskAiPane: result not delivered to UI (pane closed or handle gone)", null);
                }
            });
        }

        // Chay mot luot qua Agent Core. Tra null khi Core khong dung duoc (pane bao loi).
        private LlmResult RunViaCore(Connect host, string promptText, CancellationTokenSource run, Action<string> progress, bool plan)
        {
            CoreClient core = CoreClient.Instance;
            if (!core.Enabled)
            {
                return null;
            }

            string documentKey = DocumentKey(host);
            string conversationId = _conversationId;
            if (conversationId == null && !_freshConversation)
            {
                // Mo lai tai lieu sau: tiep tuc hoi thoai gan nhat cua tai lieu do (muc 9).
                conversationId = core.FindConversationForDocument(documentKey);
            }
            _freshConversation = false;

            LlmResult result = core.Run(host, promptText, conversationId, plan ? "plan" : null, delegate(CoreEvent item)
            {
                if (item.Type == "confirm.required" && !string.IsNullOrEmpty(item.ConfirmationId))
                {
                    CoreEvent request = item;
                    PostToUi(delegate { ShowConfirmCard(request); });
                    return;
                }
                if (item.Type == "confirm.resolved" && !string.IsNullOrEmpty(item.ConfirmationId))
                {
                    CoreEvent resolved = item;
                    PostToUi(delegate { ResolveConfirmCard(resolved); });
                    return;
                }
                if (item.Type == "memory.written" && !string.IsNullOrEmpty(item.Id))
                {
                    string memoryId = item.Id;
                    string memoryText = item.Text ?? "";
                    PostToUi(delegate { ShowMemoryNote(memoryId, memoryText); });
                    return;
                }
                if (item.Type == "run.reasoning.live" && !string.IsNullOrEmpty(item.Text))
                {
                    string live = item.Text;
                    int liveRound = item.Round;
                    PostToUi(delegate { ShowReasoning(live, liveRound, true); });
                    return;
                }
                // Dòng trạng thái khi model đang viết dài ("Đang thiết kế phần 1–15: 7 slide…") - trước đây đứng yên 1-2 phút.
                if (item.Type == "run.progress" && !string.IsNullOrEmpty(item.Text))
                {
                    string status = item.Text;
                    PostToUi(delegate
                    {
                        if (_busy)
                        {
                            SetStatus(status, false);
                        }
                    });
                    return;
                }
                if (item.Type == "run.reasoning" && !string.IsNullOrEmpty(item.Text))
                {
                    string reasoning = item.Text;
                    int round = item.Round;
                    PostToUi(delegate { ShowReasoning(reasoning, round); });
                    return;
                }
                string line = ProgressLine(item);
                if (line != null)
                {
                    progress(line);
                }
            }, run.Token);

            if (result != null && !string.IsNullOrEmpty(result.ConversationId))
            {
                // Moi luot chay mot lan mot luc (_busy) nen gan truc tiep la an toan.
                _conversationId = result.ConversationId;
            }

            return result;
        }

        // Doi su kien Core thanh dong tien trinh dung dinh dang ToolLine da co cua pane.
        private static string ProgressLine(CoreEvent item)
        {
            if (item == null)
            {
                return null;
            }

            // Danh sách việc (write_todos) và đổi sang model dự phòng: dòng thông tin, không phải dòng công cụ.
            if ((item.Type == "run.todos" || item.Type == "run.model_switched") && !string.IsNullOrEmpty(item.Text))
            {
                return "(" + item.Text + ")";
            }

            if (item.Type == "skill.loaded")
            {
                // Dòng thông tin (không có " -> ") - docs/architecture/core.md mục 7.4.
                return Ui.T("(Dùng kỹ năng: ") + (item.Name ?? "?") + ")";
            }

            // load_skill / read_skill_file: đã có dòng "Dùng kỹ năng"; remember: đã có dòng "Đã ghi nhớ".
            // Công cụ khác (look_at_document, recall, MCP) PHẢI hiện: trước đây bị ẩn hết nên một lượt gọi MCP
            // lỗi rồi chờ model trông như pane đứng ở bước office_action cuối (đo 13/10/2026).
            if (item.Type == "tool.finished" && (item.Tool == "load_skill" || item.Tool == "read_skill_file" || item.Tool == "remember"
                || item.Tool == "write_todos"))
            {
                return null;
            }

            if (item.Type == "tool.finished")
            {
                return CoreClient.ToolLine(item);
            }

            if (item.Type == "run.failed")
            {
                return "(" + (item.Error ?? "loi") + ")";
            }

            return null;
        }

        // Duong dan tai lieu chuan hoa (giong Orchestrator.DocumentKey ben Core) de tra hoi thoai cu.
        // Doc COM tu worker thread nen phai qua ComGate nhu moi noi khac trong add-in.
        internal static string DocumentKey(Connect host)
        {
            string key = null;
            try
            {
                ComGate.TryRun(2000, delegate
                {
                    try
                    {
                        System.Collections.Generic.Dictionary<string, object> info = HostProbe.ActiveDocument(host);
                        if (info == null)
                        {
                            return;
                        }

                        object value;
                        if (!info.TryGetValue("fullName", out value))
                        {
                            return;
                        }

                        string text = Convert.ToString(value);
                        if (!string.IsNullOrEmpty(text))
                        {
                            string normalized = text.Trim().Replace('/', '\\').ToLowerInvariant();
                            // Tai lieu CHUA LUU ("Document1", "Presentation1") khong phai duong dan:
                            // moi file moi trung ten nen tra hoi thoai theo ten se dinh hoi thoai cu
                            // (loi that 16/10/2026: "Document1" keo hoi thoai "dieu le giai bong ban"
                            // tu 30/09 vao luot hoi ve chan doan hinh anh y te).
                            if (normalized.IndexOf('\\') >= 0)
                            {
                                key = normalized;
                            }
                        }
                    }
                    catch (Exception ex)
                    {
                        Logger.Info("AskAiPane: doc ten tai lieu that bai: " + ex.Message);
                    }
                });
            }
            catch (Exception ex)
            {
                Logger.Info("AskAiPane: ComGate loi khi doc ten tai lieu: " + ex.Message);
            }

            return key;
        }

        private void StartNewConversation()
        {
            if (_busy)
            {
                return;
            }

            _conversationId = null;
            _freshConversation = true;
            // Xoá khung chat cũ (người dùng 13/10/2026: bấm "Trò chuyện mới" mà hội thoại cũ vẫn nằm đó) và mọi tham
            // chiếu tới thẻ của hội thoại cũ.
            _chat.ClearAll();
            _empty = null;
            _planCard = null;
            _reasoningBlock = null;
            _confirmCards.Clear();
            _lastReply = "";
            _lastPrompt = "";
            _lastPlan = false;
            _undoLink.Visible = false;
            AddGreeting();
            _chat.AddInfo(Ui.T("Bắt đầu cuộc trò chuyện mới cho tài liệu này."));
            Logger.Info("AskAiPane: new conversation requested");
            _prompt.Focus();
        }

        private readonly Dictionary<string, ConfirmCard> _confirmCards = new Dictionary<string, ConfirmCard>(StringComparer.Ordinal);

        private void ShowConfirmCard(CoreEvent request)
        {
            var card = new ConfirmCard(request.ConfirmationId, request.Action, request.Reason, request.ParamsPreview, request.Choices);
            string runId = request.RunId;
            card.Chosen += delegate(object sender, string[] picked)
            {
                ThreadPool.QueueUserWorkItem(delegate
                {
                    string error;
                    if (!CoreClient.Instance.ChooseRun(runId, card.ConfirmationId, picked[0], picked[1], out error))
                    {
                        Logger.Info("AskAiPane: choose failed: " + error);
                    }
                });
            };
            card.Answered += delegate(object sender, bool approved)
            {
                ThreadPool.QueueUserWorkItem(delegate
                {
                    string error;
                    if (!CoreClient.Instance.ConfirmRun(runId, card.ConfirmationId, approved, out error))
                    {
                        Logger.Info("AskAiPane: confirm failed: " + error);
                    }
                });
            };
            _confirmCards[request.ConfirmationId] = card;
            _chat.AddBlock(card);
            bool choosing = request.Choices != null && request.Choices.Count > 0;
            SetStatus(choosing ? Ui.T("Đang chờ bạn chọn…") : Ui.T("Đang chờ bạn xác nhận…"), false);
            Announce((choosing ? Ui.T("Cần bạn chọn: ") : Ui.T("Cần bạn xác nhận: ")) + (request.Reason ?? ""));
        }

        private void ResolveConfirmCard(CoreEvent resolved)
        {
            ConfirmCard card;
            if (_confirmCards.TryGetValue(resolved.ConfirmationId, out card))
            {
                card.SetResolved(resolved.Approved, resolved.By, resolved.Choice, resolved.Text);
                _confirmCards.Remove(resolved.ConfirmationId);
            }
        }

        // Khối suy luận của lượt đang chạy (chỉ có khi người dùng bật LlmShowReasoning). Một khối cho cả
        // lượt: mỗi vòng ghi đè bằng phần mới nhất để pane không bị lấp bởi văn bản suy luận.
        private ReasoningBlock _reasoningBlock;

        private void ShowReasoning(string text, int round)
        {
            ShowReasoning(text, round, false);
        }

        private void ShowReasoning(string text, int round, bool live)
        {
            if (_reasoningBlock == null)
            {
                _reasoningBlock = new ReasoningBlock(live ? "" : text, round);
                _chat.AddBlock(_reasoningBlock);
            }
            if (live)
            {
                _reasoningBlock.UpdateLive(text, round);
            }
            else
            {
                _reasoningBlock.Update(text, round);
            }
        }

        // Memory đã hiện trong pane (không hiện lại khi hỏi lại kết quả trích xuất nền).
        private readonly HashSet<string> _shownMemories = new HashSet<string>(StringComparer.Ordinal);

        private void ShowMemoryNote(string memoryId, string text)
        {
            if (string.IsNullOrEmpty(memoryId) || !_shownMemories.Add(memoryId))
            {
                return;
            }
            var note = new MemoryNote(memoryId, text);
            note.DeleteClicked += delegate
            {
                note.SetDeleting();
                ThreadPool.QueueUserWorkItem(delegate
                {
                    string error;
                    bool ok = CoreClient.Instance.DeleteMemory(note.MemoryId, out error);
                    if (!ok)
                    {
                        Logger.Info("AskAiPane: delete memory failed: " + error);
                    }
                    PostToUi(delegate { note.SetDeleted(ok); });
                });
            };
            _chat.AddBlock(note);
        }

        // Trích xuất memory chạy nền sau lượt (docs/architecture/core.md 8.5.4): hỏi lại sau ít giây để hiện "Đã ghi nhớ".
        private void PollExtractedMemories(string runId)
        {
            if (string.IsNullOrEmpty(runId))
            {
                return;
            }
            ThreadPool.QueueUserWorkItem(delegate
            {
                foreach (int delayMs in new[] { 8000, 17000 })
                {
                    Thread.Sleep(delayMs);
                    string error;
                    List<Dictionary<string, object>> items = CoreClient.Instance.ListMemories(null, runId, false, out error);
                    if (items == null)
                    {
                        return;
                    }
                    foreach (Dictionary<string, object> item in items)
                    {
                        string id = item.ContainsKey("id") ? Convert.ToString(item["id"]) : null;
                        string text = item.ContainsKey("text") ? Convert.ToString(item["text"]) : "";
                        if (!PostToUi(delegate { ShowMemoryNote(id, text); }))
                        {
                            return;
                        }
                    }
                }
            });
        }

        private bool PostToUi(Action action)
        {
            try
            {
                if (IsDisposed || !IsHandleCreated)
                {
                    return false;
                }
                BeginInvoke((MethodInvoker)delegate
                {
                    try
                    {
                        action();
                    }
                    catch (Exception ex)
                    {
                        Logger.Error("AskAiPane: UI update failed", ex);
                    }
                });
                return true;
            }
            catch (Exception ex)
            {
                Logger.Error("AskAiPane: BeginInvoke failed", ex);
                return false;
            }
        }

        private void OnProgress(CancellationTokenSource run, string line)
        {
            if (run != _run)
            {
                return;
            }
            ToolLine item = _chat.AddProgress(line);
            if (item.IsToolCall)
            {
                _toolCount++;
                SetStatus(Ui.T("Đang xử lý… · bước ") + (_toolCount + 1), false);
            }
            if (item.Succeeded && IsWordEdit(item.ActionId))
            {
                _runEdits++;
            }
        }

        // --- BEGIN SINH TU DONG (scripts/generate_mcp_commands.py) - KHONG SUA TAY ---
        // Lenh writer.* co `undo` trong catalog = dung mot buoc Undo (UndoRecordScope).
        private static readonly HashSet<string> WordEditActions = new HashSet<string>(StringComparer.Ordinal)
        {
            "writer.addTableRows",
            "writer.appendText",
            "writer.deleteParagraphs",
            "writer.formatSelection",
            "writer.formatTable",
            "writer.heading",
            "writer.insertBlocks",
            "writer.insertCaption",
            "writer.insertHyperlink",
            "writer.insertImage",
            "writer.insertLayoutTable",
            "writer.insertPageBreak",
            "writer.insertStyledText",
            "writer.insertTable",
            "writer.insertToc",
            "writer.replaceAll",
            "writer.replaceMany",
            "writer.restyle",
            "writer.setHeaderFooter",
            "writer.setParagraphAlignment",
            "writer.setStyle",
            "writer.typeText",
            "writer.writeContent",
        };
        // --- END SINH TU DONG ---

        // Lệnh writer.* có sửa tài liệu = lệnh có `undo` trong registry (WordEditActions sinh tự động ở trên):
        // mỗi lệnh = 1 bước Undo nhờ UndoRecordScope; lệnh đọc/mở/soát/lưu không tính.
        internal static bool IsWordEdit(string action)
        {
            return !string.IsNullOrEmpty(action) && WordEditActions.Contains(action);
        }

        // "Hoàn tác lượt này" (docs/architecture/core.md mục 9.2, chỉ Word): writer.undo {count: số thao tác sửa của lượt}.
        // Excel/PowerPoint không hoàn tác được thay đổi qua COM nên không có nút.
        private void UndoLastRun()
        {
            Connect host = CurrentHost;
            if (host == null || _busy || _lastRunEdits <= 0)
            {
                return;
            }
            try
            {
                var serializer = new System.Web.Script.Serialization.JavaScriptSerializer();
                var arguments = new Dictionary<string, object>
                {
                    { "action", "writer.undo" },
                    { "params", new Dictionary<string, object> { { "count", _lastRunEdits } } }
                };
                string reply = OfficeActionTool.Execute(host, OfficeActionTool.ToolName, serializer.Serialize(arguments));
                Logger.Info("AskAiPane: undo last run (" + _lastRunEdits + " edits) -> " + Truncate(reply, 200));
                bool ok = reply.IndexOf("\"ok\":true", StringComparison.Ordinal) >= 0;
                _chat.AddInfo(ok ? Ui.T("Đã hoàn tác ") + _lastRunEdits + Ui.T(" thao tác của lượt vừa rồi.") : Ui.T("Không hoàn tác được: ") + Truncate(reply, 120));
                _undoLink.Visible = !ok;
                if (ok)
                {
                    _lastRunEdits = 0;
                }
            }
            catch (Exception ex)
            {
                SetStatus(Ui.T("Hoàn tác thất bại: ") + Truncate(ex.Message, 100), true);
            }
        }

        private void FinishRun(CancellationTokenSource run, Connect host, LlmResult result, bool plan)
        {
            if (run != _run)
            {
                return;
            }
            _run = null;
            run.Dispose();
            CommandDispatcher.UnlockSelection();
            _busy = false;
            _stopLink.Visible = false;
            _busyDot.Visible = false;
            _chat.RemoveTyping();
            _lastRunViaCore = result.ViaCore;
            // Core biet chinh xac so buoc hoan tac con lai cua luot (da tru lan agent tu writer.undo); tu dem tool.finished
            // thi khong tru duoc va nut nay lui vao noi dung co tu truoc luot (audit agent runtime F02).
            _lastRunEdits = result.UndoSteps >= 0 ? result.UndoSteps : _runEdits;
            _undoLink.Visible = host.AppKind == "wps" && _lastRunEdits > 0;
            // Excel/PowerPoint cua Microsoft: thay doi qua COM khong vao ngan hoan tac (khong co nut "Hoan tac luot nay") - noi
            // ro pham vi thay vi de nguoi dung tuong Ctrl+Z se lay lai duoc (Claude Code /rewind bao ro cai gi khong khoi phuc).
            if (host.IsOfficeHost && (host.AppKind == "et" || host.AppKind == "wpp") && result.EditSteps > 0)
            {
                _chat.AddInfo(Ui.T("Lưu ý: Excel/PowerPoint không hoàn tác được bằng Ctrl+Z các thay đổi AI vừa làm. Nếu cần quay lại, hãy đóng tài liệu không lưu hoặc mở bản đã lưu trước đó."));
            }
            if (result.ViaCore && result.Ok)
            {
                PollExtractedMemories(result.RunId);
            }
            _newChatLink.Visible = CoreClient.Instance.Enabled;
            string seconds = result.Seconds.ToString("0.0");

            if (result.Ok)
            {
                if (!string.IsNullOrEmpty(result.Text))
                {
                    _chat.AddMessage(result.Text, false);
                    _lastReply = result.Text;
                    _insertLink.Visible = host.AppKind == "wps" && !plan;
                }
                if (plan)
                {
                    _planCard = new PlanCard();
                    PlanCard card = _planCard;
                    card.RunClicked += delegate
                    {
                        if (_busy)
                        {
                            return;
                        }
                        RefreshState();
                        if (_blockedReason == null)
                        {
                            StartRun(ExecutePlanPrompt, false);
                        }
                    };
                    _chat.AddBlock(card);
                    SetStatus(Ui.T("Đã lập kế hoạch trong ") + seconds + Ui.T("s · chưa sửa gì"), false);
                    Announce(Ui.T("Đã lập kế hoạch. ") + (result.Text ?? ""));
                    UpdateSendEnabled();
                    return;
                }
                SetStatus("Xong trong " + seconds + "s · " + _toolCount + Ui.T(" thao tác") + BackendNote(), false);
                Announce("Xong. " + (result.Text ?? ""));
            }
            else if (result.Cancelled)
            {
                _chat.AddInfo(Ui.T("Đã dừng theo yêu cầu. Các thao tác đã làm vẫn còn trong tài liệu."));
                SetStatus(Ui.T("Đã dừng · ") + _toolCount + Ui.T(" thao tác"), false);
                Announce(Ui.T("Đã dừng"));
            }
            else
            {
                var card = new ErrorCard(Ui.T("Không hoàn thành yêu cầu"), DescribeError(result));
                card.RetryClicked += delegate
                {
                    if (!_busy && _lastPrompt.Length > 0)
                    {
                        RefreshState();
                        if (_blockedReason == null)
                        {
                            StartRun(_lastPrompt, _lastPlan, _lastAttachments);
                        }
                    }
                };
                card.SettingsClicked += delegate { OpenSetup(); };
                _chat.AddBlock(card);
                SetStatus(Ui.T("Không hoàn thành · ") + seconds + "s" + BackendNote(), true);
                Announce(Ui.T("Lỗi: không hoàn thành yêu cầu"));
            }
            UpdateSendEnabled();
        }

        // Khi Core khong chay duoc thi noi ro de nguoi dung biet dang o che do du phong (muc 9).
        private string BackendNote()
        {
            if (_lastRunViaCore || !CoreClient.Instance.Enabled)
            {
                return "";
            }

            return Ui.T(" · chế độ cơ bản");
        }

        private string DescribeError(LlmResult result)
        {
            string error = result.Error ?? "";
            string lower = error.ToLowerInvariant();
            string message;
            if (result.TimedOut)
            {
                message = Ui.T("AI không phản hồi kịp nên lượt chạy đã dừng.");
            }
            else if (result.Stopped)
            {
                message = Ui.T("AI đã dùng hết ngân sách token cho lượt này nên dừng lại.");
            }
            else if (lower.Contains("timed out"))
            {
                message = Ui.T("Máy chủ AI không phản hồi sau 60 giây.");
            }
            else if (lower.Contains("(401)") || lower.Contains("unauthorized"))
            {
                message = Ui.T("Máy chủ AI từ chối khoá API. Kiểm tra lại khoá trong Cài đặt.");
            }
            else if (lower.Contains("unable to connect") || lower.Contains("actively refused") || lower.Contains("remote name could not be resolved"))
            {
                message = Ui.T("Không kết nối được máy chủ AI (") + (Config.LlmEndpoint ?? "") + ").";
            }
            else if (lower.Contains("repeated the same failing call"))
            {
                // Core chu dong dung (repeatedFailureGuard), khong phai may chu AI loi - luot that 25/10/2026 nguoi dung
                // doc "May chu AI bao loi" va tuong AI hong.
                message = Ui.T("AI gọi lại cùng một lệnh bị lỗi nhiều lần nên Axiom đã dừng lượt này (máy chủ AI không lỗi). Chi tiết: ")
                    + Truncate(error, 300);
            }
            else if (!string.IsNullOrEmpty(result.ErrorKind) && result.ErrorKind != "provider" && result.ErrorKind != "config")
            {
                message = Ui.T("Axiom đã dừng lượt chạy: ") + Truncate(error, 300);
            }
            else
            {
                message = Ui.T("Máy chủ AI báo lỗi: ") + Truncate(error, 300);
            }
            if (_toolCount > 0)
            {
                message += Ui.T(" Đã làm xong ") + _toolCount + Ui.T(" thao tác, phần đã ghi vẫn còn trong tài liệu.");
            }
            return message;
        }

        private void OnStop()
        {
            CancellationTokenSource run = _run;
            if (run == null)
            {
                return;
            }
            Logger.Info("AskAiPane: stop requested");
            _stopLink.Visible = false;
            SetStatus(Ui.T("Đang dừng…"), false);
            run.Cancel();
        }

        // Menu "⋯" ở header: xem lại hội thoại cũ và các lệnh AI đã gọi vào tài liệu.
        private void ShowMoreMenu()
        {
            var menu = new ContextMenuStrip { Font = PaneTheme.Body };
            menu.Items.Add(Ui.T("Hội thoại trước…"), null, delegate
            {
                Connect host = CurrentHost;
                using (var form = new ConversationsForm(host != null ? DocumentKey(host) : null))
                {
                    form.ShowDialog(this);
                }
            });
            menu.Items.Add(Ui.T("Nhật ký hoạt động…"), null, delegate
            {
                using (var form = new AuditForm())
                {
                    form.ShowDialog(this);
                }
            });
            menu.Closed += delegate { BeginInvoke((Action)menu.Dispose); };
            menu.Show(_moreLink, new Point(0, _moreLink.Height));
        }

        // Wizard thiet lap (cung noi dung voi ban Linux): nguoi dung khong chuyen cung lam duoc.
        // Cau hinh nang cao (SettingsForm) van vao duoc tu buoc 4 cua wizard.
        private void OpenSetup()
        {
            using (var form = new SetupWizardForm(CurrentHost))
            {
                form.ShowDialog(this);
            }
            RefreshState();
        }

        // Chua co cau hinh AI -> tu mo wizard MOT lan trong moi phien (giong panel.py cua ban Linux),
        // khong lam phien nguoi dung da biet minh muon tu cau hinh.
        private static bool _setupOffered;

        private void OfferSetupOnce()
        {
            if (_setupOffered)
            {
                return;
            }

            if ((Config.LlmEndpoint ?? "").Trim().Length > 0 && (Config.LlmModel ?? "").Trim().Length > 0)
            {
                return;
            }

            _setupOffered = true;
            var pane = this;
            var timer = new System.Windows.Forms.Timer { Interval = 1500 };
            timer.Tick += delegate
            {
                timer.Stop();
                timer.Dispose();
                try
                {
                    pane.OpenSetup();
                }
                catch (Exception ex)
                {
                    Logger.Error("AskAiPane: khong mo duoc wizard thiet lap", ex);
                }
            };
            timer.Start();
        }

        private void SetStatus(string text, bool isError)
        {
            _status.ForeColor = isError ? PaneTheme.DangerFg : PaneTheme.TextMuted;
            _status.Text = text;
        }

        // Báo cho trình đọc màn hình (Narrator/NVDA) mà không chuyển focus.
        private void Announce(string text)
        {
            try
            {
                _status.AccessibilityObject.RaiseAutomationNotification(
                    System.Windows.Forms.Automation.AutomationNotificationKind.ActionCompleted,
                    System.Windows.Forms.Automation.AutomationNotificationProcessing.ImportantMostRecent,
                    Truncate(text, 300));
            }
            catch (Exception)
            {
            }
        }

        // WPS có thể bóp hẹp CTP bất kể Width đặt vào; kiểm tra vài lần sau khi host layout xong.
        private void ScheduleWidthCheck()
        {
            if (CurrentHost == null)
            {
                return;
            }
            var timer = new System.Windows.Forms.Timer { Interval = 500 };
            timer.Tick += delegate
            {
                _widthChecks++;
                bool done = CurrentHost == null
                    || CurrentHost.AdjustTaskPaneWidth(Width, PaneTheme.Px(MinPaneWidth), PaneTheme.Px(TargetPaneWidth));
                if (done || _widthChecks >= 3)
                {
                    if (!done)
                    {
                        Logger.Info("Task pane stays narrow after " + _widthChecks + " attempts (" + Width + "px); host may ignore CTP Width");
                    }
                    timer.Stop();
                    timer.Dispose();
                }
            };
            timer.Start();
        }

        private void OnInsert(object sender, LinkLabelLinkClickedEventArgs e)
        {
            Connect host = CurrentHost;
            if (host == null || _lastReply.Length == 0)
            {
                return;
            }
            try
            {
                var serializer = new System.Web.Script.Serialization.JavaScriptSerializer();
                var arguments = new System.Collections.Generic.Dictionary<string, object>
                {
                    { "action", "writer.insertStyledText" },
                    { "params", new System.Collections.Generic.Dictionary<string, object> { { "text", _lastReply } } }
                };
                OfficeActionTool.Execute(host, OfficeActionTool.ToolName, serializer.Serialize(arguments));
                SetStatus(Ui.T("Đã chèn vào tài liệu"), false);
            }
            catch (Exception ex)
            {
                SetStatus(Ui.T("Chèn thất bại: ") + Truncate(ex.Message, 100), true);
            }
        }

        private static string Truncate(string text, int max)
        {
            if (string.IsNullOrEmpty(text) || text.Length <= max)
            {
                return text ?? "";
            }
            return text.Substring(0, max) + "…";
        }
    }

    internal sealed class AskAiHostForm : Form
    {
        public AskAiHostForm(Connect connect)
        {
            Text = "Axiom Office - Ask AI";
            ClientSize = new Size(PaneTheme.Px(400), PaneTheme.Px(600));
            StartPosition = FormStartPosition.CenterScreen;
            Font = PaneTheme.Body;
            BackColor = PaneTheme.PaneBg;
            Controls.Add(new AskAiPane(connect));
        }
    }
}

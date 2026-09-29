using System;
using System.Collections.Generic;
using System.Drawing;
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
        private const string PromptPlaceholder = "Nhập yêu cầu cho tài liệu này…";
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
        private readonly Label _status;
        private readonly AccentDot _busyDot;
        private readonly LinkLabel _insertLink;
        private readonly LinkLabel _stopLink;
        private readonly LinkLabel _newChatLink;
        private EmptyState _empty;
        private string _conversationId;              // hoi thoai ben Agent Core, theo tai lieu dang mo
        private bool _freshConversation;             // nguoi dung vua bam "Cuoc tro chuyen moi"
        private bool _lastRunViaCore;                // luot vua roi chay qua Core hay in-process
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
                AccessibleName = "Yêu cầu cho AI",
            };
            _placeholder = PaneTheme.MakeLabel(PromptPlaceholder, PaneTheme.Body, PaneTheme.TextMuted);
            _placeholder.Cursor = Cursors.IBeam;
            _placeholder.AccessibleRole = AccessibleRole.StaticText;
            _hint = PaneTheme.MakeLabel("Enter để gửi · Shift+Enter xuống dòng", PaneTheme.Caption, PaneTheme.TextMuted);
            _hint.AutoEllipsis = true;
            _hint.TextAlign = ContentAlignment.MiddleLeft;
            _send = new SendButton();
            _composer.Controls.Add(_placeholder);
            _composer.Controls.Add(_prompt);
            _composer.Controls.Add(_hint);
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
            _status.AccessibleName = "Trạng thái";
            _busyDot = new AccentDot(PaneTheme.Px(6)) { Dock = DockStyle.Left, Width = PaneTheme.Px(12), Visible = false };
            _insertLink = MakeFooterLink("Chèn trả lời");
            _stopLink = MakeFooterLink("Dừng");
            _newChatLink = MakeFooterLink("Cuộc trò chuyện mới");
            footer.Controls.Add(_status);
            footer.Controls.Add(_busyDot);
            footer.Controls.Add(_insertLink);
            footer.Controls.Add(_stopLink);
            footer.Controls.Add(_newChatLink);

            var header = new DividerPanel { Dock = DockStyle.Top, Height = PaneTheme.Px(PaneTheme.HeaderH) };
            var dot = new AccentDot(PaneTheme.Px(8));
            dot.Location = new Point(PaneTheme.Px(PaneTheme.PadX), (header.Height - dot.Height) / 2);
            var title = PaneTheme.MakeLabel("Axiom Office", PaneTheme.Title, PaneTheme.TextPrimary);
            title.AutoSize = true;
            title.Location = new Point(PaneTheme.Px(28), PaneTheme.Px(7));
            _headerSubtitle = PaneTheme.MakeLabel("", PaneTheme.Caption, PaneTheme.TextMuted);
            _headerSubtitle.AutoEllipsis = true;
            _settingsLink = PaneTheme.MakeLink("Cài đặt", PaneTheme.Caption);
            header.Controls.Add(dot);
            header.Controls.Add(title);
            header.Controls.Add(_headerSubtitle);
            header.Controls.Add(_settingsLink);
            header.Resize += delegate { LayoutHeader(header); };

            Controls.Add(_chat);
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
            _settingsLink.LinkClicked += delegate { OpenSettings(); };
            _composer.Resize += delegate { LayoutComposer(); };
            Load += delegate
            {
                LayoutHeader(header);
                LayoutComposer();
                RefreshState();
                _prompt.Focus();
                ScheduleWidthCheck();
            };

            AddGreeting();
        }

        protected override void Dispose(bool disposing)
        {
            if (disposing && _run != null)
            {
                _run.Cancel();
            }
            base.Dispose(disposing);
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
            int left = PaneTheme.Px(28);
            _headerSubtitle.SetBounds(left, PaneTheme.Px(26), Math.Max(PaneTheme.Px(40), _settingsLink.Left - PaneTheme.Px(8) - left), PaneTheme.Px(16));
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
                    OpenSettings();
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
            _headerSubtitle.Text = HostLabel() + " · " + (model.Length > 0 ? model : "Chưa chọn model");

            string reason = null;
            if (CurrentHost == null)
            {
                reason = "Mất kết nối với ứng dụng. Mở lại tài liệu để tiếp tục.";
            }
            else if (endpoint.Length == 0 || model.Length == 0)
            {
                reason = "Chưa cấu hình AI. Mở Cài đặt để nhập endpoint và model.";
            }
            _blockedReason = reason;
            bool blocked = reason != null;
            _composer.Blocked = blocked;
            _prompt.Enabled = !blocked;
            Color surface = blocked ? PaneTheme.InputDisabled : PaneTheme.PaneBg;
            _prompt.BackColor = surface;
            _placeholder.BackColor = surface;
            _placeholder.Text = blocked ? reason : PromptPlaceholder;
            _placeholder.Cursor = blocked ? Cursors.Hand : Cursors.IBeam;
            _hint.Visible = !blocked;
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
            return kind == "et" ? "Bảng tính" : kind == "wpp" ? "Trình chiếu" : "Văn bản";
        }

        private void AddGreeting()
        {
            string kind = HostKind();
            string noun = kind == "et" ? "bảng tính" : kind == "wpp" ? "bản trình chiếu" : "văn bản";
            _empty = new EmptyState(
                "Bạn muốn làm gì với " + noun + " này?",
                "Tôi đọc và chỉnh sửa trực tiếp tài liệu đang mở. Thay đổi hiện ngay trên trang khi tôi làm.",
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
                return new[] { "Tạo bảng điểm 5 học sinh, có cột trung bình", "Thêm cột Tổng có công thức cho bảng này", "Tô đậm và kẻ khung dòng tiêu đề" };
            }
            if (kind == "wpp")
            {
                return new[] { "Tạo 5 slide giới thiệu công ty", "Thêm slide kế hoạch quý 4", "Viết ghi chú thuyết trình cho từng slide" };
            }
            return new[] { "Soạn đơn xin việc vị trí kế toán", "Tóm tắt tài liệu này thành 5 ý", "Chèn bảng lịch họp tuần 3 cột" };
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
            StartRun(promptText);
        }

        private void StartRun(string promptText)
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
            _lastPrompt = promptText;
            _insertLink.Visible = false;
            _stopLink.Visible = true;
            _busyDot.Visible = true;
            UpdateSendEnabled();
            _chat.AddMessage(promptText, true);
            _chat.AddTyping();
            SetStatus("Đang xử lý…", false);
            Logger.Info("AskAiPane: prompt=" + promptText);

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
                    // Uu tien Agent Core (New_arch.md muc 9); Core khong dung duoc thi chay trong add-in
                    // nhu cu de pane khong bao gio bi khoa vi Core.
                    result = RunViaCore(host, promptText, run, progress);
                    if (result == null)
                    {
                        result = AiAgent.Run(host, promptText, progress, run.Token);
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
                if (!PostToUi(delegate { FinishRun(run, host, result); }))
                {
                    Logger.Error("AskAiPane: result not delivered to UI (pane closed or handle gone)", null);
                }
            });
        }

        // Chay mot luot qua Agent Core. Tra null khi Core khong dung duoc (pane se chay in-process).
        private LlmResult RunViaCore(Connect host, string promptText, CancellationTokenSource run, Action<string> progress)
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

            LlmResult result = core.Run(host, promptText, conversationId, delegate(CoreEvent item)
            {
                if (item.Type == "memory.written" && !string.IsNullOrEmpty(item.Id))
                {
                    string memoryId = item.Id;
                    string memoryText = item.Text ?? "";
                    PostToUi(delegate { ShowMemoryNote(memoryId, memoryText); });
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

            if (item.Type == "skill.loaded")
            {
                // Dòng thông tin (không có " -> ") - New_arch.md mục 7.4.
                return "(Dùng kỹ năng: " + (item.Name ?? "?") + ")";
            }

            // load_skill / read_skill_file: đã có dòng "Dùng kỹ năng", không hiện như thao tác trên tài liệu.
            if (item.Type == "tool.finished" && !string.IsNullOrEmpty(item.Tool) && item.Tool != "office_action")
            {
                return null;
            }

            if (item.Type == "tool.finished")
            {
                string outcome = !string.IsNullOrEmpty(item.ResultPreview)
                    ? item.ResultPreview
                    : (item.Ok
                        ? "{\"ok\":true}"
                        : "{\"ok\":false,\"error\":\"" + (item.Error ?? "loi") + "\"}");
                // Cung dinh dang dong tien trinh cua che do in-process ({"action": ..., "params": ...}) de
                // PaneControls hien nhan tieng Viet theo action (truoc day chi co "{}").
                string call = "{\"action\": \"" + (item.Action ?? "") + "\""
                    + (string.IsNullOrEmpty(item.ParamsPreview) ? "" : ", \"params\": " + item.ParamsPreview)
                    + "}";
                return "office_action " + call + " -> " + outcome;
            }

            if (item.Type == "run.failed")
            {
                return "(" + (item.Error ?? "loi") + ")";
            }

            return null;
        }

        // Duong dan tai lieu chuan hoa (giong Orchestrator.DocumentKey ben Core) de tra hoi thoai cu.
        // Doc COM tu worker thread nen phai qua ComGate nhu moi noi khac trong add-in.
        private static string DocumentKey(Connect host)
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
                            key = text.Trim().Replace('/', '\\').ToLowerInvariant();
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
            _newChatLink.Visible = false;
            _chat.AddInfo("Bắt đầu cuộc trò chuyện mới cho tài liệu này.");
            Logger.Info("AskAiPane: new conversation requested");
            _prompt.Focus();
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

        // Trích xuất memory chạy nền sau lượt (New_arch.md 8.5.4): hỏi lại sau ít giây để hiện "Đã ghi nhớ".
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
                SetStatus("Đang xử lý… · bước " + (_toolCount + 1), false);
            }
        }

        private void FinishRun(CancellationTokenSource run, Connect host, LlmResult result)
        {
            if (run != _run)
            {
                return;
            }
            _run = null;
            run.Dispose();
            _busy = false;
            _stopLink.Visible = false;
            _busyDot.Visible = false;
            _chat.RemoveTyping();
            _lastRunViaCore = result.ViaCore;
            if (result.ViaCore && result.Ok)
            {
                PollExtractedMemories(result.RunId);
            }
            _newChatLink.Visible = _conversationId != null;
            string seconds = result.Seconds.ToString("0.0");

            if (result.Ok)
            {
                if (!string.IsNullOrEmpty(result.Text))
                {
                    _chat.AddMessage(result.Text, false);
                    _lastReply = result.Text;
                    _insertLink.Visible = host.AppKind == "wps";
                }
                SetStatus("Xong trong " + seconds + "s · " + _toolCount + " thao tác" + BackendNote(), false);
                Announce("Xong. " + (result.Text ?? ""));
            }
            else if (result.Cancelled)
            {
                _chat.AddInfo("Đã dừng theo yêu cầu. Các thao tác đã làm vẫn còn trong tài liệu.");
                SetStatus("Đã dừng · " + _toolCount + " thao tác", false);
                Announce("Đã dừng");
            }
            else
            {
                var card = new ErrorCard("Không hoàn thành yêu cầu", DescribeError(result));
                card.RetryClicked += delegate
                {
                    if (!_busy && _lastPrompt.Length > 0)
                    {
                        RefreshState();
                        if (_blockedReason == null)
                        {
                            StartRun(_lastPrompt);
                        }
                    }
                };
                card.SettingsClicked += delegate { OpenSettings(); };
                _chat.AddBlock(card);
                SetStatus("Không hoàn thành · " + seconds + "s" + BackendNote(), true);
                Announce("Lỗi: không hoàn thành yêu cầu");
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

            return " · chế độ cơ bản";
        }

        private string DescribeError(LlmResult result)
        {
            string error = result.Error ?? "";
            string lower = error.ToLowerInvariant();
            string message;
            if (result.TimedOut)
            {
                message = "AI chưa xong sau " + (LlmClient.AgentTimeoutMs / 60000) + " phút nên đã dừng.";
            }
            else if (result.Stopped)
            {
                message = "AI đã dùng hết ngân sách token cho lượt này nên dừng lại.";
            }
            else if (result.StepLimitReached)
            {
                message = "AI chưa xong sau " + result.Rounds + " vòng nên đã dừng.";
            }
            else if (lower.Contains("timed out"))
            {
                message = "Máy chủ AI không phản hồi sau 60 giây.";
            }
            else if (lower.Contains("(401)") || lower.Contains("unauthorized"))
            {
                message = "Máy chủ AI từ chối khoá API. Kiểm tra lại khoá trong Cài đặt.";
            }
            else if (lower.Contains("unable to connect") || lower.Contains("actively refused") || lower.Contains("remote name could not be resolved"))
            {
                message = "Không kết nối được máy chủ AI (" + (Config.LlmEndpoint ?? "") + ").";
            }
            else
            {
                message = "Máy chủ AI báo lỗi: " + Truncate(error, 300);
            }
            if (_toolCount > 0)
            {
                message += " Đã làm xong " + _toolCount + " thao tác, phần đã ghi vẫn còn trong tài liệu.";
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
            SetStatus("Đang dừng…", false);
            run.Cancel();
        }

        private void OpenSettings()
        {
            using (var form = new SettingsForm())
            {
                form.ShowDialog(this);
            }
            RefreshState();
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
                SetStatus("Đã chèn vào tài liệu", false);
            }
            catch (Exception ex)
            {
                SetStatus("Chèn thất bại: " + Truncate(ex.Message, 100), true);
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

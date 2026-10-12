using System;
using System.Collections;
using System.Collections.Generic;
using System.Drawing;
using System.Globalization;
using System.Windows.Forms;

namespace AxiomOffice.Ai
{
    // Ba màn hình nhóm "Quản lý" trên ribbon (docs/ribbon-ui.md, đợt 3): Kỹ năng, Nhật ký hoạt động, Hội thoại.
    // Cùng nội dung với dialog bên LibreOffice (axiom/dialogs.py: skill_lines, audit_lines, _clock).
    internal static class AdminText
    {
        private static readonly Dictionary<string, string> SourceLabels = new Dictionary<string, string>(StringComparer.Ordinal)
        {
            { "builtin", "sẵn có" }, { "org", "tổ chức" }, { "user", "cá nhân" }
        };

        public static string Field(Dictionary<string, object> item, string key)
        {
            object value;
            return item != null && item.TryGetValue(key, out value) && value != null
                ? Convert.ToString(value, CultureInfo.InvariantCulture)
                : "";
        }

        public static string SourceLabel(string name)
        {
            string label;
            return SourceLabels.TryGetValue(name ?? "", out label) ? label : (string.IsNullOrEmpty(name) ? "?" : name);
        }

        // "2026-10-13T12:34:56.789Z" (Core lưu UTC) -> "13/10 19:34" theo giờ máy; đọc không được thì trả nguyên chuỗi.
        public static string Clock(string stamp)
        {
            DateTime when;
            if (stamp != null && stamp.Length >= 19
                && DateTime.TryParseExact(stamp.Substring(0, 19), "yyyy-MM-ddTHH:mm:ss", CultureInfo.InvariantCulture,
                    DateTimeStyles.AssumeUniversal | DateTimeStyles.AdjustToUniversal, out when))
            {
                return DateTime.SpecifyKind(when, DateTimeKind.Utc).ToLocalTime().ToString("dd/MM HH:mm", CultureInfo.InvariantCulture);
            }
            return stamp ?? "";
        }

        public static List<Dictionary<string, object>> List(Dictionary<string, object> from, string key)
        {
            var list = new List<Dictionary<string, object>>();
            object value;
            var items = from != null && from.TryGetValue(key, out value) ? value as IEnumerable : null;
            if (items == null)
            {
                return list;
            }
            foreach (object item in items)
            {
                var row = item as Dictionary<string, object>;
                if (row != null)
                {
                    list.Add(row);
                }
            }
            return list;
        }

        public static string Join(Dictionary<string, object> from, string key)
        {
            object value;
            var items = from != null && from.TryGetValue(key, out value) ? value as IEnumerable : null;
            if (items == null || items is string)
            {
                return "";
            }
            var parts = new List<string>();
            foreach (object item in items)
            {
                parts.Add(Convert.ToString(item, CultureInfo.InvariantCulture));
            }
            return string.Join(", ", parts.ToArray());
        }

        public static string KindName(string kind)
        {
            switch (kind)
            {
                case "wps": return "văn bản";
                case "et": return "bảng tính";
                case "wpp": return "trình chiếu";
                default: return "";
            }
        }
    }

    // Khung chung: danh sách + dòng trạng thái + hàng nút dưới đáy.
    internal abstract class AdminListForm : Form
    {
        protected readonly ListView List;
        protected readonly Label Status;
        protected readonly FlowLayoutPanel Buttons;

        protected AdminListForm(string title, int width, int height)
        {
            Text = "Axiom Office - " + title;
            StartPosition = FormStartPosition.CenterParent;
            MinimizeBox = false;
            ClientSize = new Size(width, height);
            MinimumSize = new Size(480, 320);
            Font = SystemFonts.MessageBoxFont;

            List = new ListView
            {
                Left = 12,
                Top = 12,
                Width = width - 24,
                Height = height - 96,
                View = View.Details,
                FullRowSelect = true,
                MultiSelect = false,
                HideSelection = false,
                Anchor = AnchorStyles.Top | AnchorStyles.Bottom | AnchorStyles.Left | AnchorStyles.Right,
            };
            Status = new Label
            {
                Left = 12,
                Top = height - 80,
                Width = width - 24,
                Height = 32,
                ForeColor = SystemColors.GrayText,
                Anchor = AnchorStyles.Bottom | AnchorStyles.Left | AnchorStyles.Right,
                Text = "Đang đọc…",
            };
            Buttons = new FlowLayoutPanel
            {
                Left = 12,
                Top = height - 44,
                Width = width - 24,
                Height = 34,
                FlowDirection = FlowDirection.LeftToRight,
                Anchor = AnchorStyles.Bottom | AnchorStyles.Left | AnchorStyles.Right,
            };
            Controls.AddRange(new Control[] { List, Status, Buttons });
            Shown += delegate { Reload(); };
        }

        protected Button AddButton(string text, EventHandler onClick)
        {
            var button = new Button { Text = text, AutoSize = true, MinimumSize = new Size(90, 0) };
            button.Click += onClick;
            Buttons.Controls.Add(button);
            return button;
        }

        protected void AddCloseButton()
        {
            Button close = AddButton("Đóng", delegate { Close(); });
            CancelButton = close;
        }

        protected Dictionary<string, object> Selected
        {
            get { return List.SelectedItems.Count == 0 ? null : List.SelectedItems[0].Tag as Dictionary<string, object>; }
        }

        protected abstract void Reload();
    }

    // Kỹ năng Core đang thấy cho app đang mở; nạp lại thư mục kỹ năng mà không khởi động lại Core.
    internal sealed class SkillsForm : AdminListForm
    {
        private readonly string _appKind;

        public SkillsForm(string appKind)
            : base("Kỹ năng", 680, 440)
        {
            _appKind = appKind;
            List.Columns.Add("Kỹ năng", 170);
            List.Columns.Add("Mô tả", 330);
            List.Columns.Add("Dùng cho", 80);
            List.Columns.Add("Nguồn", 70);
            AddButton("Nạp lại", delegate { Reload(true); });
            AddCloseButton();
        }

        protected override void Reload()
        {
            Reload(false);
        }

        private void Reload(bool reread)
        {
            string error;
            Cursor = Cursors.WaitCursor;
            if (reread && !CoreClient.Instance.ReloadSkills(out error))
            {
                Cursor = Cursors.Default;
                Status.Text = "Không nạp lại được: " + error;
                return;
            }
            Dictionary<string, object> listing = CoreClient.Instance.SkillListing(KnownKind ? _appKind : null, out error);
            Cursor = Cursors.Default;
            List.BeginUpdate();
            List.Items.Clear();
            if (listing == null)
            {
                List.EndUpdate();
                Status.Text = "Không đọc được kỹ năng: " + error;
                return;
            }
            List<Dictionary<string, object>> skills = AdminText.List(listing, "skills");
            foreach (Dictionary<string, object> skill in skills)
            {
                var row = new ListViewItem(AdminText.Field(skill, "name")) { Tag = skill };
                row.SubItems.Add(AdminText.Field(skill, "description"));
                string apps = AdminText.Join(skill, "apps");
                row.SubItems.Add(apps.Length > 0 ? apps : "mọi ứng dụng");
                row.SubItems.Add(AdminText.SourceLabel(AdminText.Field(skill, "source")));
                List.Items.Add(row);
            }
            // Thư mục nạp lỗi phải hiện ra: Core bỏ qua SKILL.md sai định dạng một cách im lặng.
            List<Dictionary<string, object>> failures = AdminText.List(listing, "errors");
            foreach (Dictionary<string, object> failure in failures)
            {
                var row = new ListViewItem("⚠ Không nạp được") { ForeColor = Color.Firebrick };
                row.SubItems.Add(AdminText.Field(failure, "directory") + ": " + AdminText.Field(failure, "error"));
                row.SubItems.Add("");
                row.SubItems.Add(AdminText.SourceLabel(AdminText.Field(failure, "source")));
                List.Items.Add(row);
            }
            List.EndUpdate();

            var parts = new List<string> { skills.Count + " kỹ năng" + (KnownKind ? " cho " + AdminText.KindName(_appKind) : "") };
            if (failures.Count > 0)
            {
                parts.Add(failures.Count + " thư mục nạp lỗi");
            }
            var sources = new List<string>();
            foreach (Dictionary<string, object> source in AdminText.List(listing, "sources"))
            {
                if (AdminText.Field(source, "exists") == "True")
                {
                    sources.Add(AdminText.SourceLabel(AdminText.Field(source, "name")));
                }
            }
            if (sources.Count > 0)
            {
                parts.Add("nguồn: " + string.Join(", ", sources.ToArray()));
            }
            Status.Text = (reread ? "Đã nạp lại · " : "") + string.Join(" · ", parts.ToArray())
                + ". AI tự chọn kỹ năng hợp với yêu cầu.";
        }

        private bool KnownKind
        {
            get { return AdminText.KindName(_appKind).Length > 0; }
        }
    }

    // Các lệnh AI đã gọi vào tài liệu (GET /v1/audit), mới nhất trước: trả lời "AI vừa làm gì với file của tôi".
    internal sealed class AuditForm : AdminListForm
    {
        private const int Limit = 100;

        public AuditForm()
            : base("Nhật ký hoạt động", 720, 460)
        {
            List.Columns.Add("Giờ", 90);
            List.Columns.Add("Lệnh", 220);
            List.Columns.Add("Kết quả", 70);
            List.Columns.Add("ms", 60);
            List.Columns.Add("Lỗi", 240);
            AddButton("Tải lại", delegate { Reload(); });
            AddCloseButton();
        }

        protected override void Reload()
        {
            string error;
            Cursor = Cursors.WaitCursor;
            List<Dictionary<string, object>> calls = CoreClient.Instance.Audit(Limit, out error);
            Cursor = Cursors.Default;
            List.BeginUpdate();
            List.Items.Clear();
            if (calls == null)
            {
                List.EndUpdate();
                Status.Text = "Không đọc được nhật ký: " + error;
                return;
            }
            int failed = 0;
            foreach (Dictionary<string, object> call in calls)
            {
                bool ok = AdminText.Field(call, "ok") == "True";
                string action = AdminText.Field(call, "action").Trim();
                string tool = AdminText.Field(call, "tool");
                var row = new ListViewItem(AdminText.Clock(AdminText.Field(call, "createdAt"))) { Tag = call };
                row.SubItems.Add(action.Length > 0 ? tool + "." + action : (tool.Length > 0 ? tool : "?"));
                row.SubItems.Add(ok ? "ok" : "LỖI");
                row.SubItems.Add(AdminText.Field(call, "ms"));
                row.SubItems.Add(AdminText.Field(call, "error"));
                if (!ok)
                {
                    failed++;
                    row.ForeColor = Color.Firebrick;
                }
                List.Items.Add(row);
            }
            List.EndUpdate();
            Status.Text = calls.Count == 0
                ? "Chưa có lượt gọi lệnh nào."
                : calls.Count + " lượt gọi lệnh (mới nhất trước, tối đa " + Limit + ")" + (failed > 0 ? " · " + failed + " lượt lỗi" : "");
        }
    }

    // Hội thoại đã lưu (GET/DELETE /v1/conversations): xem lại, xoá. Mặc định chỉ tài liệu đang mở.
    internal sealed class ConversationsForm : AdminListForm
    {
        private const int Limit = 50;
        private readonly string _documentKey;
        private readonly CheckBox _onlyDocument;
        private readonly Button _view;
        private readonly Button _delete;

        public ConversationsForm(string documentKey)
            : base("Hội thoại", 720, 460)
        {
            _documentKey = documentKey;
            List.Columns.Add("Cập nhật", 90);
            List.Columns.Add("Tiêu đề", 300);
            List.Columns.Add("Tài liệu", 200);
            List.Columns.Add("Ứng dụng", 80);
            _view = AddButton("Xem…", delegate { ShowSelected(); });
            _delete = AddButton("Xoá…", delegate { DeleteSelected(); });
            AddButton("Tải lại", delegate { Reload(); });
            AddCloseButton();
            _onlyDocument = new CheckBox
            {
                Text = "Chỉ tài liệu đang mở",
                AutoSize = true,
                Checked = !string.IsNullOrEmpty(documentKey),
                Enabled = !string.IsNullOrEmpty(documentKey),
                Margin = new Padding(16, 7, 3, 3),
            };
            Buttons.Controls.Add(_onlyDocument);
            _onlyDocument.CheckedChanged += delegate { Reload(); };
            List.SelectedIndexChanged += delegate { UpdateButtons(); };
            List.DoubleClick += delegate { ShowSelected(); };
        }

        private void UpdateButtons()
        {
            _view.Enabled = Selected != null;
            _delete.Enabled = Selected != null;
        }

        protected override void Reload()
        {
            string error;
            Cursor = Cursors.WaitCursor;
            List<Dictionary<string, object>> items = CoreClient.Instance.ListConversations(
                _onlyDocument.Checked ? _documentKey : null, Limit, out error);
            Cursor = Cursors.Default;
            List.BeginUpdate();
            List.Items.Clear();
            if (items != null)
            {
                foreach (Dictionary<string, object> item in items)
                {
                    string title = AdminText.Field(item, "title");
                    var row = new ListViewItem(AdminText.Clock(AdminText.Field(item, "updatedAt"))) { Tag = item };
                    row.SubItems.Add(title.Length > 0 ? title : "(chưa đặt tên)");
                    row.SubItems.Add(AdminText.Field(item, "documentName"));
                    row.SubItems.Add(AdminText.KindName(AdminText.Field(item, "app")));
                    List.Items.Add(row);
                }
            }
            List.EndUpdate();
            Status.Text = items == null
                ? "Không đọc được hội thoại: " + error
                : items.Count + " hội thoại" + (_onlyDocument.Checked ? " của tài liệu đang mở" : "") + ". Hội thoại nằm trên máy này (core.db).";
            UpdateButtons();
        }

        private void ShowSelected()
        {
            Dictionary<string, object> item = Selected;
            if (item == null)
            {
                return;
            }
            string error;
            Dictionary<string, object> full = CoreClient.Instance.Conversation(AdminText.Field(item, "id"), out error);
            if (full == null)
            {
                Status.Text = "Không đọc được hội thoại: " + error;
                return;
            }
            var lines = new List<string>();
            string summary = AdminText.Field(full, "summary");
            if (summary.Length > 0)
            {
                lines.Add("Tóm tắt: " + summary);
                lines.Add("");
            }
            foreach (Dictionary<string, object> message in AdminText.List(full, "messages"))
            {
                string role = AdminText.Field(message, "role");
                string who = role == "user" ? "Bạn" : role == "assistant" ? "AI" : role;
                lines.Add("[" + AdminText.Clock(AdminText.Field(message, "createdAt")) + "] " + who + ":");
                lines.Add(AdminText.Field(message, "content"));
                lines.Add("");
            }
            using (var viewer = new TextViewerForm("Hội thoại - " + AdminText.Field(item, "title"),
                lines.Count == 0 ? "(chưa có tin nhắn)" : string.Join(Environment.NewLine, lines.ToArray())))
            {
                viewer.ShowDialog(this);
            }
        }

        private void DeleteSelected()
        {
            Dictionary<string, object> item = Selected;
            if (item == null)
            {
                return;
            }
            DialogResult answer = MessageBox.Show(this,
                "Xoá hội thoại \"" + AdminText.Field(item, "title") + "\"?\nKhông khôi phục được. Ghi nhớ của AI không bị xoá.",
                "Xoá hội thoại", MessageBoxButtons.YesNo, MessageBoxIcon.Warning, MessageBoxDefaultButton.Button2);
            if (answer != DialogResult.Yes)
            {
                return;
            }
            string error;
            bool ok = CoreClient.Instance.DeleteConversation(AdminText.Field(item, "id"), out error);
            Reload();
            Status.Text = ok ? "Đã xoá hội thoại." : "Không xoá được: " + error;
        }
    }

    // Hộp xem văn bản dài, chỉ đọc (nội dung hội thoại, kết quả trạng thái).
    internal sealed class TextViewerForm : Form
    {
        public TextViewerForm(string title, string text)
        {
            Text = title;
            StartPosition = FormStartPosition.CenterParent;
            MinimizeBox = false;
            ClientSize = new Size(640, 480);
            Font = SystemFonts.MessageBoxFont;
            var box = new TextBox
            {
                Multiline = true,
                ReadOnly = true,
                ScrollBars = ScrollBars.Vertical,
                Dock = DockStyle.Fill,
                Text = text,
                BackColor = SystemColors.Window,
            };
            var close = new Button { Text = "Đóng", Dock = DockStyle.Bottom, Height = 32 };
            close.Click += delegate { Close(); };
            Controls.Add(box);
            Controls.Add(close);
            CancelButton = close;
            Shown += delegate { box.SelectionLength = 0; };
        }
    }
}

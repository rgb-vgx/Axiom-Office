using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Windows.Forms;

namespace AxiomOffice.Ai
{
    // "Quản lý ghi nhớ…" (New_arch.md mục 8.5.10, 9.4): người dùng xem, thêm, sửa, ghim, đặt hạn dùng, xoá,
    // khôi phục, xem lịch sử từng ghi nhớ và xoá toàn bộ (hỏi lại). Mọi thao tác qua Core API /v1/memory.
    internal sealed class MemoryForm : Form
    {
        private readonly TextBox _search;
        private readonly CheckBox _showDeleted;
        private readonly ListView _list;
        private readonly Label _status;
        private readonly Button _edit;
        private readonly Button _pin;
        private readonly Button _expires;
        private readonly Button _delete;
        private readonly Button _restore;
        private readonly Button _history;

        public MemoryForm()
        {
            Text = "Axiom Office - Quản lý ghi nhớ";
            StartPosition = FormStartPosition.CenterParent;
            MinimizeBox = false;
            ClientSize = new Size(760, 460);
            MinimumSize = new Size(640, 360);
            Font = SystemFonts.MessageBoxFont;

            _search = new TextBox { Left = 12, Top = 12, Width = 360, Anchor = AnchorStyles.Top | AnchorStyles.Left };
            var find = new Button { Text = "Tìm", Left = 380, Top = 10, Width = 70 };
            _showDeleted = new CheckBox { Text = "Hiện cả ghi nhớ đã xoá", Left = 462, Top = 13, Width = 200 };

            _list = new ListView
            {
                Left = 12,
                Top = 42,
                Width = 600,
                Height = 370,
                View = View.Details,
                FullRowSelect = true,
                MultiSelect = false,
                HideSelection = false,
                Anchor = AnchorStyles.Top | AnchorStyles.Bottom | AnchorStyles.Left | AnchorStyles.Right,
            };
            _list.Columns.Add("Nội dung", 290);
            _list.Columns.Add("Phạm vi", 90);
            _list.Columns.Add("Nguồn", 80);
            _list.Columns.Add("Ghim", 45);
            _list.Columns.Add("Hạn dùng", 80);
            _list.Columns.Add("Ngày tạo", 80);

            int x = 622;
            var add = MakeButton("Thêm…", x, 42);
            _edit = MakeButton("Sửa…", x, 76);
            _pin = MakeButton("Ghim / bỏ ghim", x, 110);
            _expires = MakeButton("Hạn dùng…", x, 144);
            _delete = MakeButton("Xoá", x, 178);
            _restore = MakeButton("Khôi phục", x, 212);
            _history = MakeButton("Lịch sử…", x, 246);
            var purge = MakeButton("Xoá toàn bộ…", x, 300);
            var close = MakeButton("Đóng", x, 380);
            close.Anchor = AnchorStyles.Bottom | AnchorStyles.Right;

            _status = new Label
            {
                Left = 12,
                Top = 420,
                Width = 736,
                Height = 32,
                ForeColor = SystemColors.GrayText,
                Anchor = AnchorStyles.Bottom | AnchorStyles.Left | AnchorStyles.Right,
            };

            Controls.AddRange(new Control[] { _search, find, _showDeleted, _list, add, _edit, _pin, _expires, _delete, _restore, _history, purge, close, _status });
            AcceptButton = find;
            CancelButton = close;

            find.Click += delegate { Reload(); };
            _showDeleted.CheckedChanged += delegate { Reload(); };
            _list.SelectedIndexChanged += delegate { UpdateButtons(); };
            _list.DoubleClick += delegate { EditSelected(); };
            add.Click += delegate { AddMemory(); };
            _edit.Click += delegate { EditSelected(); };
            _pin.Click += delegate { TogglePin(); };
            _expires.Click += delegate { SetExpires(); };
            _delete.Click += delegate { DeleteSelected(); };
            _restore.Click += delegate { RestoreSelected(); };
            _history.Click += delegate { ShowHistory(); };
            purge.Click += delegate { PurgeAll(); };
            close.Click += delegate { Close(); };
            Load += delegate { Reload(); };
        }

        private static Button MakeButton(string text, int left, int top)
        {
            return new Button { Text = text, Left = left, Top = top, Width = 126, Anchor = AnchorStyles.Top | AnchorStyles.Right };
        }

        private Dictionary<string, object> Selected
        {
            get { return _list.SelectedItems.Count == 0 ? null : _list.SelectedItems[0].Tag as Dictionary<string, object>; }
        }

        private static string Field(Dictionary<string, object> item, string key)
        {
            return item != null && item.ContainsKey(key) && item[key] != null ? Convert.ToString(item[key]) : "";
        }

        private void Reload()
        {
            string error;
            Cursor = Cursors.WaitCursor;
            List<Dictionary<string, object>> items = CoreClient.Instance.ListMemories(_search.Text, null, _showDeleted.Checked, out error);
            Cursor = Cursors.Default;
            _list.BeginUpdate();
            _list.Items.Clear();
            if (items != null)
            {
                foreach (Dictionary<string, object> item in items)
                {
                    bool deleted = Field(item, "deletedAt").Length > 0;
                    var row = new ListViewItem(Field(item, "text")) { Tag = item };
                    row.SubItems.Add(ScopeLabel(item));
                    row.SubItems.Add(SourceLabel(Field(item, "source")));
                    row.SubItems.Add(Field(item, "pinned") == "True" ? "✓" : "");
                    row.SubItems.Add(Field(item, "expiresAt"));
                    string created = Field(item, "createdAt");
                    row.SubItems.Add(created.Length >= 10 ? created.Substring(0, 10) : created);
                    if (deleted)
                    {
                        row.ForeColor = SystemColors.GrayText;
                        row.Text = "(đã xoá) " + row.Text;
                    }
                    _list.Items.Add(row);
                }
            }
            _list.EndUpdate();
            _status.Text = items == null
                ? "Không đọc được ghi nhớ: " + error
                : items.Count + " ghi nhớ. Ghi nhớ nằm trên máy này (core.db); AI chỉ thêm mới, sửa/xoá chỉ do bạn.";
            UpdateButtons();
        }

        private static string ScopeLabel(Dictionary<string, object> item)
        {
            string scope = Field(item, "scope");
            if (scope == "document")
            {
                string key = Field(item, "scopeKey");
                return "Tài liệu: " + (key.Length > 0 ? Path.GetFileName(key) : "?");
            }
            return scope == "user" ? "Chung" : scope;
        }

        private static string SourceLabel(string source)
        {
            switch (source)
            {
                case "user":
                    return "Bạn";
                case "agent":
                    return "AI (khi làm)";
                case "extract":
                    return "Tự ghi nhớ";
                default:
                    return source;
            }
        }

        private void UpdateButtons()
        {
            Dictionary<string, object> item = Selected;
            bool has = item != null;
            bool deleted = has && Field(item, "deletedAt").Length > 0;
            _edit.Enabled = has && !deleted;
            _pin.Enabled = has && !deleted;
            _expires.Enabled = has && !deleted;
            _delete.Enabled = has && !deleted;
            _restore.Enabled = has && deleted;
            _history.Enabled = has;
        }

        private void AddMemory()
        {
            string text = InputDialog.Ask(this, "Thêm ghi nhớ", "Một điều AI nên nhớ cho mọi tài liệu (≤ 300 ký tự), vd \"Người ký công văn: Nguyễn Văn A, Trưởng phòng\":", "");
            if (string.IsNullOrWhiteSpace(text))
            {
                return;
            }
            string error;
            Report(CoreClient.Instance.AddMemory("user", null, text.Trim(), out error) != null, "Đã thêm ghi nhớ.", error);
        }

        private void EditSelected()
        {
            Dictionary<string, object> item = Selected;
            if (item == null || Field(item, "deletedAt").Length > 0)
            {
                return;
            }
            string text = InputDialog.Ask(this, "Sửa ghi nhớ", "Nội dung (≤ 300 ký tự):", Field(item, "text"));
            if (text == null || text.Trim() == Field(item, "text"))
            {
                return;
            }
            string error;
            bool ok = CoreClient.Instance.UpdateMemory(Field(item, "id"), new Dictionary<string, object> { { "text", text.Trim() } }, out error);
            Report(ok, "Đã sửa ghi nhớ.", error);
        }

        private void TogglePin()
        {
            Dictionary<string, object> item = Selected;
            if (item == null)
            {
                return;
            }
            bool pinned = Field(item, "pinned") == "True";
            string error;
            bool ok = CoreClient.Instance.UpdateMemory(Field(item, "id"), new Dictionary<string, object> { { "pinned", !pinned } }, out error);
            Report(ok, pinned ? "Đã bỏ ghim." : "Đã ghim: ghi nhớ luôn được đưa cho AI.", error);
        }

        private void SetExpires()
        {
            Dictionary<string, object> item = Selected;
            if (item == null)
            {
                return;
            }
            string value = InputDialog.Ask(this, "Hạn dùng", "Ngày hết hạn dạng YYYY-MM-DD (bỏ trống = không hết hạn). Hết hạn thì AI không dùng nữa:", Field(item, "expiresAt"));
            if (value == null)
            {
                return;
            }
            object expires = value.Trim().Length == 0 ? null : (object)value.Trim();
            string error;
            bool ok = CoreClient.Instance.UpdateMemory(Field(item, "id"), new Dictionary<string, object> { { "expiresAt", expires } }, out error);
            Report(ok, "Đã đặt hạn dùng.", error);
        }

        private void DeleteSelected()
        {
            Dictionary<string, object> item = Selected;
            if (item == null)
            {
                return;
            }
            string error;
            Report(CoreClient.Instance.DeleteMemory(Field(item, "id"), out error), "Đã xoá (khôi phục được trong 30 ngày).", error);
        }

        private void RestoreSelected()
        {
            Dictionary<string, object> item = Selected;
            if (item == null)
            {
                return;
            }
            string error;
            Report(CoreClient.Instance.RestoreMemory(Field(item, "id"), out error), "Đã khôi phục.", error);
        }

        private void ShowHistory()
        {
            Dictionary<string, object> item = Selected;
            if (item == null)
            {
                return;
            }
            string error;
            List<Dictionary<string, object>> history = CoreClient.Instance.MemoryHistory(Field(item, "id"), out error);
            if (history == null)
            {
                _status.Text = "Không đọc được lịch sử: " + error;
                return;
            }
            var lines = new List<string>();
            foreach (Dictionary<string, object> entry in history)
            {
                string line = Field(entry, "createdAt").Replace("T", " ").TrimEnd('Z') + "  " + Field(entry, "event") + " (" + SourceLabel(Field(entry, "actor")) + ")";
                if (Field(entry, "newText").Length > 0)
                {
                    line += ": " + Field(entry, "newText");
                }
                lines.Add(line);
            }
            MessageBox.Show(this, lines.Count == 0 ? "(chưa có)" : string.Join(Environment.NewLine, lines.ToArray()), "Lịch sử ghi nhớ", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }

        private void PurgeAll()
        {
            DialogResult answer = MessageBox.Show(this,
                "Xoá VĨNH VIỄN toàn bộ ghi nhớ của AI (không khôi phục được)?",
                "Xoá toàn bộ ghi nhớ", MessageBoxButtons.YesNo, MessageBoxIcon.Warning, MessageBoxDefaultButton.Button2);
            if (answer != DialogResult.Yes)
            {
                return;
            }
            string error;
            Report(CoreClient.Instance.PurgeMemories(out error), "Đã xoá toàn bộ ghi nhớ.", error);
        }

        private void Report(bool ok, string message, string error)
        {
            Reload();
            _status.Text = ok ? message : "Không thực hiện được: " + error;
        }
    }

    // Hộp nhập một dòng/đoạn ngắn (WinForms không có sẵn InputBox).
    internal sealed class InputDialog : Form
    {
        private readonly TextBox _input;

        private InputDialog(string title, string prompt, string value)
        {
            Text = title;
            FormBorderStyle = FormBorderStyle.FixedDialog;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterParent;
            ClientSize = new Size(460, 170);
            Font = SystemFonts.MessageBoxFont;
            var label = new Label { Text = prompt, Left = 12, Top = 12, Width = 436, Height = 40 };
            _input = new TextBox { Left = 12, Top = 56, Width = 436, Height = 60, Multiline = true, Text = value ?? "", MaxLength = 300 };
            var ok = new Button { Text = "OK", Left = 282, Top = 130, Width = 80, DialogResult = DialogResult.OK };
            var cancel = new Button { Text = "Huỷ", Left = 368, Top = 130, Width = 80, DialogResult = DialogResult.Cancel };
            Controls.AddRange(new Control[] { label, _input, ok, cancel });
            AcceptButton = ok;
            CancelButton = cancel;
        }

        public static string Ask(IWin32Window owner, string title, string prompt, string value)
        {
            using (var dialog = new InputDialog(title, prompt, value))
            {
                return dialog.ShowDialog(owner) == DialogResult.OK ? dialog._input.Text.Replace("\r\n", " ").Replace("\n", " ") : null;
            }
        }
    }
}

using System;
using System.Collections;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Text;
using System.Text.RegularExpressions;
using System.Web.Script.Serialization;
using System.Windows.Forms;

namespace AxiomOffice.Ai
{
    // Đọc/ghi %LOCALAPPDATA%\AxiomOffice\mcp.json — cùng định dạng Core Go đọc
    // (core-go/internal/mcp/manager.go LoadConfigs): {"mcpServers": {tên: {command, args[],
    // env{}, url, headers{}, trusted, disabled}}}. Ghi UTF-8 không BOM, giữ nguyên các
    // khoá lạ (kể cả khoá cấp root) để không làm mất cấu hình thủ công của người dùng.
    internal static class McpFile
    {
        public static string ConfigPath
        {
            get
            {
                return Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                    "AxiomOffice", "mcp.json");
            }
        }

        public static Dictionary<string, Dictionary<string, object>> Load(out string error)
        {
            var servers = new Dictionary<string, Dictionary<string, object>>(StringComparer.Ordinal);
            error = null;
            string path = ConfigPath;
            if (!File.Exists(path))
            {
                return servers;
            }
            string text;
            try
            {
                text = File.ReadAllText(path, Encoding.UTF8);
            }
            catch (Exception ex)
            {
                error = "Không đọc được mcp.json: " + ex.Message;
                return null;
            }
            Dictionary<string, object> root;
            try
            {
                root = new JavaScriptSerializer().DeserializeObject(text) as Dictionary<string, object>;
            }
            catch (Exception ex)
            {
                error = "mcp.json không phải JSON hợp lệ: " + ex.Message;
                return null;
            }
            if (root == null)
            {
                error = "mcp.json không phải JSON object.";
                return null;
            }
            object raw;
            var section = root.TryGetValue("mcpServers", out raw) ? raw as Dictionary<string, object> : null;
            if (section == null)
            {
                return servers;
            }
            foreach (KeyValuePair<string, object> pair in section)
            {
                var entry = pair.Value as Dictionary<string, object>;
                if (entry != null)
                {
                    servers[pair.Key] = entry;
                }
            }
            return servers;
        }

        public static bool Save(Dictionary<string, Dictionary<string, object>> servers, out string error)
        {
            error = null;
            Dictionary<string, object> root = new Dictionary<string, object>(StringComparer.Ordinal);
            string path = ConfigPath;
            if (File.Exists(path))
            {
                try
                {
                    var parsed = new JavaScriptSerializer().DeserializeObject(
                        File.ReadAllText(path, Encoding.UTF8)) as Dictionary<string, object>;
                    if (parsed != null)
                    {
                        root = parsed;
                    }
                }
                catch (Exception ex)
                {
                    error = "mcp.json hiện tại hỏng, không dám ghi đè: " + ex.Message;
                    return false;
                }
            }
            var section = new Dictionary<string, object>(StringComparer.Ordinal);
            foreach (KeyValuePair<string, Dictionary<string, object>> pair in servers)
            {
                section[pair.Key] = pair.Value;
            }
            root["mcpServers"] = section;
            try
            {
                Directory.CreateDirectory(Path.GetDirectoryName(path));
                File.WriteAllText(path, new JavaScriptSerializer().Serialize(root), new UTF8Encoding(false));
                return true;
            }
            catch (Exception ex)
            {
                error = "Không ghi được mcp.json: " + ex.Message;
                return false;
            }
        }

        public static string Str(Dictionary<string, object> entry, string key)
        {
            object value;
            return entry != null && entry.TryGetValue(key, out value) && value is string
                ? ((string)value).Trim()
                : "";
        }

        public static bool Flag(Dictionary<string, object> entry, string key)
        {
            object value;
            if (entry == null || !entry.TryGetValue(key, out value) || value == null)
            {
                return false;
            }
            bool ok;
            if (value is bool)
            {
                return (bool)value;
            }
            if (bool.TryParse(Convert.ToString(value), out ok))
            {
                return ok;
            }
            return false;
        }

        public static List<string> StrList(Dictionary<string, object> entry, string key)
        {
            var list = new List<string>();
            object value;
            if (entry == null || !entry.TryGetValue(key, out value) || value is string)
            {
                return list;
            }
            var items = value as IEnumerable;
            if (items == null)
            {
                return list;
            }
            foreach (object item in items)
            {
                if (item is string)
                {
                    list.Add((string)item);
                }
            }
            return list;
        }

        public static Dictionary<string, string> StrMap(Dictionary<string, object> entry, string key)
        {
            var map = new Dictionary<string, string>(StringComparer.Ordinal);
            object value;
            var raw = entry != null && entry.TryGetValue(key, out value) ? value as Dictionary<string, object> : null;
            if (raw == null)
            {
                return map;
            }
            foreach (KeyValuePair<string, object> pair in raw)
            {
                if (pair.Value is string)
                {
                    map[pair.Key] = (string)pair.Value;
                }
            }
            return map;
        }
    }

    // Công cụ ngoài (MCP server) cho AI: gộp mcp.json trên đĩa (kể cả server đang tắt) với
    // GET /v1/mcp (tool đang nạp, lỗi khởi động). Đổi gì cũng có hiệu lực từ lượt AI sau.
    internal sealed class McpForm : AdminListForm
    {
        public McpForm()
            : base("Công cụ ngoài (MCP)", 780, 470)
        {
            List.Columns.Add("Tên", 140);
            List.Columns.Add("Chạy bằng", 250);
            List.Columns.Add("Tin cậy", 70);
            List.Columns.Add("Tool", 70);
            List.Columns.Add("Trạng thái", 210);
            AddButton("Thêm…", delegate { AddServer(); });
            AddButton("Sửa…", delegate { EditSelected(); });
            AddButton("Xoá…", delegate { DeleteSelected(); });
            AddButton("Bật/Tắt", delegate { ToggleSelected("disabled"); });
            AddButton("Tin cậy", delegate { ToggleSelected("trusted"); });
            AddButton("Tải lại", delegate { Reload(); });
            AddCloseButton();
            List.DoubleClick += delegate { EditSelected(); };
        }

        protected override void Reload()
        {
            string error;
            Cursor = Cursors.WaitCursor;
            Dictionary<string, Dictionary<string, object>> file = McpFile.Load(out error);
            Dictionary<string, Dictionary<string, object>> live = LiveByName();
            Cursor = Cursors.Default;
            List.BeginUpdate();
            List.Items.Clear();
            if (file == null)
            {
                List.EndUpdate();
                Status.Text = error ?? "Không đọc được mcp.json.";
                return;
            }
            var names = new List<string>(file.Keys);
            if (live != null)
            {
                foreach (string name in live.Keys)
                {
                    if (!names.Contains(name))
                    {
                        names.Add(name + " (ngoài file)");
                    }
                }
            }
            names.Sort(StringComparer.Ordinal);
            foreach (string key in names)
            {
                string name = key.EndsWith(" (ngoài file)", StringComparison.Ordinal)
                    ? key.Substring(0, key.Length - " (ngoài file)".Length)
                    : key;
                Dictionary<string, object> entry = file.ContainsKey(name) ? file[name] : null;
                Dictionary<string, object> state = live != null && live.ContainsKey(name) ? live[name] : null;
                List.Items.Add(BuildRow(name, entry, state));
            }
            List.EndUpdate();
            Status.Text = (error ?? "").Trim().Length > 0 ? error
                : names.Count == 0 ? "Chưa có server nào. Bấm Thêm… để cắm công cụ ngoài cho AI (tìm web, đọc file…)."
                : names.Count + " server. Đổi gì cũng có hiệu lực từ lượt AI sau, không cần khởi động lại Core."
                + (live == null ? " (Core chưa trả lời — chỉ hiện cấu hình trên đĩa.)" : "");
        }

        // live theo tên server: name → payload /v1/mcp (tools, trusted, builtIn, error).
        private Dictionary<string, Dictionary<string, object>> LiveByName()
        {
            string error;
            List<Dictionary<string, object>> servers = CoreClient.Instance.McpServers(out error);
            if (servers == null)
            {
                return null;
            }
            var byName = new Dictionary<string, Dictionary<string, object>>(StringComparer.Ordinal);
            foreach (Dictionary<string, object> server in servers)
            {
                string name = AdminText.Field(server, "name");
                if (name.Length > 0)
                {
                    byName[name] = server;
                }
            }
            return byName;
        }

        private ListViewItem BuildRow(string name, Dictionary<string, object> entry, Dictionary<string, object> state)
        {
            bool isOffice = name == "office";
            bool disabled = McpFile.Flag(entry, "disabled");
            string liveError = AdminText.Field(state, "error");
            var row = new ListViewItem(name);
            row.Tag = RowTag(name, entry);
            row.SubItems.Add(RunAs(name, entry));
            bool trusted = state != null ? McpFile.Flag(state, "trusted") : McpFile.Flag(entry, "trusted");
            if (isOffice && entry == null)
            {
                trusted = true;
            }
            row.SubItems.Add(trusted ? "có" : "không");
            int tools = state == null ? -1 : CountItems(state, "tools");
            row.SubItems.Add(tools < 0 ? "—" : tools + " tool");
            if (liveError.Length > 0)
            {
                row.SubItems.Add("Lỗi khởi động");
                row.ForeColor = Color.Firebrick;
            }
            else if (disabled)
            {
                row.SubItems.Add("Đã tắt");
                row.ForeColor = SystemColors.GrayText;
            }
            else if (state != null)
            {
                row.SubItems.Add(isOffice ? "Sẵn có" : "Đã nạp");
            }
            else
            {
                row.SubItems.Add("Sẽ nạp ở lượt sau");
            }
            return row;
        }

        private static Dictionary<string, object> RowTag(string name, Dictionary<string, object> entry)
        {
            return new Dictionary<string, object>(StringComparer.Ordinal)
            {
                { "name", name },
                { "entry", entry },
            };
        }

        private static string RunAs(string name, Dictionary<string, object> entry)
        {
            if (entry == null)
            {
                return name == "office" ? "(sẵn có trong Core)" : "(ngoài file)";
            }
            string command = McpFile.Str(entry, "command");
            string url = McpFile.Str(entry, "url");
            if (command.Length > 0)
            {
                List<string> args = McpFile.StrList(entry, "args");
                string full = command + (args.Count > 0 ? " " + string.Join(" ", args.ToArray()) : "");
                return full.Length > 60 ? full.Substring(0, 60) + "…" : full;
            }
            if (url.Length > 0)
            {
                return url.Length > 60 ? url.Substring(0, 60) + "…" : url;
            }
            return name == "office" ? "(sẵn có trong Core)" : "(thiếu command/url)";
        }

        private static int CountItems(Dictionary<string, object> row, string key)
        {
            object value;
            var items = row != null && row.TryGetValue(key, out value) ? value as IEnumerable : null;
            if (items == null || items is string)
            {
                return 0;
            }
            int count = 0;
            foreach (object item in items)
            {
                count++;
            }
            return count;
        }

        private string SelectedName(out Dictionary<string, object> entry)
        {
            entry = null;
            Dictionary<string, object> tag = Selected;
            if (tag == null)
            {
                return null;
            }
            object raw;
            entry = tag.TryGetValue("entry", out raw) ? raw as Dictionary<string, object> : null;
            return AdminText.Field(tag, "name");
        }

        private void AddServer()
        {
            using (var editor = new McpServerEditForm(null, null))
            {
                if (editor.ShowDialog(this) != DialogResult.OK)
                {
                    return;
                }
                SaveOne(editor.ServerName, editor.Entry, "Đã thêm server \"" + editor.ServerName + "\".");
            }
        }

        private void EditSelected()
        {
            Dictionary<string, object> entry;
            string name = SelectedName(out entry);
            if (name == null)
            {
                return;
            }
            using (var editor = new McpServerEditForm(name, entry))
            {
                if (editor.ShowDialog(this) != DialogResult.OK)
                {
                    return;
                }
                SaveOne(name, editor.Entry, "Đã lưu server \"" + name + "\".");
            }
        }

        private void DeleteSelected()
        {
            Dictionary<string, object> entry;
            string name = SelectedName(out entry);
            if (name == null)
            {
                return;
            }
            if (name == "office")
            {
                MessageBox.Show(this, "Server sẵn có \"office\" không xoá được — chỉ tắt được (nút Bật/Tắt).",
                    "Công cụ ngoài", MessageBoxButtons.OK, MessageBoxIcon.Information);
                return;
            }
            DialogResult answer = MessageBox.Show(this,
                "Xoá server \"" + name + "\" khỏi mcp.json?\nAI sẽ không còn tool của server này.",
                "Xoá server", MessageBoxButtons.YesNo, MessageBoxIcon.Warning, MessageBoxDefaultButton.Button2);
            if (answer != DialogResult.Yes)
            {
                return;
            }
            string error;
            Dictionary<string, Dictionary<string, object>> servers = McpFile.Load(out error);
            if (servers == null)
            {
                Status.Text = error;
                return;
            }
            servers.Remove(name);
            if (!McpFile.Save(servers, out error))
            {
                Status.Text = error;
                return;
            }
            Reload();
            Status.Text = "Đã xoá server \"" + name + "\" — có hiệu lực từ lượt AI sau.";
        }

        private void ToggleSelected(string key)
        {
            Dictionary<string, object> entry;
            string name = SelectedName(out entry);
            if (name == null)
            {
                return;
            }
            string error;
            Dictionary<string, Dictionary<string, object>> servers = McpFile.Load(out error);
            if (servers == null)
            {
                Status.Text = error;
                return;
            }
            Dictionary<string, object> current;
            if (!servers.TryGetValue(name, out current) || current == null)
            {
                current = new Dictionary<string, object>(StringComparer.Ordinal);
                servers[name] = current;
            }
            bool value = !McpFile.Flag(current, key);
            if (!value)
            {
                current.Remove(key);
            }
            else
            {
                current[key] = true;
            }
            if (!McpFile.Save(servers, out error))
            {
                Status.Text = error;
                return;
            }
            Reload();
            Status.Text = key == "disabled"
                ? "Server \"" + name + "\" đã " + (value ? "tắt" : "bật") + " — có hiệu lực từ lượt AI sau."
                : "Server \"" + name + "\" đã " + (value ? "được tin cậy (AI gọi không cần hỏi)" : "bị bỏ tin cậy (mỗi lần gọi AI đều hỏi)") + ".";
        }

        private void SaveOne(string name, Dictionary<string, object> entry, string done)
        {
            string error;
            Dictionary<string, Dictionary<string, object>> servers = McpFile.Load(out error);
            if (servers == null)
            {
                Status.Text = error;
                return;
            }
            servers[name] = entry;
            if (!McpFile.Save(servers, out error))
            {
                Status.Text = error;
                return;
            }
            Reload();
            Status.Text = done + " Có hiệu lực từ lượt AI sau.";
        }
    }

    // Thêm/sửa một MCP server: chạy lệnh (stdio) hoặc gọi URL (Streamable HTTP).
    internal sealed class McpServerEditForm : Form
    {
        private static readonly Regex NamePattern = new Regex("^[A-Za-z0-9_-]{1,64}$");

        private readonly bool _isNew;
        private readonly bool _isOffice;
        private readonly TextBox _name;
        private readonly RadioButton _stdio;
        private readonly RadioButton _http;
        private readonly TextBox _command;
        private readonly TextBox _args;
        private readonly TextBox _env;
        private readonly TextBox _url;
        private readonly TextBox _headers;
        private readonly CheckBox _trusted;
        private readonly CheckBox _disabled;
        private readonly Label _error;

        public string ServerName { get; private set; }

        public Dictionary<string, object> Entry { get; private set; }

        public McpServerEditForm(string name, Dictionary<string, object> entry)
        {
            _isNew = string.IsNullOrEmpty(name);
            _isOffice = name == "office";
            Text = _isNew ? "Axiom Office - Thêm công cụ ngoài" : "Axiom Office - Sửa \"" + name + "\"";
            StartPosition = FormStartPosition.CenterParent;
            MinimizeBox = false;
            ClientSize = new Size(560, 560);
            MinimumSize = new Size(520, 480);
            Font = SystemFonts.MessageBoxFont;
            ServerName = name ?? "";
            Entry = entry;

            int top = 12;
            Controls.Add(MakeLabel("Tên server (chữ, số, -, _):", 12, top));
            _name = new TextBox { Left = 200, Top = top - 3, Width = 348, Text = ServerName, ReadOnly = !_isNew };
            Controls.Add(_name);
            top += 30;

            _stdio = new RadioButton { Text = "Chạy lệnh trên máy (stdio)", Left = 12, Top = top, Width = 250, Checked = true };
            _http = new RadioButton { Text = "Gọi địa chỉ HTTP", Left = 270, Top = top, Width = 200 };
            Controls.Add(_stdio);
            Controls.Add(_http);
            top += 28;

            Controls.Add(MakeLabel("Lệnh chạy:", 12, top));
            _command = new TextBox { Left = 200, Top = top - 3, Width = 268 };
            var browse = new Button { Text = "Chọn…", Left = 474, Top = top - 4, Width = 74 };
            browse.Click += delegate
            {
                using (var picker = new OpenFileDialog { Filter = "Chương trình (*.exe;*.cmd;*.bat;*.js)|*.exe;*.cmd;*.bat;*.js|Mọi tệp (*.*)|*.*" })
                {
                    if (picker.ShowDialog(this) == DialogResult.OK)
                    {
                        _command.Text = picker.FileName;
                    }
                }
            };
            Controls.Add(_command);
            Controls.Add(browse);
            top += 30;

            Controls.Add(MakeLabel("Tham số (mỗi dòng một cái):", 12, top));
            _args = new TextBox { Left = 200, Top = top - 3, Width = 348, Height = 56, Multiline = true, ScrollBars = ScrollBars.Vertical };
            Controls.Add(_args);
            top += 64;

            Controls.Add(MakeLabel("Biến môi trường (MỖI=DÒNG):", 12, top));
            _env = new TextBox { Left = 200, Top = top - 3, Width = 348, Height = 56, Multiline = true, ScrollBars = ScrollBars.Vertical };
            Controls.Add(_env);
            top += 64;

            Controls.Add(MakeLabel("Địa chỉ HTTP:", 12, top));
            _url = new TextBox { Left = 200, Top = top - 3, Width = 348 };
            Controls.Add(_url);
            top += 30;

            Controls.Add(MakeLabel("Header (Key: value mỗi dòng):", 12, top));
            _headers = new TextBox { Left = 200, Top = top - 3, Width = 348, Height = 56, Multiline = true, ScrollBars = ScrollBars.Vertical };
            Controls.Add(_headers);
            top += 64;

            _trusted = new CheckBox { Text = "Tin cậy — AI gọi không cần hỏi lại", Left = 12, Top = top, Width = 536, AutoSize = true };
            Controls.Add(_trusted);
            top += 26;
            var trustNote = MakeLabel("Bỏ chọn: mỗi lần AI định gọi tool của server này đều hiện thẻ Đồng ý / Từ chối.", 32, top);
            trustNote.ForeColor = SystemColors.GrayText;
            Controls.Add(trustNote);
            top += 24;

            _disabled = new CheckBox { Text = "Tắt server này", Left = 12, Top = top, Width = 536, AutoSize = true };
            Controls.Add(_disabled);
            top += 30;

            _error = new Label { Left = 12, Top = top, Width = 536, Height = 34, ForeColor = Color.Firebrick };
            Controls.Add(_error);
            top += 38;

            var ok = new Button { Text = "Lưu", Left = 384, Top = top, Width = 80, DialogResult = DialogResult.None };
            var cancel = new Button { Text = "Huỷ", Left = 470, Top = top, Width = 80, DialogResult = DialogResult.Cancel };
            ok.Click += delegate { Save(); };
            Controls.Add(ok);
            Controls.Add(cancel);
            AcceptButton = ok;
            CancelButton = cancel;

            _stdio.CheckedChanged += delegate { SwitchMode(); };
            _http.CheckedChanged += delegate { SwitchMode(); };
            Fill(entry);
            SwitchMode();
            if (_isOffice)
            {
                LockForOffice();
            }
        }

        private static Label MakeLabel(string text, int left, int top)
        {
            return new Label { Text = text, Left = left, Top = top, Width = 182, AutoSize = false };
        }

        private void Fill(Dictionary<string, object> entry)
        {
            if (entry == null)
            {
                return;
            }
            _command.Text = McpFile.Str(entry, "command");
            _args.Lines = McpFile.StrList(entry, "args").ToArray();
            _env.Lines = MapLines(McpFile.StrMap(entry, "env"), "=");
            _url.Text = McpFile.Str(entry, "url");
            _headers.Lines = MapLines(McpFile.StrMap(entry, "headers"), ": ");
            _trusted.Checked = McpFile.Flag(entry, "trusted");
            _disabled.Checked = McpFile.Flag(entry, "disabled");
            if (McpFile.Str(entry, "url").Length > 0)
            {
                _http.Checked = true;
            }
        }

        private static string[] MapLines(Dictionary<string, string> map, string separator)
        {
            var lines = new List<string>();
            foreach (KeyValuePair<string, string> pair in map)
            {
                lines.Add(pair.Key + separator + pair.Value);
            }
            return lines.ToArray();
        }

        private void SwitchMode()
        {
            bool stdio = _stdio.Checked;
            _command.Enabled = stdio;
            _args.Enabled = stdio;
            _env.Enabled = stdio;
            _url.Enabled = !stdio;
            _headers.Enabled = !stdio;
        }

        private void LockForOffice()
        {
            foreach (Control control in Controls)
            {
                if (control is Button || control == _disabled || control == _error)
                {
                    continue;
                }
                control.Enabled = false;
            }
            _error.Text = "";
            var note = new Label
            {
                Text = "Server sẵn có của Core (đọc/ghi file docx/xlsx/pptx, không cần mở app). Chỉ tắt được, không sửa.",
                Left = 12,
                Top = 44,
                Width = 536,
                Height = 30,
                ForeColor = SystemColors.GrayText,
            };
            Controls.Add(note);
        }

        private void Save()
        {
            string name = _isNew ? _name.Text.Trim() : ServerName;
            if (_isNew && !NamePattern.IsMatch(name))
            {
                _error.Text = "Tên chỉ gồm chữ cái, số, - và _ (tối đa 64 ký tự).";
                return;
            }
            Dictionary<string, object> kept = Entry;
            var entry = new Dictionary<string, object>(StringComparer.Ordinal);
            if (_isOffice && kept != null)
            {
                // Giữ command/url tuỳ biến (nếu có) của server sẵn có, chỉ đổi cờ.
                foreach (KeyValuePair<string, object> pair in kept)
                {
                    if (pair.Key != "trusted" && pair.Key != "disabled")
                    {
                        entry[pair.Key] = pair.Value;
                    }
                }
            }
            else if (!_isOffice)
            {
                if (_stdio.Checked)
                {
                    if (_command.Text.Trim().Length == 0)
                    {
                        _error.Text = "Chế độ chạy lệnh cần có Lệnh chạy (vd node, python).";
                        return;
                    }
                    entry["command"] = _command.Text.Trim();
                    List<string> args = SplitLines(_args.Lines);
                    if (args.Count > 0)
                    {
                        entry["args"] = args.ToArray();
                    }
                    Dictionary<string, string> env = ParseMap(_env.Lines, '=');
                    if (env.Count > 0)
                    {
                        entry["env"] = env;
                    }
                }
                else
                {
                    if (_url.Text.Trim().Length == 0
                        || (!_url.Text.Trim().StartsWith("http://") && !_url.Text.Trim().StartsWith("https://")))
                    {
                        _error.Text = "Chế độ HTTP cần Địa chỉ bắt đầu bằng http:// hoặc https://.";
                        return;
                    }
                    entry["url"] = _url.Text.Trim();
                    Dictionary<string, string> headers = ParseMap(_headers.Lines, ':');
                    if (headers.Count > 0)
                    {
                        entry["headers"] = headers;
                    }
                }
            }
            if (_trusted.Checked)
            {
                entry["trusted"] = true;
            }
            if (_disabled.Checked)
            {
                entry["disabled"] = true;
            }
            ServerName = name;
            Entry = entry;
            DialogResult = DialogResult.OK;
            Close();
        }

        private static List<string> SplitLines(string[] lines)
        {
            var list = new List<string>();
            if (lines == null)
            {
                return list;
            }
            foreach (string line in lines)
            {
                string trimmed = line.Trim();
                if (trimmed.Length > 0)
                {
                    list.Add(trimmed);
                }
            }
            return list;
        }

        private static Dictionary<string, string> ParseMap(string[] lines, char separator)
        {
            var map = new Dictionary<string, string>(StringComparer.Ordinal);
            if (lines == null)
            {
                return map;
            }
            foreach (string line in lines)
            {
                int at = line.IndexOf(separator);
                if (at <= 0)
                {
                    continue;
                }
                string key = line.Substring(0, at).Trim();
                string value = line.Substring(at + 1).Trim();
                if (key.Length > 0)
                {
                    map[key] = value;
                }
            }
            return map;
        }
    }
}

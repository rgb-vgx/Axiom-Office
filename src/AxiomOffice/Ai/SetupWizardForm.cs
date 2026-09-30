using System;
using System.Collections.Generic;
using System.Drawing;
using System.Threading;
using System.Threading.Tasks;
using System.Windows.Forms;
using AxiomOffice.Bridge;
using AxiomOffice.Setup;

namespace AxiomOffice.Ai
{
    // Wizard thiet lap cho nguoi dung khong chuyen (New_arch.md muc 9; cung noi dung voi
    // setupwizard.py cua ban Linux - catalog/setup.json sinh ra SetupCatalog.cs).
    // 5 buoc: chao mung -> kiem tra may -> noi may chu AI -> tinh nang -> hoan tat.
    // Khong co Tab control: cac buoc la cac panel an/hien.
    internal sealed class SetupWizardForm : Form
    {
        private const int Width0 = 640;
        private const int Height0 = 560;

        private readonly Connect _host;
        private readonly SetupInfo _info;
        private readonly string _catalogProblem;

        private readonly Panel _content = new Panel();
        private readonly Label _stepLabel = new Label();
        private readonly Label _titleLabel = new Label();
        private readonly Label _subtitleLabel = new Label();
        private readonly Panel _footer = new Panel();
        private readonly ChipButton _back = new ChipButton("← Quay lại");
        private readonly ChipButton _later = new ChipButton("Để sau");
        private readonly SendButton _next = new SendButton();

        private readonly List<Panel> _panels = new List<Panel>();
        private int _step;

        // Buoc 3: lua chon nha cung cap + cac o nhap.
        private readonly List<ChipButton> _providerChips = new List<ChipButton>();
        private string _providerId = "company";
        private readonly TextBox _endpoint = new TextBox();
        private readonly TextBox _apiKey = new TextBox();
        private readonly ComboBox _model = new ComboBox();
        private readonly Label _keyLink = new Label();
        private readonly Label _probe = new Label();
        private readonly ChipButton _test = new ChipButton("Kiểm tra kết nối");
        private readonly ChipButton _loadModels = new ChipButton("Tải danh sách model");
        private readonly ChipButton _showKey = new ChipButton("Xem khoá");
        private bool _saved;

        // Buoc 2: danh sach kiem tra (nhan + ket qua + nut Sua).
        private readonly Panel _checks = new Panel();
        private readonly List<Label> _checkLines = new List<Label>();

        // Buoc 4: tinh nang.
        private readonly CheckBox _memory = new CheckBox();
        private readonly CheckBox _autoExtract = new CheckBox();
        private readonly CheckBox _visualQa = new CheckBox();
        private readonly ChipButton _advanced = new ChipButton("Tuỳ chọn nâng cao…");

        // Buoc 5: thu ngay.
        private readonly Label _summary = new Label();
        private readonly ChipButton _tryNow = new ChipButton("Thử ngay trên tài liệu đang mở");
        private readonly Label _tryResult = new Label();

        public SetupWizardForm(Connect host)
        {
            _host = host;
            string error;
            _info = CoreClient.Instance.Setup(out error);
            _catalogProblem = error;

            Text = "Thiết lập Axiom Office";
            FormBorderStyle = FormBorderStyle.FixedDialog;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterScreen;
            ClientSize = new Size(Width0, Height0);
            Font = PaneTheme.Body;
            BackColor = PaneTheme.PaneBg;
            KeyPreview = true;

            BuildChrome();
            BuildWelcome();
            BuildChecks();
            BuildConnect();
            BuildFeatures();
            BuildDone();
            ShowStep(Configured() ? 1 : 0);
        }

        // Da co endpoint + model: wizard vao thang buoc kiem tra (khiem luon man sua chua).
        private bool Configured()
        {
            return !string.IsNullOrEmpty(Config.LlmEndpoint) && !string.IsNullOrEmpty(Config.LlmModel);
        }

        // ---------- khung ----------

        private void BuildChrome()
        {
            _stepLabel.AutoSize = false;
            _stepLabel.Location = new Point(24, 18);
            _stepLabel.Size = new Size(Width0 - 48, 18);
            _stepLabel.Font = PaneTheme.Caption;
            _stepLabel.ForeColor = PaneTheme.TextMuted;

            _titleLabel.AutoSize = false;
            _titleLabel.Location = new Point(24, 38);
            _titleLabel.Size = new Size(Width0 - 48, 28);
            _titleLabel.Font = PaneTheme.EmptyTitle;
            _titleLabel.ForeColor = PaneTheme.TextPrimary;

            _subtitleLabel.AutoSize = false;
            _subtitleLabel.Location = new Point(24, 68);
            _subtitleLabel.Size = new Size(Width0 - 48, 34);
            _subtitleLabel.Font = PaneTheme.Small;
            _subtitleLabel.ForeColor = PaneTheme.TextSecondary;

            _content.Location = new Point(0, 106);
            _content.Size = new Size(Width0, Height0 - 106 - 64);
            _content.BackColor = PaneTheme.PaneBg;

            _footer.Location = new Point(0, Height0 - 64);
            _footer.Size = new Size(Width0, 64);
            _footer.BackColor = PaneTheme.PaneBg;

            _later.Location = new Point(24, 16);
            _later.Size = new Size(110, 32);
            _later.Click += delegate { Close(); };

            _back.Location = new Point(Width0 - 24 - 260, 16);
            _back.Size = new Size(120, 32);
            _back.Click += delegate { ShowStep(_step - 1); };

            _next.Location = new Point(Width0 - 24 - 130, 16);
            _next.Size = new Size(130, 32);
            _next.Click += delegate { OnNext(); };

            var buttons = new Control[] { _later, _back, _next };
            foreach (Control control in buttons)
            {
                control.Anchor = AnchorStyles.Bottom | AnchorStyles.Left | AnchorStyles.Right;
            }

            _footer.Controls.AddRange(buttons);
            Controls.AddRange(new Control[] { _stepLabel, _titleLabel, _subtitleLabel, _content, _footer });
            KeyDown += delegate (object sender, KeyEventArgs e)
            {
                if (e.KeyCode == Keys.Escape)
                {
                    Close();
                }
            };
        }

        private Panel NewStep()
        {
            var panel = new Panel();
            panel.Location = new Point(0, 0);
            panel.Size = _content.Size;
            panel.BackColor = PaneTheme.PaneBg;
            panel.Visible = false;
            _content.Controls.Add(panel);
            _panels.Add(panel);
            return panel;
        }

        private void ShowStep(int index)
        {
            if (index < 0 || index >= _panels.Count)
            {
                return;
            }

            _step = index;
            for (int i = 0; i < _panels.Count; i++)
            {
                _panels[i].Visible = i == index;
            }

            SetupStepInfo step = SetupCatalog.Steps[index];
            string title = step.Title;
            if (Configured() && index == 1)
            {
                title = "Kiểm tra Axiom Office";
            }

            _titleLabel.Text = title;
            _subtitleLabel.Text = step.Subtitle;
            _stepLabel.Text = StepsDots(index);

            _back.Visible = index > 0 && index < 4;
            _later.Visible = index < 4;
            _later.Text = index == 0 ? "Để sau" : "Đóng";
            _next.Text = index == 4 ? "Hoàn tất" : "Tiếp tục →";

            if (index == 1)
            {
                RefreshChecks();
            }

            if (index == 2)
            {
                _apiKey.Text = Config.LlmApiKey;
            }

            if (index == 4)
            {
                _summary.Text = SummaryText();
            }
        }

        private static string StepsDots(int index)
        {
            string text = "";
            for (int i = 0; i < 5; i++)
            {
                text += i == index ? "●" : "○";
            }

            return text + "   bước " + (index + 1) + "/5";
        }

        private void OnNext()
        {
            if (_step == 2 && !_saved)
            {
                if (!SaveConnection(showMessage: true))
                {
                    return;
                }
            }

            if (_step == 3)
            {
                SaveFeatures(showMessage: true);
            }

            if (_step == 4)
            {
                Close();
                return;
            }

            ShowStep(_step + 1);
        }

        // ---------- B1: chao mung ----------

        private void BuildWelcome()
        {
            Panel panel = NewStep();
            var body = new Label
            {
                AutoSize = false,
                Location = new Point(24, 6),
                Size = new Size(Width0 - 48, 210),
                Font = PaneTheme.Body,
                ForeColor = PaneTheme.TextSecondary
            };

            string text = "AI đọc và sửa trực tiếp tài liệu đang mở trong Word, Excel hoặc PowerPoint.\r\n\r\n"
                + "Axiom Office sẽ hỏi bạn ba điều:\r\n"
                + "   1.  Kiểm tra máy đã sẵn sàng chưa\r\n"
                + "   2.  Kết nối tới máy chủ AI (của công ty hoặc tài khoản riêng)\r\n"
                + "   3.  Chọn những gì AI được làm thêm\r\n\r\n"
                + "Mất khoảng một phút. Có thể mở lại mục \"Thiết lập…\" bất cứ lúc nào.";

            if (_info != null && _info.Version.Length > 0)
            {
                text += "\r\n\r\nPhiên bản: " + _info.Version;
            }
            else if (!string.IsNullOrEmpty(_catalogProblem))
            {
                text += "\r\n\r\n(Agent Core chưa chạy — dùng bản thiết lập có sẵn trong ứng dụng.)";
            }

            body.Text = text;
            panel.Controls.Add(body);
        }

        // ---------- B2: kiem tra may ----------

        private void BuildChecks()
        {
            Panel panel = NewStep();
            _checks.Location = new Point(24, 0);
            _checks.Size = new Size(Width0 - 48, 260);
            _checks.BackColor = PaneTheme.PaneBg;

            var again = new ChipButton("Kiểm tra lại");
            again.Location = new Point(24, 268);
            again.Size = new Size(120, 30);
            again.Click += delegate
            {
                CoreClient.Instance.RestartCore(out _);
                RefreshChecks();
            };

            panel.Controls.AddRange(new Control[] { _checks, again });
        }

        private void RefreshChecks()
        {
            _checks.Controls.Clear();
            _checkLines.Clear();

            int y = 0;
            var rows = new List<CheckRow>();
            rows.Add(CoreRow());
            rows.Add(ConfigRow());
            rows.Add(TokenRow());
            rows.AddRange(RowsFromProblems());

            foreach (CheckRow row in rows)
            {
                var icon = new Label
                {
                    AutoSize = false,
                    Location = new Point(0, y + 2),
                    Size = new Size(24, 20),
                    Font = PaneTheme.Body,
                    ForeColor = row.Danger ? PaneTheme.Danger : (row.Warn ? PaneTheme.TextMuted : PaneTheme.Success),
                    Text = row.Icon
                };

                var text = new Label
                {
                    AutoSize = false,
                    Location = new Point(26, y),
                    Size = new Size(Width0 - 48 - 26 - 130, 40),
                    Font = PaneTheme.Small,
                    ForeColor = PaneTheme.TextPrimary,
                    Text = row.Title + "\r\n" + row.Detail
                };

                _checks.Controls.Add(icon);
                _checks.Controls.Add(text);
                _checkLines.Add(text);

                if (row.Fixable)
                {
                    string label = row.FixLabel;
                    ChipButton fix = new ChipButton(label);
                    fix.Location = new Point(Width0 - 48 - 120, y + 4);
                    fix.Size = new Size(120, 28);
                    string fixId = row.Id;
                    fix.Click += delegate { RunFix(fixId); };
                    _checks.Controls.Add(fix);
                }

                y += 48;
            }
        }

        private sealed class CheckRow
        {
            public string Id;
            public string Icon;
            public string Title;
            public string Detail;
            public bool Danger;
            public bool Warn;
            public bool Fixable;
            public string FixLabel;
        }

        private CheckRow CoreRow()
        {
            string error = null;
            SetupInfo info = _info ?? CoreClient.Instance.Setup(out error);
            bool running = info != null;
            return new CheckRow
            {
                Id = "core",
                Icon = running ? "✓" : "✗",
                Title = running ? "Agent Core đang chạy" : "Agent Core chưa chạy",
                Detail = running
                    ? "Bộ não chạy nền của Axiom Office (cổng " + SetupInfo.Text("port", info.Core, "?") + ")."
                    : (string.IsNullOrEmpty(error) ? "Bấm \"Khởi động Core\" để chạy lại." : error),
                Danger = !running,
                Fixable = !running,
                FixLabel = "Khởi động Core"
            };
        }

        private CheckRow ConfigRow()
        {
            bool ready = !string.IsNullOrEmpty(Config.LlmEndpoint) && !string.IsNullOrEmpty(Config.LlmModel);
            return new CheckRow
            {
                Id = "config",
                Icon = ready ? "✓" : "!",
                Title = ready ? "Đã có cấu hình AI" : "Chưa cấu hình AI",
                Detail = ready
                    ? Config.LlmModel + " tại " + Config.LlmEndpoint
                    : "Cần địa chỉ máy chủ AI và tên model.",
                Warn = !ready,
                Fixable = !ready,
                FixLabel = "Thiết lập ngay"
            };
        }

        private CheckRow TokenRow()
        {
            bool ready = !string.IsNullOrEmpty(Config.Token);
            return new CheckRow
            {
                Id = "token",
                Icon = ready ? "✓" : "!",
                Title = ready ? "Đã có khoá bảo vệ" : "Chưa có khoá bảo vệ",
                Detail = ready
                    ? "Khoá chỉ đi trên máy này, để ứng dụng khác không điều khiển được Agent Core."
                    : "Nên tạo khoá mới để Agent Core không bị ứng dụng khác điều khiển.",
                Warn = !ready,
                Fixable = !ready,
                FixLabel = "Tạo khoá mới"
            };
        }

        private List<CheckRow> RowsFromProblems()
        {
            var rows = new List<CheckRow>();
            if (_info == null)
            {
                return rows;
            }

            foreach (Dictionary<string, object> problem in _info.Problems)
            {
                string id = SetupInfo.Text("id", problem, "");
                if (id == "core" || id == "config" || id == "token")
                {
                    continue;   // da hien o cac dong tren
                }

                SetupCheckInfo meta = SetupCatalog.FindCheck(id);
                rows.Add(new CheckRow
                {
                    Id = id,
                    Icon = SetupInfo.Text("severity", problem, "warning") == "error" ? "✗" : "!",
                    Title = SetupInfo.Text("label", problem, id),
                    Detail = SetupInfo.Text("hint", problem, ""),
                    Danger = SetupInfo.Text("severity", problem, "warning") == "error",
                    Warn = true,
                    Fixable = false,
                    FixLabel = meta != null ? meta.FixLabel : ""
                });
            }

            return rows;
        }

        private void RunFix(string id)
        {
            if (id == "core")
            {
                string error;
                CoreClient.Instance.RestartCore(out error);
                RefreshChecks();
                return;
            }

            if (id == "config")
            {
                ShowStep(2);
                return;
            }

            if (id == "token")
            {
                CoreClient.GenerateToken();
                string error;
                CoreClient.Instance.RestartCore(out error);
                RefreshChecks();
                return;
            }

            MessageBox.Show(this,
                "Dòng này cần sửa bằng tay, xem hướng dẫn trong tệp cài đặt.",
                "Thiết lập Axiom Office", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }

        // ---------- B3: ket noi may chu AI ----------

        private void BuildConnect()
        {
            Panel panel = NewStep();
            int y = 0;

            panel.Controls.Add(Label2("AI của bạn nằm ở đâu?", 24, y, 400, true));

            y += 26;
            foreach (SetupProviderInfo provider in SetupCatalog.Providers)
            {
                var chip = new ChipButton(provider.Label);
                chip.Location = new Point(24, y);
                chip.Size = new Size(140, 30);
                string id = provider.Id;
                chip.Click += delegate { SelectProvider(id); };
                _providerChips.Add(chip);
                panel.Controls.Add(chip);
                y += 36;
            }

            y += 6;
            panel.Controls.Add(Label2("Địa chỉ máy chủ", 24, y, 300, false));
            y += 20;
            _endpoint.Location = new Point(24, y);
            _endpoint.Size = new Size(Width0 - 48, 26);
            _endpoint.Font = PaneTheme.Small;
            panel.Controls.Add(_endpoint);

            y += 34;
            panel.Controls.Add(Label2("Khoá truy cập", 24, y, 300, false));
            _keyLink.AutoSize = true;
            _keyLink.Location = new Point(140, y + 2);
            _keyLink.Font = PaneTheme.Caption;
            _keyLink.ForeColor = PaneTheme.AccentFg;
            _keyLink.Cursor = Cursors.Hand;
            _keyLink.Click += delegate { OpenKeyPage(); };
            panel.Controls.Add(_keyLink);

            y += 20;
            _apiKey.Location = new Point(24, y);
            _apiKey.Size = new Size(Width0 - 48 - 130, 26);
            _apiKey.Font = PaneTheme.Small;
            _apiKey.UseSystemPasswordChar = true;
            panel.Controls.Add(_apiKey);

            _showKey.Location = new Point(Width0 - 48 - 120, y - 2);
            _showKey.Size = new Size(120, 30);
            _showKey.Click += delegate
            {
                _apiKey.UseSystemPasswordChar = !_apiKey.UseSystemPasswordChar;
                _showKey.Text = _apiKey.UseSystemPasswordChar ? "Xem khoá" : "Ẩn khoá";
            };
            panel.Controls.Add(_showKey);

            y += 36;
            panel.Controls.Add(Label2("Model", 24, y, 300, false));
            y += 20;
            _model.Location = new Point(24, y);
            _model.Size = new Size(Width0 - 48 - 160, 26);
            _model.Font = PaneTheme.Small;
            _model.DropDownStyle = ComboBoxStyle.DropDown;
            panel.Controls.Add(_model);

            _loadModels.Location = new Point(Width0 - 48 - 150, y - 2);
            _loadModels.Size = new Size(150, 30);
            _loadModels.Click += delegate { LoadModels(); };
            panel.Controls.Add(_loadModels);

            y += 40;
            _test.Location = new Point(24, y);
            _test.Size = new Size(160, 32);
            _test.Click += delegate { TestConnection(); };
            panel.Controls.Add(_test);

            _probe.AutoSize = false;
            _probe.Location = new Point(196, y - 2);
            _probe.Size = new Size(Width0 - 48 - 180, 60);
            _probe.Font = PaneTheme.Small;
            _probe.ForeColor = PaneTheme.TextSecondary;
            panel.Controls.Add(_probe);

            SelectProvider(_info != null && _info.ProviderId.Length > 0 ? _info.ProviderId : "company");
        }

        private static Label Label2(string text, int x, int y, int width, bool strong)
        {
            return new Label
            {
                AutoSize = false,
                Location = new Point(x, y),
                Size = new Size(width, 20),
                Text = text,
                Font = strong ? PaneTheme.SmallBold : PaneTheme.Small,
                ForeColor = PaneTheme.TextSecondary
            };
        }

        private void SelectProvider(string id)
        {
            SetupProviderInfo provider = SetupCatalog.FindProvider(id) ?? SetupCatalog.Providers[0];
            _providerId = provider.Id;

            foreach (ChipButton chip in _providerChips)
            {
                chip.Enabled = true;
            }

            for (int i = 0; i < _providerChips.Count && i < SetupCatalog.Providers.Count; i++)
            {
                if (SetupCatalog.Providers[i].Id == provider.Id)
                {
                    _providerChips[i].Text = "● " + provider.Label;
                }
                else
                {
                    _providerChips[i].Text = SetupCatalog.Providers[i].Label;
                }
            }

            _endpoint.Text = provider.Endpoint.Length > 0 ? provider.Endpoint : ConfigEndpointFallback();
            _keyLink.Text = provider.KeyUrl.Length > 0 ? "Lấy khoá ở đâu?" : "";
            _apiKey.Enabled = true;

            _model.Items.Clear();
            foreach (string suggested in provider.SuggestedModels)
            {
                _model.Items.Add(suggested);
            }

            string current = Config.LlmModel;
            if (current.Length > 0 && _info != null && _info.ProviderId == provider.Id)
            {
                _model.Text = current;
            }
            else if (_model.Items.Count > 0)
            {
                _model.SelectedIndex = 0;
            }

            _probe.Text = provider.Description;
            _probe.ForeColor = PaneTheme.TextMuted;
        }

        // Nha cung cap "may chu cua cong ty" khong dien san dia chi: giu gia tri dang dung neu co.
        private string ConfigEndpointFallback()
        {
            return Config.LlmEndpoint;
        }

        private void OpenKeyPage()
        {
            SetupProviderInfo provider = SetupCatalog.FindProvider(_providerId);
            if (provider == null || provider.KeyUrl.Length == 0)
            {
                return;
            }

            try
            {
                System.Diagnostics.Process.Start(new System.Diagnostics.ProcessStartInfo(provider.KeyUrl)
                {
                    UseShellExecute = true
                });
            }
            catch (Exception ex)
            {
                MessageBox.Show(this, "Không mở được trình duyệt: " + ex.Message, "Thiết lập Axiom Office",
                    MessageBoxButtons.OK, MessageBoxIcon.Warning);
            }
        }

        private void LoadModels()
        {
            SetupProviderInfo provider = SetupCatalog.FindProvider(_providerId);
            string codec = provider != null ? provider.Codec : "openai";
            _loadModels.Enabled = false;
            _probe.ForeColor = PaneTheme.TextMuted;
            _probe.Text = "Đang tải danh sách model…";

            string endpoint = _endpoint.Text.Trim();
            string key = _apiKey.Text.Trim();
            var form = this;
            Task.Factory.StartNew(delegate
            {
                string error;
                string hint;
                List<string> models = CoreClient.Instance.ListModels(codec, endpoint, key, out error, out hint);
                form.BeginInvoke((MethodInvoker)delegate
                {
                    _loadModels.Enabled = true;
                    if (models == null)
                    {
                        _probe.ForeColor = PaneTheme.Danger;
                        _probe.Text = "✗ " + (error ?? "Không lấy được danh sách model.")
                            + (string.IsNullOrEmpty(hint) ? "" : "\r\n" + hint)
                            + "\r\nVẫn có thể tự gõ tên model ở ô trên.";
                        return;
                    }

                    string chosen = _model.Text;
                    _model.Items.Clear();
                    foreach (string name in models)
                    {
                        _model.Items.Add(name);
                    }

                    _probe.ForeColor = PaneTheme.TextSecondary;
                    _probe.Text = "Đã tải " + models.Count + " model. Chọn một model ở ô trên.";
                    if (!string.IsNullOrEmpty(chosen))
                    {
                        _model.Text = chosen;
                    }
                    else if (_model.Items.Count > 0)
                    {
                        _model.SelectedIndex = 0;
                    }
                });
            });
        }

        private void TestConnection()
        {
            SetupProviderInfo provider = SetupCatalog.FindProvider(_providerId);
            string codec = provider != null ? provider.Codec : "openai";
            string endpoint = _endpoint.Text.Trim();
            string model = _model.Text.Trim();
            string key = _apiKey.Text.Trim();

            if (endpoint.Length == 0 || model.Length == 0)
            {
                _probe.ForeColor = PaneTheme.Danger;
                _probe.Text = "✗ Điền đủ Địa chỉ máy chủ và Model rồi thử lại.";
                return;
            }

            _test.Enabled = false;
            _probe.ForeColor = PaneTheme.TextMuted;
            _probe.Text = "Đang kiểm tra kết nối…";

            var form = this;
            Task.Factory.StartNew(delegate
            {
                LlmProbe probe = CoreClient.Instance.TestLlm(_providerId, codec, endpoint, model, key, CancellationToken.None);
                form.BeginInvoke((MethodInvoker)delegate
                {
                    _test.Enabled = true;
                    if (probe.Ok)
                    {
                        _probe.ForeColor = PaneTheme.Success;
                        _probe.Text = "✓ Kết nối tốt (" + probe.Seconds.ToString("0.0") + " giây) — máy chủ trả lời \""
                            + Shorten(probe.Reply, 40) + "\".";
                        return;
                    }

                    _probe.ForeColor = PaneTheme.Danger;
                    _probe.Text = "✗ " + probe.Message + (string.IsNullOrEmpty(probe.Hint) ? "" : "\r\n" + probe.Hint);
                });
            });
        }

        private static string Shorten(string value, int max)
        {
            if (string.IsNullOrEmpty(value))
            {
                return "";
            }

            string text = value.Replace("\r", " ").Replace("\n", " ").Trim();
            return text.Length <= max ? text : text.Substring(0, max) + "…";
        }

        // Luu cau hinh AI (nhu SettingsForm, dung Config de khong sinh nguon su that thu hai).
        private bool SaveConnection(bool showMessage)
        {
            string endpoint = _endpoint.Text.Trim();
            string model = _model.Text.Trim();
            if (endpoint.Length == 0 || model.Length == 0)
            {
                if (showMessage)
                {
                    MessageBox.Show(this, "Điền đủ Địa chỉ máy chủ và Model rồi tiếp tục.", "Thiết lập Axiom Office",
                        MessageBoxButtons.OK, MessageBoxIcon.Warning);
                }

                return false;
            }

            SetupProviderInfo provider = SetupCatalog.FindProvider(_providerId);
            string codec = provider != null ? provider.Codec : "openai";
            bool ok = Config.WriteString("LlmProvider", codec)
                && Config.WriteString("LlmEndpoint", endpoint)
                && Config.WriteString("LlmModel", model);

            string key = _apiKey.Text.Trim();
            if (key.Length > 0 && key != Config.LlmApiKey)
            {
                ok = Config.WriteSecret("LlmApiKey", key) && ok;
            }

            if (!ok)
            {
                MessageBox.Show(this, "Không ghi được cấu hình (kiểm tra quyền với HKCU).", "Thiết lập Axiom Office",
                    MessageBoxButtons.OK, MessageBoxIcon.Error);
                return false;
            }

            _saved = true;
            Logger.Info("SetupWizard: da luu cau hinh AI provider=" + codec + " model=" + model);
            return true;
        }

        // ---------- B4: tinh nang ----------

        private void BuildFeatures()
        {
            Panel panel = NewStep();
            int y = 0;
            foreach (SetupFeatureInfo feature in SetupCatalog.Features)
            {
                CheckBox box = FeatureBox(feature.Key);
                box.Location = new Point(24, y);
                box.Size = new Size(Width0 - 48, 24);
                box.Text = feature.Label;
                box.Font = PaneTheme.SmallBold;
                box.ForeColor = PaneTheme.TextPrimary;
                box.Checked = FeatureValue(feature.Key);

                var hint = new Label
                {
                    AutoSize = false,
                    Location = new Point(42, y + 22),
                    Size = new Size(Width0 - 70, 34),
                    Font = PaneTheme.Caption,
                    ForeColor = PaneTheme.TextMuted,
                    Text = feature.Description
                };

                panel.Controls.Add(box);
                panel.Controls.Add(hint);
                y += 62;
            }

            _advanced.Location = new Point(24, y + 6);
            _advanced.Size = new Size(180, 30);
            _advanced.Click += delegate
            {
                using (var form = new SettingsForm())
                {
                    form.ShowDialog(this);
                }

                _memory.Checked = Config.MemoryEnabled;
                _autoExtract.Checked = Config.MemoryAutoExtract;
                _visualQa.Checked = Config.VisualQaEnabled;
            };
            panel.Controls.Add(_advanced);

            var note = new Label
            {
                AutoSize = false,
                Location = new Point(214, y + 10),
                Size = new Size(Width0 - 48 - 200, 40),
                Font = PaneTheme.Caption,
                ForeColor = PaneTheme.TextMuted,
                Text = "Cổng của Agent Core, hạn giờ, model ghi nhớ… nằm trong Tuỳ chọn nâng cao."
            };
            panel.Controls.Add(note);
        }

        private CheckBox FeatureBox(string key)
        {
            if (key == "MemoryEnabled")
            {
                return _memory;
            }

            if (key == "MemoryAutoExtract")
            {
                return _autoExtract;
            }

            return _visualQa;
        }

        private bool FeatureValue(string key)
        {
            if (key == "MemoryEnabled")
            {
                return Config.MemoryEnabled;
            }

            if (key == "MemoryAutoExtract")
            {
                return Config.MemoryAutoExtract;
            }

            return Config.VisualQaEnabled;
        }

        private void SaveFeatures(bool showMessage)
        {
            bool ok = Config.WriteDword("MemoryEnabled", _memory.Checked ? 1 : 0)
                & Config.WriteDword("MemoryAutoExtract", _autoExtract.Checked ? 1 : 0)
                & Config.WriteDword("VisualQaEnabled", _visualQa.Checked ? 1 : 0);
            if (!ok && showMessage)
            {
                MessageBox.Show(this, "Không ghi được tuỳ chọn tính năng.", "Thiết lập Axiom Office",
                    MessageBoxButtons.OK, MessageBoxIcon.Error);
            }

            Logger.Info("SetupWizard: tinh nang memory=" + _memory.Checked + " autoExtract=" + _autoExtract.Checked
                + " visualQa=" + _visualQa.Checked);
        }

        // ---------- B5: hoan tat ----------

        private void BuildDone()
        {
            Panel panel = NewStep();
            _summary.AutoSize = false;
            _summary.Location = new Point(24, 0);
            _summary.Size = new Size(Width0 - 48, 90);
            _summary.Font = PaneTheme.Body;
            _summary.ForeColor = PaneTheme.TextPrimary;
            panel.Controls.Add(_summary);

            _tryNow.Location = new Point(24, 96);
            _tryNow.Size = new Size(260, 32);
            _tryNow.Click += delegate { TryNow(); };
            panel.Controls.Add(_tryNow);

            _tryResult.AutoSize = false;
            _tryResult.Location = new Point(24, 136);
            _tryResult.Size = new Size(Width0 - 48, 70);
            _tryResult.Font = PaneTheme.Small;
            _tryResult.ForeColor = PaneTheme.TextSecondary;
            panel.Controls.Add(_tryResult);

            var hint = new Label
            {
                AutoSize = false,
                Location = new Point(24, 212),
                Size = new Size(Width0 - 48, 60),
                Font = PaneTheme.Caption,
                ForeColor = PaneTheme.TextMuted,
                Text = "Mở lại \"Ask AI\" trong thẻ Axiom Office để bắt đầu. Muốn Claude Code/Desktop điều khiển "
                    + "Word/LibreOffice qua MCP thì lấy cấu hình trong Cài đặt nâng cao."
            };
            panel.Controls.Add(hint);
        }

        private string SummaryText()
        {
            string model = _model.Text.Trim();
            string endpoint = _endpoint.Text.Trim();
            string memory = _memory.Checked ? "bật" : "tắt";
            string core = _info != null ? "Agent Core: cổng " + SetupInfo.Text("port", _info.Core, "?") : "Agent Core: chưa chạy";
            return "AI:  " + (model.Length > 0 ? model : "(chưa chọn)") + " · " + (endpoint.Length > 0 ? endpoint : "(chưa có địa chỉ)")
                + "\r\nGhi nhớ dài hạn: " + memory + " · " + core
                + "\r\n\r\nBấm \"Thử ngay\" để Axiom Office viết một câu ngắn vào tài liệu đang mở — "
                + "cách chắc chắn nhất để biết mọi thứ đã chạy.";
        }

        private void TryNow()
        {
            if (_host == null)
            {
                _tryResult.ForeColor = PaneTheme.Danger;
                _tryResult.Text = "Không có tài liệu đang mở để thử. Mở một tài liệu rồi bấm Thử ngay.";
                return;
            }

            _tryNow.Enabled = false;
            _tryResult.ForeColor = PaneTheme.TextMuted;
            _tryResult.Text = "Đang chạy thử…";

            var form = this;
            Task.Factory.StartNew(delegate
            {
                string reply = null;
                string error = null;
                try
                {
                    // Chung duong voi pane (AskAiPane.RunViaCore): Core chay agent that tren tai lieu dang mo.
                    // Khong chay lai in-process o day: buoc nay de CHUNG MINH chuoi Core -> bridge -> tai lieu chay duoc.
                    LlmResult result = CoreClient.Instance.Run(_host, "Viết một câu chào ngắn vào đầu tài liệu.",
                        null, null, CancellationToken.None);
                    if (result != null && result.Ok)
                    {
                        reply = result.Text;
                    }
                    else if (result != null)
                    {
                        error = result.Error;
                    }
                    else
                    {
                        error = CoreClient.Instance.LastError;
                    }
                }
                catch (Exception ex)
                {
                    error = ex.GetType().Name + ": " + ex.Message;
                }

                form.BeginInvoke((MethodInvoker)delegate
                {
                    _tryNow.Enabled = true;
                    if (reply != null)
                    {
                        _tryResult.ForeColor = PaneTheme.Success;
                        _tryResult.Text = "✓ Xong: " + Shorten(reply, 160);
                        return;
                    }

                    _tryResult.ForeColor = PaneTheme.Danger;
                    _tryResult.Text = "✗ Chưa chạy được: " + (error ?? "không rõ lỗi")
                        + "\r\nKiểm tra lại bước \"Kết nối máy chủ AI\".";
                });
            });
        }
    }
}

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
        private readonly StepBar _stepBar = new StepBar();
        private readonly Label _stepLabel = new Label();
        private readonly Label _titleLabel = new Label();
        private readonly Label _subtitleLabel = new Label();
        private readonly Panel _footer = new Panel();
        private readonly Panel _footerDivider = new Panel();
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
        private readonly CardBox _probeCard = new CardBox();
        private readonly FieldBox _endpointBox;
        private readonly FieldBox _apiKeyBox;
        private readonly FieldBox _modelBox;
        private readonly Label _probe = new Label();
        private readonly ChipButton _test = new ChipButton("Kiểm tra kết nối");
        private readonly ChipButton _loadModels = new ChipButton("Tải danh sách model");
        private readonly ChipButton _showKey = new ChipButton("Xem khoá");
        private bool _saved;

        // Buoc 2: danh sach kiem tra (nhan + ket qua + nut Sua).
        private readonly Panel _checks = new Panel();
        private readonly List<Label> _checkLines = new List<Label>();

        // Buoc 4: tinh nang.
        private readonly ToggleBox _memory = new ToggleBox();
        private readonly ToggleBox _autoExtract = new ToggleBox();
        private readonly ToggleBox _verify = new ToggleBox();
        private readonly ToggleBox _reasoning = new ToggleBox();
        private readonly ToggleBox _visualQa = new ToggleBox();
        private readonly ChipButton _advanced = new ChipButton("Tuỳ chọn nâng cao…");
        private readonly ChipButton _again = new ChipButton("Kiểm tra lại");

        // Buoc 5: thu ngay.
        private readonly Panel _summaryPanel = new Panel();
        private readonly CardBox _tryResultCard = new CardBox();
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

            // Ba ô nhập dùng chung kiểu "khung bo góc + TextBox phẳng" của Axiom thay vì viền vuông của WinForms.
            _endpoint.Font = PaneTheme.Small;
            _apiKey.Font = PaneTheme.Small;
            _apiKey.UseSystemPasswordChar = true;
            _model.Font = PaneTheme.Small;
            _model.DropDownStyle = ComboBoxStyle.DropDown;
            _endpointBox = new FieldBox(_endpoint);
            _apiKeyBox = new FieldBox(_apiKey);
            _modelBox = new FieldBox(_model);

            BuildChrome();
            BuildWelcome();
            BuildChecks();
            BuildConnect();
            BuildFeatures();
            BuildDone();
            // Luon mo o buoc 1/5 (Chao mung): may da thiet lap thi nguoi dung van bam "Tiep tuc" de sang buoc
            // kiem tra. Truoc day nhay thang buoc 2/5 khien nguoi dung khong ro vi sao minh o do.
            ShowStep(0);
        }

        // Da co endpoint + model (dung de doi tieu de buoc kiem tra khi may da thiet lap).
        private bool Configured()
        {
            return !string.IsNullOrEmpty(Config.LlmEndpoint) && !string.IsNullOrEmpty(Config.LlmModel);
        }

        // ---------- khung ----------

        private void BuildChrome()
        {
            // Chi bao buoc: thanh 5 doan (de thay) + chu "bước N/5" (de doc) - thay cho cham ●○○○○ o co 8.25pt.
            _stepBar.Location = new Point(24, 20);
            _stepLabel.AutoSize = false;
            _stepLabel.Location = new Point(24 + _stepBar.Width + 12, 16);
            _stepLabel.Size = new Size(200, 18);
            _stepLabel.Font = PaneTheme.Small;
            _stepLabel.ForeColor = PaneTheme.TextMuted;

            _titleLabel.AutoSize = false;
            _titleLabel.Location = new Point(24, 40);
            _titleLabel.Size = new Size(Width0 - 48, 32);
            _titleLabel.Font = PaneTheme.DialogTitle;
            _titleLabel.ForeColor = PaneTheme.TextPrimary;

            _subtitleLabel.AutoSize = false;
            _subtitleLabel.Location = new Point(24, 76);
            _subtitleLabel.Size = new Size(Width0 - 48, 40);
            _subtitleLabel.Font = PaneTheme.DialogSubtitle;
            _subtitleLabel.ForeColor = PaneTheme.TextSecondary;

            _content.Location = new Point(0, 124);
            _content.Size = new Size(Width0, Height0 - 124 - 64);
            _content.BackColor = PaneTheme.PaneBg;

            _footerDivider.Location = new Point(0, Height0 - 64);
            _footerDivider.Size = new Size(Width0, 1);
            _footerDivider.BackColor = PaneTheme.Divider;

            _footer.Location = new Point(0, Height0 - 63);
            _footer.Size = new Size(Width0, 63);
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
            Controls.AddRange(new Control[] { _stepBar, _stepLabel, _titleLabel, _subtitleLabel, _content,
                _footerDivider, _footer });
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
            _stepBar.Step = index;
            _stepLabel.Text = "bước " + (index + 1) + "/5";

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
                RefreshSummary();
            }
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
            int y = 0;

            panel.Controls.Add(new Label
            {
                AutoSize = false,
                Location = new Point(24, y),
                Size = new Size(Width0 - 48, 46),
                Font = PaneTheme.DialogSubtitle,
                ForeColor = PaneTheme.TextSecondary,
                Text = "AI đọc và sửa trực tiếp tài liệu đang mở trong Word, Excel hoặc PowerPoint. "
                    + "Axiom Office sẽ hỏi bạn ba việc:"
            });
            y += 56;

            // Ba muc danh so bang huy hieu tron: truoc day la mot doan chu voi so thu tu go tay nen cac dong le
            // nhau, dong thu ba khong thang hang voi hai dong dau.
            string[] items =
            {
                "Kiểm tra máy đã sẵn sàng chưa",
                "Kết nối tới máy chủ AI (máy chủ công ty hoặc tài khoản riêng)",
                "Chọn những gì AI được làm thêm",
            };
            for (int index = 0; index < items.Length; index++)
            {
                var badge = new StatusBadge();
                badge.SetGlyph((index + 1).ToString(), PaneTheme.ActionBg);
                badge.Location = new Point(24, y);

                panel.Controls.Add(badge);
                panel.Controls.Add(new Label
                {
                    AutoSize = false,
                    Location = new Point(24 + 30, y - 2),
                    Size = new Size(Width0 - 48 - 30, 24),
                    Font = PaneTheme.Body,
                    ForeColor = PaneTheme.TextPrimary,
                    Text = items[index]
                });
                y += 32;
            }

            y += 10;
            var note = new Label
            {
                AutoSize = false,
                Location = new Point(24, y),
                Size = new Size(Width0 - 48, 46),
                Font = PaneTheme.Small,
                ForeColor = PaneTheme.TextMuted,
                Text = "Mở lại mục \"Thiết lập…\" trong thẻ Axiom Office bất cứ lúc nào."
            };
            if (_info != null && _info.Version.Length > 0)
            {
                note.Text += "  ·  Phiên bản " + _info.Version;
            }
            else if (!string.IsNullOrEmpty(_catalogProblem))
            {
                note.Text += "\r\n(Chưa đọc được Agent Core — dùng bản thiết lập có sẵn trong ứng dụng.)";
            }

            panel.Controls.Add(note);
        }

        // ---------- B2: kiem tra may ----------

        private void BuildChecks()
        {
            Panel panel = NewStep();
            _checks.Location = new Point(24, 0);
            _checks.Size = new Size(Width0 - 48, 10);
            _checks.BackColor = PaneTheme.PaneBg;

            _again.Location = new Point(24, 0);          // dat lai sau khi ve xong danh sach (xem RefreshChecks)
            _again.Size = new Size(130, 30);
            _again.Click += delegate
            {
                CoreClient.Instance.RestartCore(out _);
                RefreshChecks();
            };

            panel.Controls.AddRange(new Control[] { _checks, _again });
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
                // Moi dong la mot the bo goc: huy hieu tron + tieu de dam + chi tiet mo + nut Sua trong the. Truoc
                // day la ba control roi tren nen trang nen "cai gi sai" va "sua the nao" khong gan voi nhau.
                var card = new CardBox
                {
                    Location = new Point(0, y),
                    Size = new Size(Width0 - 48, 54),
                    Tone = row.Danger ? "danger" : row.Warn ? "warn" : "success"
                };

                var badge = new StatusBadge { Location = new Point(14, 17) };
                badge.Set(!row.Danger && !row.Warn, row.Warn, row.Danger);

                var title = new Label
                {
                    AutoSize = false,
                    Location = new Point(44, 9),
                    Size = new Size(Width0 - 48 - 44 - 14 - (row.Fixable ? 140 : 0), 20),
                    Font = PaneTheme.SmallBold,
                    ForeColor = PaneTheme.TextPrimary,
                    Text = row.Title
                };

                var detail = new Label
                {
                    AutoSize = false,
                    Location = new Point(44, 28),
                    Size = new Size(Width0 - 48 - 44 - 14 - (row.Fixable ? 140 : 0), 20),
                    Font = PaneTheme.Small,
                    ForeColor = PaneTheme.TextMuted,
                    Text = row.Detail
                };

                card.Controls.Add(badge);
                card.Controls.Add(title);
                card.Controls.Add(detail);
                _checkLines.Add(detail);

                if (row.Fixable)
                {
                    ChipButton fix = new ChipButton(row.FixLabel);
                    fix.Location = new Point(card.Width - 130, 12);
                    fix.Size = new Size(116, 30);
                    string fixId = row.Id;
                    fix.Click += delegate { RunFix(fixId); };
                    card.Controls.Add(fix);
                }

                _checks.Controls.Add(card);
                y += 62;
            }

            _checks.Height = Math.Max(10, y - 8);
            _again.Location = new Point(0, _checks.Height + 12);
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
                // Dong chi tiet phai la cau nguoi dung hieu duoc: truoc day in nguyen van loi ky thuat cua Core
                // (vd "khong thay"), khong noi len duoc phai lam gi. Loi goc van nam trong core.log.
                Detail = running
                    ? "Bộ não chạy nền của Axiom Office (cổng " + SetupInfo.Text("port", info.Core, "?") + ")."
                    : "Bấm \"Khởi động Core\" để Axiom Office tự chạy lại (chi tiết kỹ thuật ở core.log).",
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

            // Nha cung cap: luoi 2 cot, be rong theo chu (truoc day chip co dinh 140px nen "May chu cua cong ty"
            // bi cat thanh "May chu cua co..." - nguoi dung khong doc duoc lua chon cua chinh minh).
            panel.Controls.Add(FieldLabel("AI của bạn nằm ở đâu?", 24, y));
            y += 26;

            // O rong bang nhau theo cot (luoi 2x3 gon gan, khong con chip ngang ngan khac nhau). Nhan dai nhat
            // "May chu cua cong ty" ~170px nen 292px la du, khong bao gio bi cat.
            int columnWidth = (Width0 - 48 - 8) / 2;
            for (int index = 0; index < SetupCatalog.Providers.Count; index++)
            {
                SetupProviderInfo provider = SetupCatalog.Providers[index];
                var chip = new ChipButton(provider.Label);
                chip.Location = new Point(24 + (index % 2) * (columnWidth + 8), y + (index / 2) * 36);
                chip.Size = new Size(columnWidth, 30);
                string id = provider.Id;
                chip.Click += delegate { SelectProvider(id); };
                _providerChips.Add(chip);
                panel.Controls.Add(chip);
            }

            y = 26 + ((SetupCatalog.Providers.Count + 1) / 2) * 36 + 8;
            panel.Controls.Add(FieldLabel("Địa chỉ máy chủ", 24, y));
            y += 20;
            _endpointBox.Location = new Point(24, y);
            _endpointBox.Size = new Size(Width0 - 48, 30);
            panel.Controls.Add(_endpointBox);

            y += 38;
            panel.Controls.Add(FieldLabel("Khoá truy cập", 24, y));
            _keyLink.AutoSize = true;
            _keyLink.Location = new Point(24 + TextRenderer.MeasureText("Khoá truy cập", PaneTheme.FieldLabel).Width + 10, y + 1);
            _keyLink.Font = PaneTheme.Small;
            _keyLink.ForeColor = PaneTheme.AccentFg;
            _keyLink.Cursor = Cursors.Hand;
            _keyLink.Click += delegate { OpenKeyPage(); };
            panel.Controls.Add(_keyLink);

            y += 20;
            _apiKeyBox.Location = new Point(24, y);
            _apiKeyBox.Size = new Size(Width0 - 48 - 128, 30);
            panel.Controls.Add(_apiKeyBox);

            _showKey.Location = new Point(24 + Width0 - 48 - 120, y);
            _showKey.Size = new Size(120, 30);
            _showKey.Click += delegate
            {
                _apiKey.UseSystemPasswordChar = !_apiKey.UseSystemPasswordChar;
                _showKey.Text = _apiKey.UseSystemPasswordChar ? "Xem khoá" : "Ẩn khoá";
            };
            panel.Controls.Add(_showKey);

            y += 38;
            panel.Controls.Add(FieldLabel("Model", 24, y));
            y += 20;
            _modelBox.Location = new Point(24, y);
            _modelBox.Size = new Size(Width0 - 48 - 150, 30);
            panel.Controls.Add(_modelBox);

            _loadModels.Location = new Point(24 + Width0 - 48 - 142, y);
            _loadModels.Size = new Size(142, 30);
            _loadModels.Click += delegate { LoadModels(); };
            panel.Controls.Add(_loadModels);

            // Enter o bat ky o nao cung = "Kiem tra ket noi" (truoc day Enter khong lam gi).
            KeyEventHandler submit = delegate (object sender, KeyEventArgs e)
            {
                if (e.KeyCode == Keys.Enter)
                {
                    e.Handled = true;
                    e.SuppressKeyPress = true;
                    TestConnection();
                }
            };
            _endpoint.KeyDown += submit;
            _apiKey.KeyDown += submit;
            _model.KeyDown += submit;

            y += 38;
            _test.Location = new Point(24, y);
            _test.Size = new Size(170, 32);
            _test.Click += delegate { TestConnection(); };
            panel.Controls.Add(_test);

            // Ket qua/thong bao nam trong the ngay duoi cac o nhap (khong con nam canh nut o day khung).
            y += 36;
            _probeCard.Location = new Point(24, y);
            _probeCard.Size = new Size(Width0 - 48, 46);
            _probe.AutoSize = false;
            _probe.Location = new Point(12, 6);
            _probe.Size = new Size(Width0 - 48 - 24, 34);
            // Thong bao 1 dong phai nam giua the, neu khong chu dinh le tren va the trong nhu bi ho.
            _probe.TextAlign = ContentAlignment.MiddleLeft;
            _probe.Font = PaneTheme.Small;
            _probe.ForeColor = PaneTheme.TextSecondary;
            _probeCard.Controls.Add(_probe);
            panel.Controls.Add(_probeCard);

            SelectProvider(_info != null && _info.ProviderId.Length > 0 ? _info.ProviderId : "company");
        }

        private static Label FieldLabel(string text, int x, int y)
        {
            return new Label
            {
                AutoSize = false,
                Location = new Point(x, y),
                Size = new Size(Width0 - 48, 20),
                Text = text,
                Font = PaneTheme.FieldLabel,
                ForeColor = PaneTheme.TextPrimary
            };
        }

        // Trang thai o the ket qua: "neutral" (mo ta nha cung cap), "success", "danger".
        private void SetProbe(string text, string tone)
        {
            _probe.Text = text;
            _probe.ForeColor = tone == "danger" ? PaneTheme.DangerFg
                : tone == "success" ? PaneTheme.Success
                : PaneTheme.TextSecondary;
            _probeCard.Tone = tone;
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

            SetProbe(provider.Description, "neutral");
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
            SetProbe("Đang tải danh sách model…", "neutral");

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
                        SetProbe((error ?? "Không lấy được danh sách model.")
                            + (string.IsNullOrEmpty(hint) ? "" : " " + hint)
                            + " Vẫn có thể tự gõ tên model ở ô trên.", "danger");
                        return;
                    }

                    string chosen = _model.Text;
                    _model.Items.Clear();
                    foreach (string name in models)
                    {
                        _model.Items.Add(name);
                    }

                    SetProbe("Đã tải " + models.Count + " model. Chọn một model ở ô trên.", "neutral");
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
                SetProbe("Điền đủ Địa chỉ máy chủ và Model rồi thử lại.", "danger");
                return;
            }

            _test.Enabled = false;
            SetProbe("Đang kiểm tra kết nối…", "neutral");

            var form = this;
            Task.Factory.StartNew(delegate
            {
                LlmProbe probe = CoreClient.Instance.TestLlm(_providerId, codec, endpoint, model, key, CancellationToken.None);
                form.BeginInvoke((MethodInvoker)delegate
                {
                    _test.Enabled = true;
                    if (probe.Ok)
                    {
                        SetProbe("Kết nối tốt (" + probe.Seconds.ToString("0.0") + " giây) — máy chủ trả lời \""
                            + Shorten(probe.Reply, 40) + "\".", "success");
                        return;
                    }

                    SetProbe(probe.Message + (string.IsNullOrEmpty(probe.Hint) ? "" : " " + probe.Hint), "danger");
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
                // ToggleBox ve tick phang theo bang mau cua Axiom; CheckBox cua WinForms ve o vuong kieu 3D nen
                // dat canh cac nut phang cua pane thi nhu hai san pham ghep lai.
                ToggleBox box = FeatureBox(feature.Key);
                box.Location = new Point(24, y);
                box.Size = new Size(Width0 - 48, 46);
                box.Text = feature.Label;
                box.Hint = feature.Description;
                box.Checked = FeatureValue(feature.Key);
                panel.Controls.Add(box);
                y += 58;
            }

            y += 4;
            _advanced.Location = new Point(24, y);
            _advanced.Size = new Size(190, 32);
            _advanced.Click += delegate
            {
                using (var form = new SettingsForm())
                {
                    form.ShowDialog(this);
                }

                _memory.Checked = Config.MemoryEnabled;
                _autoExtract.Checked = Config.MemoryAutoExtract;
                _verify.Checked = Config.VerifyWorkEnabled;
                _reasoning.Checked = Config.LlmShowReasoning;
                _visualQa.Checked = Config.VisualQaEnabled;
            };
            panel.Controls.Add(_advanced);

            panel.Controls.Add(new Label
            {
                AutoSize = false,
                Location = new Point(24 + 190 + 12, y + 6),
                Size = new Size(Width0 - 48 - 190 - 12, 40),
                Font = PaneTheme.Small,
                ForeColor = PaneTheme.TextMuted,
                Text = "Mở để đổi cổng Agent Core, hạn giờ, model ghi nhớ."
            });
        }

        private ToggleBox FeatureBox(string key)
        {
            if (key == "MemoryEnabled")
            {
                return _memory;
            }

            if (key == "MemoryAutoExtract")
            {
                return _autoExtract;
            }

            if (key == "VerifyWorkEnabled")
            {
                return _verify;
            }

            if (key == "LlmShowReasoning")
            {
                return _reasoning;
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

            if (key == "VerifyWorkEnabled")
            {
                return Config.VerifyWorkEnabled;
            }

            if (key == "LlmShowReasoning")
            {
                return Config.LlmShowReasoning;
            }

            return Config.VisualQaEnabled;
        }

        private void SaveFeatures(bool showMessage)
        {
            bool ok = Config.WriteDword("MemoryEnabled", _memory.Checked ? 1 : 0)
                & Config.WriteDword("MemoryAutoExtract", _autoExtract.Checked ? 1 : 0)
                & Config.WriteDword("VerifyWorkEnabled", _verify.Checked ? 1 : 0)
                & Config.WriteDword("LlmShowReasoning", _reasoning.Checked ? 1 : 0)
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
            int y = 0;

            // Tom tat dang cap nhan (xam, cot trai) + gia tri (dam): truoc day la mot doan van voi dau "·" nen
            // khoang 5 thong tin tron vao nhau, kho quet lai.
            _summaryPanel.Location = new Point(24, y);
            _summaryPanel.Size = new Size(Width0 - 48, 122);      // 4 dong x 30px
            _summaryPanel.BackColor = PaneTheme.PaneBg;
            panel.Controls.Add(_summaryPanel);
            y += 130;

            _tryNow.Location = new Point(24, y);
            _tryNow.Size = new Size(Width0 - 48, 34);
            _tryNow.Click += delegate { TryNow(); };
            panel.Controls.Add(_tryNow);

            y += 42;
            _tryResultCard.Location = new Point(24, y);
            _tryResultCard.Size = new Size(Width0 - 48, 54);
            _tryResultCard.Visible = false;      // chua thu thi khong hien o trang
            _tryResult.AutoSize = false;
            _tryResult.Location = new Point(12, 8);
            _tryResult.Size = new Size(Width0 - 48 - 24, 38);
            _tryResult.TextAlign = ContentAlignment.MiddleLeft;   // cung ly do nhu the ket qua thu ket noi
            _tryResult.Font = PaneTheme.Small;
            _tryResult.ForeColor = PaneTheme.TextSecondary;
            _tryResultCard.Controls.Add(_tryResult);
            panel.Controls.Add(_tryResultCard);

            y += 62;
            panel.Controls.Add(new Label
            {
                AutoSize = false,
                Location = new Point(24, y),
                Size = new Size(Width0 - 48, 40),
                Font = PaneTheme.Small,
                ForeColor = PaneTheme.TextMuted,
                Text = "Mở lại \"Ask AI\" trong thẻ Axiom Office để bắt đầu."
            });
        }

        // Tom tat duoc ve lai moi lan vao buoc 5 (gia tri lay tu cac o cua buoc 3 va 4).
        private void RefreshSummary()
        {
            _summaryPanel.Controls.Clear();
            string model = _model.Text.Trim();
            string endpoint = _endpoint.Text.Trim();
            string core = _info != null ? "cổng " + SetupInfo.Text("port", _info.Core, "?") : "chưa chạy";
            string[][] rows =
            {
                new[] { "Model", model.Length > 0 ? model : "(chưa chọn)" },
                new[] { "Máy chủ AI", endpoint.Length > 0 ? endpoint : "(chưa có địa chỉ)" },
                new[] { "Ghi nhớ dài hạn", _memory.Checked ? "bật" : "tắt" },
                // Trang thai Core la mot su that rieng, khong duoc dan vao gia tri cua dong tren: gop lai thi
                // nguoi dung doc thanh "ghi nho: bat, core chua chay" nhu the hai thu lien quan voi nhau.
                new[] { "Agent Core", core },
            };

            int y = 0;
            foreach (string[] row in rows)
            {
                _summaryPanel.Controls.Add(new Label
                {
                    AutoSize = false,
                    Location = new Point(0, y),
                    Size = new Size(150, 22),
                    Font = PaneTheme.Small,
                    ForeColor = PaneTheme.TextMuted,
                    Text = row[0]
                });
                _summaryPanel.Controls.Add(new Label
                {
                    AutoSize = false,
                    Location = new Point(154, y - 1),
                    Size = new Size(_summaryPanel.Width - 154, 22),
                    Font = PaneTheme.SmallBold,
                    ForeColor = PaneTheme.TextPrimary,
                    Text = row[1]
                });
                y += 30;
            }
        }

        private void SetTryResult(string text, string tone)
        {
            _tryResultCard.Visible = true;
            _tryResult.Text = text;
            _tryResult.ForeColor = tone == "danger" ? PaneTheme.DangerFg
                : tone == "success" ? PaneTheme.Success
                : PaneTheme.TextSecondary;
            _tryResultCard.Tone = tone;
        }

        private void TryNow()
        {
            if (_host == null)
            {
                SetTryResult("Không có tài liệu đang mở để thử. Mở một tài liệu rồi bấm Thử ngay.", "danger");
                return;
            }

            _tryNow.Enabled = false;
            SetTryResult("Đang chạy thử…", "neutral");

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
                        SetTryResult("Xong: " + Shorten(reply, 160), "success");
                        return;
                    }

                    SetTryResult("Chưa chạy được: " + (error ?? "không rõ lỗi")
                        + " — kiểm tra lại bước Kết nối máy chủ AI.", "danger");
                });
            });
        }
    }
}

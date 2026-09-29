using System;
using System.Drawing;
using System.Windows.Forms;
using AxiomOffice.Bridge;

namespace AxiomOffice.Ai
{
    internal sealed class SettingsForm : Form
    {
        private const string DefaultOpenAiEndpoint = "https://api.openai.com/v1";
        private const string DefaultAnthropicEndpoint = "https://api.anthropic.com/v1";
        // Gemini dung endpoint OpenAI-compatible cua Google: luu provider "openai" nen Core, agent in-process
        // va ban cai cu deu chay duoc, khong can SDK rieng.
        private const string DefaultGeminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/openai";
        private const string DefaultGeminiModel = "gemini-2.5-flash";

        private readonly ComboBox _provider;
        private readonly TextBox _endpoint;
        private readonly TextBox _apiKey;
        private readonly TextBox _model;
        private readonly Label _status;
        private readonly Button _test;
        private readonly CheckBox _core;
        private readonly CheckBox _memory;
        private readonly CheckBox _autoExtract;
        private readonly CheckBox _visual;

        public SettingsForm()
        {
            Text = "Axiom Office - AI Settings";
            FormBorderStyle = FormBorderStyle.FixedDialog;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterScreen;
            ClientSize = new Size(500, 334);
            Font = SystemFonts.MessageBoxFont;

            var lblProvider = new Label { Text = "Provider:", Left = 14, Top = 19, Width = 90 };
            _provider = new ComboBox { Left = 110, Top = 15, Width = 370, DropDownStyle = ComboBoxStyle.DropDownList };
            _provider.Items.Add("OpenAI-compatible");
            _provider.Items.Add("Anthropic");
            _provider.Items.Add("Google Gemini");

            var lblEndpoint = new Label { Text = "Endpoint:", Left = 14, Top = 55, Width = 90 };
            _endpoint = new TextBox { Left = 110, Top = 51, Width = 370 };

            var lblKey = new Label { Text = "API key:", Left = 14, Top = 91, Width = 90 };
            _apiKey = new TextBox { Left = 110, Top = 87, Width = 370, UseSystemPasswordChar = true };

            var lblModel = new Label { Text = "Model:", Left = 14, Top = 127, Width = 90 };
            _model = new TextBox { Left = 110, Top = 123, Width = 370 };

            var hint = new Label
            {
                Text = "OpenAI-compatible: base URL (e.g. https://api.openai.com/v1, http://localhost:11434/v1). Gemini: key from aistudio.google.com.",
                Left = 110,
                Top = 151,
                Width = 370,
                Height = 28,
                ForeColor = SystemColors.GrayText
            };

            // Agent Core + ghi nhớ dài hạn (New_arch.md mục 9.4, 8.5.10).
            var lblAgent = new Label { Text = "Agent:", Left = 14, Top = 190, Width = 90 };
            _core = new CheckBox { Text = "Dùng Agent Core (hội thoại liên tục, kỹ năng, ghi nhớ)", Left = 110, Top = 186, Width = 370 };
            _memory = new CheckBox { Text = "Ghi nhớ dài hạn", Left = 110, Top = 210, Width = 150 };
            _autoExtract = new CheckBox { Text = "Tự ghi nhớ sau mỗi lượt", Left = 270, Top = 210, Width = 210 };
            var manage = new Button { Text = "Quản lý ghi nhớ…", Left = 110, Top = 236, Width = 150 };
            // QA thị giác (New_arch.md 8.4.6): tắt mặc định vì mỗi ảnh tốn nhiều token; cần model đọc được ảnh.
            _visual = new CheckBox { Text = "Cho AI xem ảnh chụp cửa sổ (tốn token)", Left = 270, Top = 239, Width = 210 };

            _status = new Label { Left = 14, Top = 270, Width = 466, Height = 18, AutoEllipsis = true, ForeColor = SystemColors.GrayText };

            _test = new Button { Text = "Test", Left = 110, Top = 294, Width = 80 };
            var save = new Button { Text = "Save", Left = 312, Top = 294, Width = 80 };
            var cancel = new Button { Text = "Cancel", Left = 400, Top = 294, Width = 80 };

            Controls.AddRange(new Control[] { lblProvider, _provider, lblEndpoint, _endpoint, lblKey, _apiKey, lblModel, _model, hint,
                lblAgent, _core, _memory, _autoExtract, manage, _visual, _status, _test, save, cancel });
            _memory.CheckedChanged += delegate { _autoExtract.Enabled = _memory.Checked; };
            manage.Click += delegate
            {
                using (var form = new MemoryForm())
                {
                    form.ShowDialog(this);
                }
            };
            AcceptButton = save;
            CancelButton = cancel;

            _provider.SelectedIndexChanged += OnProviderChanged;
            _test.Click += OnTest;
            save.Click += OnSave;
            cancel.Click += delegate { DialogResult = DialogResult.Cancel; Close(); };

            Load += delegate { LoadConfig(); };
        }

        private string SelectedProvider()
        {
            return _provider.SelectedIndex == 1 ? "anthropic" : "openai";
        }

        private string DefaultEndpoint()
        {
            return _provider.SelectedIndex == 1 ? DefaultAnthropicEndpoint
                : _provider.SelectedIndex == 2 ? DefaultGeminiEndpoint
                : DefaultOpenAiEndpoint;
        }

        internal static bool IsGeminiEndpoint(string endpoint)
        {
            return (endpoint ?? "").IndexOf("generativelanguage.googleapis.com", StringComparison.OrdinalIgnoreCase) >= 0;
        }

        private void LoadConfig()
        {
            _provider.SelectedIndex = Config.LlmProvider == "anthropic" ? 1 : IsGeminiEndpoint(Config.LlmEndpoint) ? 2 : 0;
            _endpoint.Text = Config.LlmEndpoint;
            _apiKey.Text = Config.LlmApiKey;
            _model.Text = Config.LlmModel;
            _core.Checked = Config.CoreEnabled;
            _memory.Checked = Config.MemoryEnabled;
            _autoExtract.Checked = Config.MemoryAutoExtract;
            _autoExtract.Enabled = _memory.Checked;
            _visual.Checked = Config.VisualQaEnabled;
            if (string.IsNullOrEmpty(_endpoint.Text))
            {
                _endpoint.Text = DefaultEndpoint();
            }
        }

        private void OnProviderChanged(object sender, EventArgs e)
        {
            string current = (_endpoint.Text ?? "").Trim();
            if (current.Length == 0 || current == DefaultOpenAiEndpoint || current == DefaultAnthropicEndpoint || current == DefaultGeminiEndpoint)
            {
                _endpoint.Text = DefaultEndpoint();
            }
            if (_provider.SelectedIndex == 2 && (_model.Text ?? "").Trim().Length == 0)
            {
                _model.Text = DefaultGeminiModel;
            }
        }

        private void OnSave(object sender, EventArgs e)
        {
            if ((_endpoint.Text ?? "").Trim().Length == 0)
            {
                _status.Text = "Endpoint is required";
                return;
            }
            if ((_model.Text ?? "").Trim().Length == 0)
            {
                _status.Text = "Model is required";
                return;
            }
            bool saved = Config.WriteString("LlmProvider", SelectedProvider());
            saved &= Config.WriteString("LlmEndpoint", _endpoint.Text.Trim());
            saved &= Config.WriteSecret("LlmApiKey", _apiKey.Text);
            saved &= Config.WriteString("LlmModel", _model.Text.Trim());
            saved &= Config.WriteDword("CoreEnabled", _core.Checked ? 1 : 0);
            saved &= Config.WriteDword("MemoryEnabled", _memory.Checked ? 1 : 0);
            saved &= Config.WriteDword("MemoryAutoExtract", _autoExtract.Checked ? 1 : 0);
            saved &= Config.WriteDword("VisualQaEnabled", _visual.Checked ? 1 : 0);
            if (!saved)
            {
                _status.Text = "Failed to save settings to the registry";
                Logger.Error("AI settings save failed (registry write)", null);
                return;
            }
            Logger.Info("AI settings saved: provider=" + SelectedProvider() + " endpoint=" + _endpoint.Text.Trim() + " model=" + _model.Text.Trim());
            DialogResult = DialogResult.OK;
            Close();
        }

        private void OnTest(object sender, EventArgs e)
        {
            _test.Enabled = false;
            _status.Text = "Testing connection...";
            Cursor = Cursors.WaitCursor;
            Application.DoEvents();
            LlmResult result = LlmClient.Chat(
                "You are a connectivity test.",
                "Reply with a single word: OK",
                SelectedProvider(),
                _endpoint.Text.Trim(),
                _apiKey.Text,
                _model.Text.Trim());
            Cursor = Cursors.Default;
            _test.Enabled = true;
            if (result.Ok)
            {
                string text = (result.Text ?? "").Trim();
                if (text.Length > 60)
                {
                    text = text.Substring(0, 60) + "...";
                }
                _status.Text = "OK (" + result.Seconds.ToString("0.0") + "s): " + text;
            }
            else
            {
                string error = result.Error ?? "unknown error";
                if (error.Length > 140)
                {
                    error = error.Substring(0, 140) + "...";
                }
                _status.Text = "Error: " + error;
            }
        }
    }
}

using System;
using System.Drawing;
using System.Windows.Forms;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Ai
{
    internal sealed class SettingsForm : Form
    {
        private const string DefaultOpenAiEndpoint = "https://api.openai.com/v1";
        private const string DefaultAnthropicEndpoint = "https://api.anthropic.com/v1";

        private readonly ComboBox _provider;
        private readonly TextBox _endpoint;
        private readonly TextBox _apiKey;
        private readonly TextBox _model;
        private readonly Label _status;
        private readonly Button _test;

        public SettingsForm()
        {
            Text = "WPS AI Bridge - AI Settings";
            FormBorderStyle = FormBorderStyle.FixedDialog;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterScreen;
            ClientSize = new Size(500, 250);
            Font = SystemFonts.MessageBoxFont;

            var lblProvider = new Label { Text = "Provider:", Left = 14, Top = 19, Width = 90 };
            _provider = new ComboBox { Left = 110, Top = 15, Width = 370, DropDownStyle = ComboBoxStyle.DropDownList };
            _provider.Items.Add("OpenAI-compatible");
            _provider.Items.Add("Anthropic");

            var lblEndpoint = new Label { Text = "Endpoint:", Left = 14, Top = 55, Width = 90 };
            _endpoint = new TextBox { Left = 110, Top = 51, Width = 370 };

            var lblKey = new Label { Text = "API key:", Left = 14, Top = 91, Width = 90 };
            _apiKey = new TextBox { Left = 110, Top = 87, Width = 370, UseSystemPasswordChar = true };

            var lblModel = new Label { Text = "Model:", Left = 14, Top = 127, Width = 90 };
            _model = new TextBox { Left = 110, Top = 123, Width = 370 };

            var hint = new Label
            {
                Text = "OpenAI-compatible: base URL (e.g. https://api.openai.com/v1, http://localhost:11434/v1). Model name is free text.",
                Left = 110,
                Top = 151,
                Width = 370,
                Height = 28,
                ForeColor = SystemColors.GrayText
            };

            _status = new Label { Left = 14, Top = 186, Width = 466, Height = 18, AutoEllipsis = true, ForeColor = SystemColors.GrayText };

            _test = new Button { Text = "Test", Left = 110, Top = 210, Width = 80 };
            var save = new Button { Text = "Save", Left = 312, Top = 210, Width = 80 };
            var cancel = new Button { Text = "Cancel", Left = 400, Top = 210, Width = 80 };

            Controls.AddRange(new Control[] { lblProvider, _provider, lblEndpoint, _endpoint, lblKey, _apiKey, lblModel, _model, hint, _status, _test, save, cancel });
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

        private void LoadConfig()
        {
            _provider.SelectedIndex = Config.LlmProvider == "anthropic" ? 1 : 0;
            _endpoint.Text = Config.LlmEndpoint;
            _apiKey.Text = Config.LlmApiKey;
            _model.Text = Config.LlmModel;
            if (string.IsNullOrEmpty(_endpoint.Text))
            {
                _endpoint.Text = SelectedProvider() == "anthropic" ? DefaultAnthropicEndpoint : DefaultOpenAiEndpoint;
            }
        }

        private void OnProviderChanged(object sender, EventArgs e)
        {
            string current = (_endpoint.Text ?? "").Trim();
            if (current.Length == 0 || current == DefaultOpenAiEndpoint || current == DefaultAnthropicEndpoint)
            {
                _endpoint.Text = SelectedProvider() == "anthropic" ? DefaultAnthropicEndpoint : DefaultOpenAiEndpoint;
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
            Config.WriteString("LlmProvider", SelectedProvider());
            Config.WriteString("LlmEndpoint", _endpoint.Text.Trim());
            Config.WriteString("LlmApiKey", _apiKey.Text);
            Config.WriteString("LlmModel", _model.Text.Trim());
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

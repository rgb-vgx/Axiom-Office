using System;
using System.Drawing;
using System.Windows.Forms;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Ai
{
    internal sealed class AskAiForm : Form
    {
        private readonly Connect _connect;
        private readonly string _selectionText;

        private readonly TextBox _prompt;
        private readonly ComboBox _contextMode;
        private readonly TextBox _result;
        private readonly Label _status;
        private readonly Button _ask;
        private readonly Button _insert;

        public AskAiForm(Connect connect)
        {
            _connect = connect;
            _selectionText = DocumentContext.GetSelection(connect);

            Text = "WPS AI Bridge - Ask AI";
            FormBorderStyle = FormBorderStyle.Sizable;
            MaximizeBox = true;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterScreen;
            ClientSize = new Size(560, 460);
            MinimumSize = new Size(460, 380);
            Font = SystemFonts.MessageBoxFont;

            var lblPrompt = new Label { Text = "Prompt:", Left = 14, Top = 14, Width = 90 };
            _prompt = new TextBox
            {
                Left = 14,
                Top = 34,
                Width = 532,
                Height = 90,
                Multiline = true,
                ScrollBars = ScrollBars.Vertical,
                Anchor = AnchorStyles.Top | AnchorStyles.Left | AnchorStyles.Right
            };

            var lblContext = new Label { Text = "Context:", Left = 14, Top = 134, Width = 90 };
            _contextMode = new ComboBox
            {
                Left = 110,
                Top = 130,
                Width = 300,
                DropDownStyle = ComboBoxStyle.DropDownList
            };
            _contextMode.Items.Add("None");
            _contextMode.Items.Add(_selectionText.Length > 0 ? "Selection (" + _selectionText.Length + " chars)" : "Selection (empty)");
            _contextMode.Items.Add("Whole document (max " + DocumentContext.MaxChars + " chars)");
            _contextMode.SelectedIndex = _selectionText.Length > 0 ? 1 : 2;

            var lblResult = new Label { Text = "Result:", Left = 14, Top = 160, Width = 90 };
            _result = new TextBox
            {
                Left = 14,
                Top = 180,
                Width = 532,
                Height = 210,
                Multiline = true,
                ReadOnly = true,
                ScrollBars = ScrollBars.Vertical,
                Anchor = AnchorStyles.Top | AnchorStyles.Bottom | AnchorStyles.Left | AnchorStyles.Right
            };

            _status = new Label
            {
                Left = 14,
                Top = 400,
                Width = 532,
                Height = 18,
                AutoEllipsis = true,
                ForeColor = SystemColors.GrayText,
                Anchor = AnchorStyles.Bottom | AnchorStyles.Left | AnchorStyles.Right
            };

            _ask = new Button { Text = "Ask", Left = 320, Top = 424, Width = 100, Anchor = AnchorStyles.Bottom | AnchorStyles.Right };
            _insert = new Button { Text = "Insert at cursor", Left = 426, Top = 424, Width = 120, Enabled = false, Anchor = AnchorStyles.Bottom | AnchorStyles.Right };
            var close = new Button { Text = "Close", Left = 14, Top = 424, Width = 90, Anchor = AnchorStyles.Bottom | AnchorStyles.Left };

            Controls.AddRange(new Control[] { lblPrompt, _prompt, lblContext, _contextMode, lblResult, _result, _status, _ask, _insert, close });
            AcceptButton = _ask;

            _ask.Click += OnAsk;
            _insert.Click += OnInsert;
            close.Click += delegate { Close(); };
        }

        private void OnAsk(object sender, EventArgs e)
        {
            string promptText = (_prompt.Text ?? "").Trim();
            if (promptText.Length == 0)
            {
                _status.Text = "Enter a prompt first";
                return;
            }
            if (Config.LlmEndpoint.Trim().Length == 0 || Config.LlmModel.Trim().Length == 0)
            {
                _status.Text = "AI is not configured - open Settings first";
                return;
            }

            int contextMode = _contextMode.SelectedIndex;
            string model = Config.LlmModel;
            string systemPrompt =
                "You are an AI assistant integrated into WPS Office. " +
                "You may receive the content of the currently open document or the user's selection after '--- Document context ---'. " +
                "Reply concisely, in the same language as the user's request. " +
                "If the user asks to rewrite, translate, summarize or generate text, reply with only the resulting text, without explanations.";

            _ask.Enabled = false;
            _insert.Enabled = false;
            _status.Text = contextMode == 2 ? "Reading document..." : "Calling " + model + "...";
            Logger.Info("AskAI: model=" + model + " context=" + contextMode + " promptChars=" + promptText.Length);

            System.Threading.ThreadPool.QueueUserWorkItem(delegate
            {
                string context = "";
                try
                {
                    if (contextMode == 1)
                    {
                        context = _selectionText;
                    }
                    else if (contextMode == 2)
                    {
                        context = DocumentContext.GetWholeDocument(_connect, _connect.AppKind);
                    }
                }
                catch
                {
                }

                string userPrompt = promptText;
                if (context.Length > 0)
                {
                    userPrompt += "\n\n--- Document context (" + context.Length + " chars) ---\n" + context;
                }

                try
                {
                    BeginInvoke(new Action(delegate { _status.Text = "Calling " + model + "..."; }));
                }
                catch
                {
                }

                LlmResult result = LlmClient.Chat(systemPrompt, userPrompt);
                try
                {
                    BeginInvoke(new Action(delegate
                    {
                        _ask.Enabled = true;
                        if (result.Ok)
                        {
                            _result.Text = result.Text;
                            _insert.Enabled = (_result.Text ?? "").Length > 0;
                            _status.Text = "Done in " + result.Seconds.ToString("0.0") + "s, context " + context.Length + " chars";
                            Logger.Info("AskAI: ok in " + result.Seconds.ToString("0.0") + "s, " + (_result.Text ?? "").Length + " chars, context " + context.Length);
                        }
                        else
                        {
                            _status.Text = "Error: " + Trim(result.Error, 140);
                            Logger.Error("AskAI failed: " + result.Error, null);
                        }
                    }));
                }
                catch
                {
                }
            });
        }

        private void OnInsert(object sender, EventArgs e)
        {
            string text = _result.Text ?? "";
            if (text.Length == 0)
            {
                return;
            }
            try
            {
                dynamic app = _connect.Application;
                app.Selection.TypeText(text);
                _status.Text = "Inserted into document.";
                Logger.Info("AskAI: inserted " + text.Length + " chars into document");
            }
            catch (Exception ex)
            {
                _status.Text = "Insert failed: " + Trim(ex.Message, 120);
                Logger.Error("AskAI insert failed", ex);
            }
        }

        private static string Trim(string s, int max)
        {
            if (string.IsNullOrEmpty(s) || s.Length <= max)
            {
                return s ?? "";
            }
            return s.Substring(0, max) + "...";
        }
    }
}

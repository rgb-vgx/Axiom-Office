using System;
using System.Drawing;
using System.Threading;
using System.Windows.Forms;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Ai
{
    [System.Runtime.InteropServices.ComVisible(true)]
    [System.Runtime.InteropServices.Guid("D99F8693-4316-45AF-8916-B70D87DEEF87")]
    [System.Runtime.InteropServices.ProgId("WpsAiBridge.AskAiPane")]
    [System.Runtime.InteropServices.ClassInterface(System.Runtime.InteropServices.ClassInterfaceType.AutoDispatch)]
    public class AskAiPane : UserControl
    {
        internal static Connect CurrentHost;

        private readonly RichTextBox _log;
        private readonly TextBox _prompt;
        private readonly Button _send;
        private readonly Button _insert;
        private string _lastReply = "";
        private bool _busy;

        public AskAiPane() : this(null)
        {
        }

        internal AskAiPane(Connect connect)
        {
            if (connect != null)
            {
                CurrentHost = connect;
            }
            Dock = DockStyle.Fill;
            Font = SystemFonts.MessageBoxFont;
            BackColor = Color.White;

            _log = new RichTextBox
            {
                Dock = DockStyle.Fill,
                ReadOnly = true,
                BorderStyle = BorderStyle.None,
                BackColor = Color.White,
                WordWrap = true,
            };

            var bottom = new Panel { Dock = DockStyle.Bottom, Height = 140 };

            var buttons = new FlowLayoutPanel
            {
                Dock = DockStyle.Bottom,
                Height = 30,
                FlowDirection = FlowDirection.LeftToRight,
            };
            _send = new Button { Text = "Ask", Width = 70, Height = 24 };
            _insert = new Button { Text = "Insert reply", Width = 96, Height = 24, Enabled = false };
            var settings = new Button { Text = "Settings", Width = 76, Height = 24 };
            buttons.Controls.Add(settings);
            buttons.Controls.Add(_insert);
            buttons.Controls.Add(_send);

            var status = new Label
            {
                Dock = DockStyle.Bottom,
                Height = 18,
                AutoEllipsis = true,
                ForeColor = SystemColors.GrayText,
            };
            _status = status;

            _prompt = new TextBox
            {
                Dock = DockStyle.Fill,
                Multiline = true,
                ScrollBars = ScrollBars.Vertical,
                Font = SystemFonts.MessageBoxFont,
            };

            bottom.Controls.Add(_prompt);
            bottom.Controls.Add(status);
            bottom.Controls.Add(buttons);
            Controls.Add(_log);
            Controls.Add(bottom);

            _send.Click += OnSend;
            _insert.Click += OnInsert;
            settings.Click += delegate
            {
                using (var form = new SettingsForm())
                {
                    form.ShowDialog(this);
                }
            };

            AppendLine("WPS AI Bridge - Ask AI");
            AppendLine("Vi du: \"Soan cho toi mot don xin viec\"; \"Tao 5 slide gioi thieu cong ty\"; \"Bang diem 3 mon cho 5 hoc sinh\".");
        }

        private Label _status;

        private void AppendLine(string line)
        {
            _log.AppendText(line + Environment.NewLine);
            _log.SelectionStart = _log.TextLength;
            _log.ScrollToCaret();
        }

        private void OnSend(object sender, EventArgs e)
        {
            if (_busy)
            {
                return;
            }
            string promptText = (_prompt.Text ?? "").Trim();
            if (promptText.Length == 0)
            {
                _status.Text = "Nhap yeu cau truoc";
                return;
            }
            if (Config.LlmEndpoint.Trim().Length == 0 || Config.LlmModel.Trim().Length == 0)
            {
                _status.Text = "Chua cau hinh AI - bam Settings";
                return;
            }
            Connect host = CurrentHost;
            if (host == null)
            {
                _status.Text = "Khong co ket noi toi ung dung";
                return;
            }

            _busy = true;
            _send.Enabled = false;
            _insert.Enabled = false;
            AppendLine("");
            AppendLine("> " + promptText);
            _prompt.Clear();
            _status.Text = "AI dang lam viec...";
            Logger.Info("AskAiPane: prompt=" + promptText);

            Action<string> progress = delegate(string line)
            {
                try
                {
                    BeginInvoke(new Action(delegate { AppendLine("  " + line); }));
                }
                catch
                {
                }
            };

            ThreadPool.QueueUserWorkItem(delegate
            {
                LlmResult result = AiAgent.Run(host, promptText, progress);
                try
                {
                    BeginInvoke(new Action(delegate
                    {
                        _busy = false;
                        _send.Enabled = true;
                        if (result.Ok)
                        {
                            if (!string.IsNullOrEmpty(result.Text))
                            {
                                AppendLine("");
                                AppendLine(result.Text);
                                _lastReply = result.Text;
                                _insert.Enabled = host.AppKind == "wps";
                            }
                            _status.Text = "Xong trong " + result.Seconds.ToString("0.0") + "s";
                            Logger.Info("AskAiPane: ok in " + result.Seconds.ToString("0.0") + "s, " + result.Transcript.Count + " tool calls");
                        }
                        else
                        {
                            _status.Text = "Loi: " + Truncate(result.Error, 120);
                            AppendLine("Loi: " + result.Error);
                            Logger.Error("AskAiPane failed: " + result.Error, null);
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
                _status.Text = "Da chen vao tai lieu.";
            }
            catch (Exception ex)
            {
                _status.Text = "Chen that bai: " + Truncate(ex.Message, 100);
            }
        }

        private static string Truncate(string text, int max)
        {
            if (string.IsNullOrEmpty(text) || text.Length <= max)
            {
                return text ?? "";
            }
            return text.Substring(0, max) + "...";
        }
    }

    internal sealed class AskAiHostForm : Form
    {
        public AskAiHostForm(Connect connect)
        {
            Text = "WPS AI Bridge - Ask AI";
            ClientSize = new Size(430, 560);
            StartPosition = FormStartPosition.CenterScreen;
            Font = SystemFonts.MessageBoxFont;
            Controls.Add(new AskAiPane(connect));
        }
    }
}

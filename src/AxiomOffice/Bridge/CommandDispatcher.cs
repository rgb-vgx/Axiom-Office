using System;
using System.Collections.Generic;
using System.Linq;
using System.Runtime.InteropServices;
using System.Threading;

namespace AxiomOffice.Bridge
{
    // Thực thi lệnh POST /cmd (và tool office_action của Ask AI pane). Lệnh được khai báo cạnh handler
    // trong CommandDispatcher.Writer/Spreadsheet/Presentation.cs; thêm lệnh = thêm một dòng Command(...)
    // + handler, không cần sửa nơi nào khác (README: AxiomOffice.Host.exe commands --markdown).
    internal static partial class CommandDispatcher
    {
        public static readonly string BridgeVersion =
            typeof(CommandDispatcher).Assembly.GetName().Version.ToString(3);

        private const int MaxComRetries = 10;

        private static int _firstCommandLogged;

        // Mọi lệnh, theo thứ tự hiển thị trong README và mô tả tool.
        internal static readonly CommandInfo[] Commands = GeneralCommands()
            .Concat(WriterCommands())
            .Concat(SpreadsheetCommands())
            .Concat(PresentationCommands())
            .Concat(CheckCommands())
            .ToArray();

        private static readonly Dictionary<string, CommandInfo> CommandsByName =
            Commands.ToDictionary(c => c.Name, StringComparer.Ordinal);

        public static CommandInfo FindCommand(string action)
        {
            CommandInfo command;
            return action != null && CommandsByName.TryGetValue(action, out command) ? command : null;
        }

        public static object Execute(IAppHost host, string action, Dictionary<string, object> p)
        {
            CommandInfo command = FindCommand(action);
            bool gated = command == null || command.Gated;
            if (Interlocked.Exchange(ref _firstCommandLogged, 1) == 0)
            {
                try
                {
                    Logger.Info("First bridge command '" + action + "': " + ComGate.Run(delegate { return HostProbe.Describe(host); }));
                }
                catch (Exception ex)
                {
                    Logger.Error("First bridge command: document state unavailable", ex);
                }
            }
            int attempt = 0;
            while (true)
            {
                attempt++;
                try
                {
                    return gated
                        ? ComGate.Run(delegate { return ExecuteAction(host, action, command, p); })
                        : ExecuteAction(host, action, command, p);
                }
                catch (COMException ex)
                {
                    if (IsRetryableComError(ex) && attempt <= MaxComRetries)
                    {
                        int delayMs = Math.Min(100 * attempt, 1000);
                        Logger.Info("COM call rejected [" + ex.HResult.ToString("X8") + "], retry " + attempt + "/" + MaxComRetries + " in " + delayMs + "ms");
                        Thread.Sleep(delayMs);
                        continue;
                    }
                    Logger.Error("Action failed: " + action, ex);
                    return Err(ex.GetType().Name + ": " + ex.Message);
                }
                catch (Exception ex)
                {
                    Logger.Error("Action failed: " + action, ex);
                    return Err(ex.GetType().Name + ": " + ex.Message);
                }
            }
        }

        private static object ExecuteAction(IAppHost host, string action, CommandInfo command, Dictionary<string, object> p)
        {
            EnsureVisibleWordDocument(host, action);
            if (command == null)
            {
                return Err("unknown action: " + action);
            }
            if (command.Kind != null)
            {
                RequireKind(host, command.Kind);
            }
            try
            {
                return Ok(command.Handler(host, p));
            }
            catch (CommandRejectedException ex)
            {
                return Err(ex.Message);
            }
        }

        private static CommandInfo Command(string name, string kind, CommandHandler handler, string summary, params CommandParam[] parameters)
        {
            return new CommandInfo(name, kind, handler, summary, parameters);
        }

        private static CommandParam Req(string name, string hint = null)
        {
            return new CommandParam(name, true, hint);
        }

        private static CommandParam Opt(string name, string hint = null)
        {
            return new CommandParam(name, false, hint);
        }

        // Lỗi trả nguyên văn cho client ({"ok":false,"error":message}), không log như lỗi COM/tham số.
        private sealed class CommandRejectedException : Exception
        {
            public CommandRejectedException(string message)
                : base(message)
            {
            }
        }

        private static bool IsRetryableComError(COMException ex)
        {
            return HostProbe.IsRetryable(ex);
        }

        // Lệnh writer.* luôn nhắm vào tài liệu người dùng đang thấy: nếu ActiveDocument không có cửa sổ
        // hiển thị (tài liệu ẩn) mà có tài liệu khác đang hiển thị thì kích hoạt tài liệu đó trước.
        private static void EnsureVisibleWordDocument(IAppHost host, string action)
        {
            if (host.AppKind != "wps" || !action.StartsWith("writer.", StringComparison.Ordinal)
                || action == "writer.newDocument" || action == "writer.open" || action == "writer.closeAll")
            {
                return;
            }
            dynamic app = host.Application;
            dynamic active;
            try
            {
                active = app.ActiveDocument;
            }
            catch (COMException ex)
            {
                if (IsRetryableComError(ex))
                {
                    throw;
                }
                return;
            }
            if (IsWordDocumentVisible(active))
            {
                return;
            }
            dynamic documents = app.Documents;
            int count = Convert.ToInt32(documents.Count);
            for (int i = 1; i <= count; i++)
            {
                dynamic doc = documents[i];
                if (IsWordDocumentVisible(doc))
                {
                    Logger.Info("ActiveDocument '" + Convert.ToString(active.FullName) + "' has no visible window; activating '" +
                        Convert.ToString(doc.FullName) + "' for " + action);
                    doc.Activate();
                    return;
                }
            }
        }

        private static bool IsWordDocumentVisible(dynamic doc)
        {
            try
            {
                dynamic windows = doc.Windows;
                if (Convert.ToInt32(windows.Count) == 0)
                {
                    return false;
                }
                return Convert.ToBoolean(windows[1].Visible);
            }
            catch (COMException ex)
            {
                if (IsRetryableComError(ex))
                {
                    throw;
                }
                return true;
            }
        }

        public static object Health(IAppHost host, int port)
        {
            return Ok(new Dictionary<string, object>
            {
                { "app", host.AppKind },
                { "pid", System.Diagnostics.Process.GetCurrentProcess().Id },
                { "port", port },
                { "version", BridgeVersion },
                { "log", Logger.LogFilePath }
            });
        }

        public static object ConfigInfo()
        {
            return Ok(new Dictionary<string, object>
            {
                { "provider", Config.LlmProvider },
                { "endpoint", Config.LlmEndpoint },
                { "model", Config.LlmModel },
                { "apiKey", MaskKey(Config.LlmApiKey) }
            });
        }

        private static string MaskKey(string key)
        {
            if (string.IsNullOrEmpty(key))
            {
                return "";
            }
            if (key.Length <= 8)
            {
                return "********";
            }
            return key.Substring(0, 4) + "..." + key.Substring(key.Length - 4);
        }

        // Lệnh dùng được với mọi app.
        private static IEnumerable<CommandInfo> GeneralCommands()
        {
            return new[]
            {
                Command("app.info", null, AppInfo, "Tên/version app, tài liệu đang mở, `state` (tài liệu, cửa sổ, visible)"),
                // ai.ask giữ lệnh suốt các vòng gọi LLM (hàng chục giây); từng tool của nó tự qua ComGate.
                Command("ai.ask", null, AiAsk, "Chạy AI agent trên tài liệu đang mở; trả `reply`, `transcript`, `seconds`, `rounds`",
                    Req("prompt")).Ungated(),
                // ui.askpane chuyển sang UI thread; giữ cổng ở đây có thể deadlock với UI thread đang chờ cổng.
                Command("ui.askpane", null, UiAskPane, "Mở task pane Ask AI").Ungated()
            };
        }

        private static Dictionary<string, object> AiAsk(IAppHost host, Dictionary<string, object> p)
        {
            string prompt = ParamString(p, "prompt", null);
            if (string.IsNullOrEmpty(prompt))
            {
                throw new InvalidOperationException("'prompt' is required");
            }
            // Giai đoạn 4 (New_arch.md 7.7): chạy qua Agent Core khi có (skill, memory, policy, MCP), dịch kết quả về
            // đúng hình dạng cũ; Core không dùng được (chưa nhận lượt chạy) thì chạy agent in-process như trước.
            Ai.LlmResult agentResult = null;
            var connect = host as Connect;
            if (connect != null && Ai.CoreClient.Instance.Enabled)
            {
                agentResult = Ai.CoreClient.Instance.Run(connect, prompt, null, delegate(Ai.CoreEvent item)
                {
                    if (item.Type == "tool.finished" || item.Type == "skill.loaded" || item.Type == "memory.written")
                    {
                        Logger.Info("ai.ask (core) " + item.Type + ": " + (item.Action ?? item.Name ?? item.Text ?? item.Tool));
                    }
                }, System.Threading.CancellationToken.None, false);
            }
            if (agentResult == null)
            {
                agentResult = Ai.AiAgent.Run(host, prompt, delegate(string line)
                {
                    Logger.Info("ai.ask progress: " + line);
                });
            }
            var reply = new Dictionary<string, object>();
            reply["ok"] = agentResult.Ok;
            if (agentResult.Ok)
            {
                reply["reply"] = agentResult.Text;
            }
            else
            {
                reply["error"] = agentResult.Error;
            }
            reply["transcript"] = agentResult.Transcript;
            reply["seconds"] = agentResult.Seconds;
            reply["rounds"] = agentResult.Rounds;
            reply["viaCore"] = agentResult.ViaCore;
            return reply;
        }

        private static Dictionary<string, object> UiAskPane(IAppHost host, Dictionary<string, object> p)
        {
            Connect connect = host as Connect;
            if (connect == null)
            {
                throw new CommandRejectedException("ui.askpane is only available inside the in-process add-in");
            }
            bool shown = connect.TryShowTaskPane();
            return new Dictionary<string, object> { { "taskPane", shown } };
        }

        private static Dictionary<string, object> AppInfo(IAppHost host, Dictionary<string, object> p)
        {
            dynamic app = host.Application;
            var info = new Dictionary<string, object>
            {
                { "kind", host.AppKind },
                { "name", Safe(() => Convert.ToString(app.Name)) },
                { "version", Safe(() => Convert.ToString(app.Version)) }
            };

            info["state"] = Safe(() => HostProbe.Describe(host));
            if (host.AppKind == "wps")
            {
                info["documents"] = Safe(() => Convert.ToInt32(app.Documents.Count));
                info["activeDocument"] = Safe(() => new Dictionary<string, object>
                {
                    { "name", Convert.ToString(app.ActiveDocument.Name) },
                    { "fullName", Convert.ToString(app.ActiveDocument.FullName) },
                    { "saved", Convert.ToBoolean(app.ActiveDocument.Saved) }
                });
            }
            else if (host.AppKind == "et")
            {
                info["workbooks"] = Safe(() => Convert.ToInt32(app.Workbooks.Count));
                info["activeWorkbook"] = Safe(() => new Dictionary<string, object>
                {
                    { "name", Convert.ToString(app.ActiveWorkbook.Name) },
                    { "fullName", Convert.ToString(app.ActiveWorkbook.FullName) },
                    { "saved", Convert.ToBoolean(app.ActiveWorkbook.Saved) },
                    { "sheet", Convert.ToString(app.ActiveSheet.Name) }
                });
            }
            else if (host.AppKind == "wpp")
            {
                info["presentations"] = Safe(() => Convert.ToInt32(app.Presentations.Count));
                info["activePresentation"] = Safe(() => new Dictionary<string, object>
                {
                    { "name", Convert.ToString(app.ActivePresentation.Name) },
                    { "fullName", Convert.ToString(app.ActivePresentation.FullName) },
                    { "saved", Convert.ToBoolean(app.ActivePresentation.Saved) },
                    { "slides", Convert.ToInt32(app.ActivePresentation.Slides.Count) }
                });
            }
            return info;
        }
    }
}

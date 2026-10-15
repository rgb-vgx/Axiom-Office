using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Text;
using System.Threading;
using System.Windows.Forms;
using AxiomOffice.Bridge;

namespace AxiomOffice.Ribbon
{
    // docs/ribbon-ui.md: ribbon khởi động việc, pane điều phối. Chỉ đặt nút có đường thật đứng sau.
    internal static class RibbonUi
    {
        private const string VisualQaId = "tglVisualQa";

        // Nút soát hiện theo app (getVisible); host bỏ qua getVisible thì nút lạ báo "dùng cho …" khi bấm.
        public const string RibbonXml =
            "<customUI xmlns=\"http://schemas.microsoft.com/office/2006/01/customui\" onLoad=\"OnRibbonLoad\">" +
            "<ribbon><tabs>" +
            "<tab id=\"axiomOfficeTab\" label=\"Axiom Office\">" +
            "<group id=\"axiomAssistantGroup\" label=\"Trợ lý\">" +
            "<button id=\"btnAskAi\" label=\"Ask AI\" size=\"large\" imageMso=\"Help\" onAction=\"OnButtonAction\" tag=\"askAi\"" +
            " screentip=\"Ask AI\" supertip=\"Mở khung trò chuyện với AI bên cạnh tài liệu.\"/>" +
            "</group>" +
            "<group id=\"axiomCheckGroup\" label=\"Kiểm tra\">" +
            "<toggleButton id=\"" + VisualQaId + "\" label=\"Nhìn tài liệu\" size=\"large\" imageMso=\"ZoomPrintPreviewExcel\"" +
            " getPressed=\"GetPressed\" getEnabled=\"GetEnabled\" onAction=\"OnToggleAction\" tag=\"visualQa\"" +
            " screentip=\"Cho AI nhìn tài liệu\" supertip=\"Bật: AI xem ảnh trang tài liệu để soát bố cục. Tốn nhiều token hơn" +
            " và cần model đọc được ảnh. Có hiệu lực từ lượt hỏi kế tiếp.\"/>" +
            "<button id=\"btnCheckLayout\" label=\"Soát bố cục\" size=\"large\" imageMso=\"SlideShowFromCurrent\" getVisible=\"GetVisible\"" +
            " onAction=\"OnButtonAction\" tag=\"checkLayout\" screentip=\"Soát bố cục\"" +
            " supertip=\"Tìm chữ tràn khung, khung ra ngoài slide, khung chồng nhau, chữ quá nhỏ. Chỉ đọc, không dùng AI.\"/>" +
            "<button id=\"btnCheckTables\" label=\"Soát bảng\" size=\"large\" imageMso=\"TableInsert\" getVisible=\"GetVisible\"" +
            " onAction=\"OnButtonAction\" tag=\"checkTables\" screentip=\"Soát bảng\"" +
            " supertip=\"Tìm ô trống và ô tiêu đề lẫn đoạn văn trong các bảng. Chỉ đọc, không dùng AI.\"/>" +
            "<button id=\"btnCheckRange\" label=\"Soát vùng dữ liệu\" size=\"large\" imageMso=\"DataValidation\" getVisible=\"GetVisible\"" +
            " onAction=\"OnButtonAction\" tag=\"checkRange\" screentip=\"Soát vùng dữ liệu\"" +
            " supertip=\"Tìm cột thiếu tiêu đề, số lưu dạng chữ, ô lỗi, dữ liệu lạc ngoài bảng. Chỉ đọc, không dùng AI.\"/>" +
            "</group>" +
            // Một tab duy nhất (người dùng chốt 13/10): màn hình quản trị ở nhóm "Quản lý", không mở tab thứ hai.
            // Hội thoại / nhật ký hoạt động nằm trong menu "⋯" của khung chat, không đặt ở đây.
            "<group id=\"axiomManageGroup\" label=\"Quản lý\">" +
            "<button id=\"btnMemory\" label=\"Bộ nhớ AI…\" size=\"large\" imageMso=\"RecordsSaveRecord\" onAction=\"OnButtonAction\" tag=\"memory\"" +
            " screentip=\"Bộ nhớ AI\" supertip=\"Xem, sửa, ghim, đặt hạn dùng, xoá những điều AI đã ghi nhớ về bạn và tài liệu.\"/>" +
            "<button id=\"btnSkills\" label=\"Kỹ năng…\" size=\"large\" imageMso=\"SmartArtInsert\" onAction=\"OnButtonAction\" tag=\"skills\"" +
            " screentip=\"Kỹ năng\" supertip=\"Các kỹ năng AI đang có cho ứng dụng này; nạp lại sau khi thêm hoặc sửa kỹ năng.\"/>" +
            "<button id=\"btnMcp\" label=\"Công cụ ngoài…\" size=\"large\" imageMso=\"HyperlinkInsert\" onAction=\"OnButtonAction\" tag=\"mcp\"" +
            " screentip=\"Công cụ ngoài (MCP)\" supertip=\"Máy chủ công cụ ngoài cho AI (tìm web, đọc file…): thêm, tắt, đặt tin cậy.\"/>" +
            "</group>" +
            "<group id=\"axiomSystemGroup\" label=\"Hệ thống\">" +
            "<button id=\"btnSetup\" label=\"Thiết lập…\" size=\"large\" imageMso=\"ControlWizards\" onAction=\"OnButtonAction\" tag=\"setup\"/>" +
            "<button id=\"btnStatus\" label=\"Trạng thái\" imageMso=\"Info\" onAction=\"OnButtonAction\" tag=\"status\"" +
            " screentip=\"Trạng thái\" supertip=\"Kiểm tra thật: add-in có trả lời không, Agent Core có chạy không, model đang dùng.\"/>" +
            "<button id=\"btnOpenLog\" label=\"Mở nhật ký\" imageMso=\"FileOpen\" onAction=\"OnButtonAction\" tag=\"openLog\"/>" +
            "<menu id=\"mnuMore\" label=\"Khác\" imageMso=\"PropertySheet\">" +
            "<button id=\"btnSettings\" label=\"Cài đặt nâng cao…\" imageMso=\"PropertySheet\" onAction=\"OnButtonAction\" tag=\"settings\"/>" +
            "<button id=\"btnTestModel\" label=\"Kiểm tra mô hình\" imageMso=\"AcceptInvitation\" onAction=\"OnButtonAction\" tag=\"testModel\"/>" +
            "<button id=\"btnCopyUrl\" label=\"Sao chép API URL\" imageMso=\"Copy\" onAction=\"OnButtonAction\" tag=\"copyUrl\"/>" +
            "<menuSeparator id=\"sepMore\"/>" +
            "<button id=\"btnStopCore\" label=\"Tắt Agent Core…\" imageMso=\"Delete\" onAction=\"OnButtonAction\" tag=\"stopCore\"/>" +
            "</menu>" +
            "</group>" +
            "</tab>" +
            "</tabs></ribbon>" +
            "</customUI>";

        private static object _ribbon;
        // Trạng thái Core báo lại sau lần bấm gần nhất (null = chưa hỏi): nút hiện trạng thái thật của Core,
        // không chỉ giá trị vừa ghi HKCU (biến môi trường AXIOM_VISUAL_QA của Core đè được HKCU).
        private static bool? _coreVisualQa;
        private static int _checkRunning;

        public static void OnLoad(object ribbon)
        {
            _ribbon = ribbon;
        }

        public static bool GetVisible(Connect connect, string tag)
        {
            RibbonChecks.CheckInfo check;
            if (RibbonChecks.Checks.TryGetValue(tag, out check))
            {
                return connect.AppKind == check.Kind || connect.AppKind == "unknown";
            }
            return true;
        }

        public static bool GetEnabled(string tag)
        {
            // Agent in-process không dùng ảnh: tắt Core thì nút này không có tác dụng.
            return tag != "visualQa" || Config.CoreEnabled;
        }

        public static bool GetPressed(string tag)
        {
            if (tag != "visualQa")
            {
                return false;
            }
            bool? core = _coreVisualQa;
            return core.HasValue ? core.Value : Config.VisualQaEnabled;
        }

        public static void HandleToggle(Connect connect, string tag, bool pressed)
        {
            if (tag != "visualQa")
            {
                Logger.Info("Ribbon: unknown toggle tag '" + tag + "'");
                return;
            }
            if (!Config.WriteDword("VisualQaEnabled", pressed ? 1 : 0))
            {
                // Ghi hỏng thì vẽ lại theo HKCU: nút không được hiện trạng thái chưa lưu.
                Invalidate(connect, VisualQaId);
                MessageBox.Show("Không lưu được thiết lập \"Nhìn tài liệu\" (HKCU\\Software\\AxiomOffice).",
                    "Axiom Office", MessageBoxButtons.OK, MessageBoxIcon.Warning);
                return;
            }
            _coreVisualQa = null;
            Logger.Info("Ribbon: VisualQaEnabled=" + (pressed ? 1 : 0));
            // Hỏi lại Core ở luồng nền (có thể phải khởi động Core): đọc trạng thái thật, không đoán.
            var thread = new Thread(() => ConfirmVisualQa(connect, pressed)) { IsBackground = true, Name = "AxiomRibbonVisualQa" };
            thread.Start();
        }

        private static void ConfirmVisualQa(Connect connect, bool wanted)
        {
            try
            {
                string error;
                Ai.SetupInfo info = Ai.CoreClient.Instance.Setup(out error);
                if (info == null || !info.Current.ContainsKey("visualQaEnabled"))
                {
                    // Core chưa chạy được: lượt hỏi kế tiếp Core khởi động và đọc HKCU vừa ghi.
                    Logger.Info("Ribbon: VisualQa not confirmed by Core (" + error + ")");
                    return;
                }
                bool actual = Ai.SetupInfo.Flag("visualQaEnabled", info.Current, wanted);
                _coreVisualQa = actual;
                if (actual == wanted)
                {
                    Logger.Info("Ribbon: Core confirmed visualQaEnabled=" + actual);
                    return;
                }
                Logger.Info("Ribbon: Core reports visualQaEnabled=" + actual + ", wrote " + wanted);
                Invalidate(connect, VisualQaId);
                connect.OnUiThread(() => MessageBox.Show(
                    "Đã lưu thiết lập nhưng Agent Core vẫn đang " + (actual ? "BẬT" : "TẮT") + " \"Nhìn tài liệu\".\n\n"
                    + "Thường do biến môi trường AXIOM_VISUAL_QA đang đặt cho Core; bỏ biến đó rồi bấm lại.",
                    "Axiom Office", MessageBoxButtons.OK, MessageBoxIcon.Warning));
            }
            catch (Exception ex)
            {
                Logger.Error("Ribbon: confirm VisualQa failed", ex);
            }
        }

        private static void Invalidate(Connect connect, string controlId)
        {
            object ribbon = _ribbon;
            if (ribbon == null)
            {
                return;
            }
            try
            {
                connect.OnUiThread(() =>
                {
                    dynamic ui = ribbon;
                    ui.InvalidateControl(controlId);
                    return true;
                });
            }
            catch (Exception ex)
            {
                Logger.Error("Ribbon invalidate failed", ex);
            }
        }

        // Wizard / cài đặt có thể đổi VisualQaEnabled, CoreEnabled: vẽ lại để nút toggle khớp HKCU.
        private static void RefreshAfterSettings(Connect connect)
        {
            _coreVisualQa = null;
            Invalidate(connect, VisualQaId);
        }

        // Lệnh soát chạy ở luồng nền như lệnh /cmd: luồng UI rảnh để host trả lời các cuộc gọi COM.
        private static void RunCheck(Connect connect, string tag)
        {
            if (Interlocked.Exchange(ref _checkRunning, 1) == 1)
            {
                return;
            }
            RibbonChecks.CheckInfo check = RibbonChecks.Checks[tag];
            var thread = new Thread(() =>
            {
                string text;
                bool clean = false;
                try
                {
                    if (connect.AppKind != check.Kind)
                    {
                        text = "Nút này dùng cho " + KindName(check.Kind) + "; tài liệu đang mở là " + KindName(connect.AppKind) + ".";
                    }
                    else
                    {
                        object response = CommandDispatcher.Execute(connect, check.Action, new Dictionary<string, object>());
                        text = RibbonChecks.Describe(tag, response, out clean);
                    }
                }
                catch (Exception ex)
                {
                    Logger.Error("Ribbon check failed: " + check.Action, ex);
                    text = "Không soát được: " + ex.Message;
                }
                finally
                {
                    Interlocked.Exchange(ref _checkRunning, 0);
                }
                Logger.Info("Ribbon: " + check.Action + " clean=" + clean);
                connect.OnUiThread(() => MessageBox.Show(text, "Axiom Office - " + check.Title, MessageBoxButtons.OK,
                    clean ? MessageBoxIcon.Information : MessageBoxIcon.Warning));
            }) { IsBackground = true, Name = "AxiomRibbonCheck" };
            thread.Start();
        }

        private static string KindName(string kind)
        {
            switch (kind)
            {
                case "wps": return "văn bản";
                case "et": return "bảng tính";
                case "wpp": return "trình chiếu";
                default: return "ứng dụng chưa nhận ra";
            }
        }

        public static void HandleButton(Connect connect, string tag)
        {
            if (RibbonChecks.Checks.ContainsKey(tag))
            {
                RunCheck(connect, tag);
                return;
            }
            switch (tag)
            {
                case "status":
                    ShowStatus(connect);
                    break;
                case "copyUrl":
                    CopyUrl(connect);
                    break;
                case "openLog":
                    OpenLog();
                    break;
                case "askAi":
                    connect.ShowAskAiPane();
                    break;
                case "setup":
                    // Wizard thiet lap cho nguoi dung khong chuyen (New_arch.md muc 9) - cung noi dung voi
                    // ban Linux (menu "Thiet lap..." + "Cai dat nang cao..."). Cau hinh nang cao cung nam
                    // trong buoc 4 cua wizard.
                    using (var form = new Ai.SetupWizardForm(connect))
                    {
                        form.ShowDialog();
                    }
                    RefreshAfterSettings(connect);
                    break;
                case "settings":
                    using (var form = new Ai.SettingsForm())
                    {
                        form.ShowDialog();
                    }
                    RefreshAfterSettings(connect);
                    break;
                case "memory":
                    using (var form = new Ai.MemoryForm(Ai.AskAiPane.DocumentKey(connect)))
                    {
                        form.ShowDialog();
                    }
                    break;
                case "skills":
                    using (var form = new Ai.SkillsForm(connect.AppKind))
                    {
                        form.ShowDialog();
                    }
                    break;
                case "mcp":
                    using (var form = new Ai.McpForm())
                    {
                        form.ShowDialog();
                    }
                    break;
                case "testModel":
                    RunInBackground("AxiomRibbonTestModel", () => TestModel(connect));
                    break;
                case "stopCore":
                    StopCore();
                    break;
                default:
                    Logger.Info("Ribbon: unknown button tag '" + tag + "'");
                    break;
            }
        }

        private static void RunInBackground(string name, Action action)
        {
            var thread = new Thread(() =>
            {
                try
                {
                    action();
                }
                catch (Exception ex)
                {
                    Logger.Error("Ribbon " + name + " failed", ex);
                }
            }) { IsBackground = true, Name = name };
            thread.Start();
        }

        // Nút Trạng thái: kiểm THẬT từng tầng (docs/ribbon-ui.md §6) - gọi /health của bridge trong add-in qua HTTP,
        // hỏi /health của Core đang chạy (không khởi động Core chỉ để xem), đếm các phiên Office đã đăng ký.
        // Chạy ở luồng nền: mỗi lần hỏi có hạn chờ, luồng UI không được đứng.
        private static void ShowStatus(Connect connect)
        {
            RunInBackground("AxiomRibbonStatus", () =>
            {
                bool healthy;
                string text = StatusReport(connect, out healthy);
                connect.OnUiThread(() => MessageBox.Show(text, "Axiom Office - Trạng thái", MessageBoxButtons.OK,
                    healthy ? MessageBoxIcon.Information : MessageBoxIcon.Warning));
            });
        }

        private static string StatusReport(Connect connect, out bool healthy)
        {
            string kind = connect.AppKind;
            int port = Config.PortForKind(kind, connect.IsOfficeHost);
            var sb = new StringBuilder();
            healthy = true;

            string bridgeError;
            Dictionary<string, object> bridge = GetHealth("http://127.0.0.1:" + port + "/health", out bridgeError);
            if (bridge != null)
            {
                sb.AppendLine("✓ Add-in: đang trả lời ở cổng " + port + " (phiên bản " + Field(bridge, "version") + ").");
            }
            else
            {
                healthy = false;
                sb.AppendLine("✗ Add-in: không trả lời ở cổng " + port + " (" + bridgeError + ").");
                sb.AppendLine("   AI sẽ không đọc/sửa được tài liệu này. Đóng và mở lại ứng dụng.");
            }

            if (!Config.CoreEnabled)
            {
                sb.AppendLine("• Agent Core: đang tắt trong cài đặt (CoreEnabled = 0) - AI chạy trong add-in.");
            }
            else
            {
                string coreError;
                Dictionary<string, object> core = Ai.CoreClient.Instance.RunningHealth(out coreError);
                if (core != null)
                {
                    sb.AppendLine("✓ Agent Core: đang chạy (phiên bản " + Field(core, "version") + ", pid " + Field(core, "pid")
                        + ", đã chạy " + Uptime(Field(core, "uptimeSeconds")) + ").");
                    string model = Field(core, "model");
                    sb.AppendLine("   Model: " + (model.Length > 0 ? model : "(chưa chọn - bấm Thiết lập…)"));
                    string memoryError = Field(core, "memoryError");
                    if (memoryError.Length > 0)
                    {
                        healthy = false;
                        sb.AppendLine("   ✗ Bộ nhớ AI lỗi: " + memoryError);
                    }
                }
                else
                {
                    sb.AppendLine("• Agent Core: chưa chạy. Bình thường nếu chưa hỏi AI lần nào - Core tự khởi động ở lượt hỏi đầu.");
                    sb.AppendLine("   (" + coreError + ")");
                }
            }

            List<string> sessions = Sessions();
            sb.AppendLine("• Ứng dụng Office đang kết nối: " + (sessions.Count == 0 ? "không có" : string.Join(", ", sessions.ToArray())));
            sb.AppendLine();
            sb.AppendLine("Nhật ký: " + Logger.LogFilePath);
            return sb.ToString().TrimEnd();
        }

        private static Dictionary<string, object> GetHealth(string url, out string error)
        {
            error = null;
            try
            {
                var request = (System.Net.HttpWebRequest)System.Net.WebRequest.Create(url);
                request.Timeout = 2000;
                request.ReadWriteTimeout = 2000;
                using (var response = request.GetResponse())
                using (var reader = new System.IO.StreamReader(response.GetResponseStream(), Encoding.UTF8))
                {
                    var reply = new System.Web.Script.Serialization.JavaScriptSerializer()
                        .Deserialize<Dictionary<string, object>>(reader.ReadToEnd());
                    object result;
                    if (reply != null && reply.TryGetValue("result", out result) && result is Dictionary<string, object>)
                    {
                        return (Dictionary<string, object>)result;
                    }
                    error = "trả lời không hợp lệ";
                }
            }
            catch (Exception ex)
            {
                error = ex.Message;
            }
            return null;
        }

        // sessions/{pid}.json do SessionRegistry ghi; chỉ liệt kê tiến trình còn sống.
        private static List<string> Sessions()
        {
            var list = new List<string>();
            try
            {
                if (!System.IO.Directory.Exists(SessionRegistry.SessionsDir))
                {
                    return list;
                }
                var serializer = new System.Web.Script.Serialization.JavaScriptSerializer();
                foreach (string path in System.IO.Directory.GetFiles(SessionRegistry.SessionsDir, "*.json"))
                {
                    try
                    {
                        var info = serializer.Deserialize<Dictionary<string, object>>(System.IO.File.ReadAllText(path, Encoding.UTF8));
                        int pid;
                        if (!int.TryParse(System.IO.Path.GetFileNameWithoutExtension(path), out pid) || !ProcessAlive(pid))
                        {
                            continue;
                        }
                        string app = KindName(Field(info, "app"));
                        list.Add(app + " (cổng " + Field(info, "port") + ")");
                    }
                    catch (Exception)
                    {
                    }
                }
            }
            catch (Exception ex)
            {
                Logger.Error("Ribbon status: sessions unreadable", ex);
            }
            return list;
        }

        private static bool ProcessAlive(int pid)
        {
            try
            {
                using (Process process = Process.GetProcessById(pid))
                {
                    return !process.HasExited;
                }
            }
            catch (Exception)
            {
                return false;
            }
        }

        private static string Field(Dictionary<string, object> map, string key)
        {
            object value;
            return map != null && map.TryGetValue(key, out value) && value != null
                ? Convert.ToString(value, System.Globalization.CultureInfo.InvariantCulture)
                : "";
        }

        private static string Uptime(string seconds)
        {
            double value;
            if (!double.TryParse(seconds, System.Globalization.NumberStyles.Float, System.Globalization.CultureInfo.InvariantCulture, out value))
            {
                return "?";
            }
            TimeSpan span = TimeSpan.FromSeconds(value);
            return span.TotalHours >= 1 ? (int)span.TotalHours + " giờ " + span.Minutes + " phút"
                : span.TotalMinutes >= 1 ? (int)span.TotalMinutes + " phút" : (int)span.TotalSeconds + " giây";
        }

        // Kiểm tra mô hình: gửi thử một câu tới model ĐÃ LƯU (apiKey rỗng = Core dùng khoá đã lưu).
        private static void TestModel(Connect connect)
        {
            if (string.IsNullOrEmpty(Config.LlmEndpoint) || string.IsNullOrEmpty(Config.LlmModel))
            {
                connect.OnUiThread(() => MessageBox.Show("Chưa cấu hình AI. Bấm Thiết lập… để chọn nhà cung cấp và model.",
                    "Axiom Office - Kiểm tra mô hình", MessageBoxButtons.OK, MessageBoxIcon.Information));
                return;
            }
            string error;
            Ai.SetupInfo info = Ai.CoreClient.Instance.Setup(out error);
            string providerId = info != null ? info.ProviderId : "";
            Ai.LlmProbe probe = Ai.CoreClient.Instance.TestLlm(providerId, Config.LlmProvider, Config.LlmEndpoint, Config.LlmModel, "",
                CancellationToken.None);
            Logger.Info("Ribbon: test model ok=" + probe.Ok + " kind=" + probe.Kind);
            string text = probe.Ok
                ? "Kết nối được với " + (string.IsNullOrEmpty(probe.Model) ? Config.LlmModel : probe.Model)
                    + " (" + probe.Seconds.ToString("0.0") + " giây).\n\nModel trả lời: " + probe.Reply
                : probe.Message + (string.IsNullOrEmpty(probe.Hint) ? "" : "\n\n" + probe.Hint)
                    + (string.IsNullOrEmpty(probe.Detail) ? "" : "\n\nChi tiết: " + probe.Detail);
            connect.OnUiThread(() => MessageBox.Show(text, "Axiom Office - Kiểm tra mô hình", MessageBoxButtons.OK,
                probe.Ok ? MessageBoxIcon.Information : MessageBoxIcon.Warning));
        }

        private static void StopCore()
        {
            DialogResult answer = MessageBox.Show(
                "Tắt Agent Core?\n\nLượt AI đang chạy (nếu có) sẽ bị dừng giữa chừng. Lần hỏi AI sau Core tự khởi động lại.\n"
                + "Chỉ cần khi Core bị treo hoặc trước khi cập nhật Axiom Office.",
                "Axiom Office - Tắt Agent Core", MessageBoxButtons.YesNo, MessageBoxIcon.Warning, MessageBoxDefaultButton.Button2);
            if (answer != DialogResult.Yes)
            {
                return;
            }
            string error;
            bool ok = Ai.CoreClient.Instance.ShutdownCore(out error);
            Logger.Info("Ribbon: stop core ok=" + ok + (ok ? "" : " (" + error + ")"));
            MessageBox.Show(ok ? "Đã yêu cầu Agent Core dừng." : "Không tắt được: " + error,
                "Axiom Office - Tắt Agent Core", MessageBoxButtons.OK, ok ? MessageBoxIcon.Information : MessageBoxIcon.Warning);
        }

        private static void CopyUrl(Connect connect)
        {
            int port = Config.PortForKind(connect.AppKind, connect.IsOfficeHost);
            string url = "http://127.0.0.1:" + port + "/";
            try
            {
                Clipboard.SetText(url);
                Logger.Info("Ribbon: copied " + url);
                MessageBox.Show("Đã chép vào clipboard:\n" + url, "Axiom Office", MessageBoxButtons.OK, MessageBoxIcon.Information);
            }
            catch (Exception ex)
            {
                Logger.Error("Ribbon copy failed", ex);
            }
        }

        private static void OpenLog()
        {
            try
            {
                Process.Start(Logger.LogFilePath);
            }
            catch (Exception ex)
            {
                Logger.Error("Ribbon open log failed", ex);
                MessageBox.Show("Không mở được file nhật ký:\n" + Logger.LogFilePath, "Axiom Office", MessageBoxButtons.OK, MessageBoxIcon.Warning);
            }
        }
    }
}

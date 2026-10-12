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
            "<group id=\"axiomQuickGroup\" label=\"Truy cập nhanh\">" +
            "<button id=\"btnSetup\" label=\"Thiết lập…\" size=\"large\" imageMso=\"ControlWizards\" onAction=\"OnButtonAction\" tag=\"setup\"/>" +
            "<button id=\"btnStatus\" label=\"Trạng thái\" imageMso=\"Info\" onAction=\"OnButtonAction\" tag=\"status\"/>" +
            "<button id=\"btnOpenLog\" label=\"Mở nhật ký\" imageMso=\"FileOpen\" onAction=\"OnButtonAction\" tag=\"openLog\"/>" +
            // Hai nút dưới chuyển sang tab "Axiom · Quản lý" khi tab đó mở (đợt 3).
            "<button id=\"btnSettings\" label=\"Cài đặt nâng cao…\" imageMso=\"PropertySheet\" onAction=\"OnButtonAction\" tag=\"settings\"/>" +
            "<button id=\"btnCopyUrl\" label=\"Sao chép API URL\" imageMso=\"Copy\" onAction=\"OnButtonAction\" tag=\"copyUrl\"/>" +
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
                default:
                    Logger.Info("Ribbon: unknown button tag '" + tag + "'");
                    break;
            }
        }

        private static void ShowStatus(Connect connect)
        {
            string kind = connect.AppKind;
            int port = Config.PortForKind(kind, connect.IsOfficeHost);
            var sb = new StringBuilder();
            sb.AppendLine("Axiom Office (in-proc add-in)");
            sb.AppendLine();
            sb.AppendLine("Application: " + kind);
            sb.AppendLine("HTTP port:   " + port);
            sb.AppendLine("API base:    http://127.0.0.1:" + port + "/");
            sb.AppendLine("Health:      http://127.0.0.1:" + port + "/health");
            sb.AppendLine();
            sb.AppendLine("Log file: " + Logger.LogFilePath);
            MessageBox.Show(sb.ToString(), "Axiom Office", MessageBoxButtons.OK, MessageBoxIcon.Information);
        }

        private static void CopyUrl(Connect connect)
        {
            int port = Config.PortForKind(connect.AppKind, connect.IsOfficeHost);
            string url = "http://127.0.0.1:" + port + "/";
            try
            {
                Clipboard.SetText(url);
                Logger.Info("Ribbon: copied " + url);
                MessageBox.Show("Copied to clipboard:\n" + url, "Axiom Office", MessageBoxButtons.OK, MessageBoxIcon.Information);
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
                MessageBox.Show("Could not open log file:\n" + Logger.LogFilePath, "Axiom Office", MessageBoxButtons.OK, MessageBoxIcon.Warning);
            }
        }
    }
}

using System;
using System.Diagnostics;
using System.Text;
using System.Windows.Forms;
using AxiomOffice.Bridge;

namespace AxiomOffice.Ribbon
{
    internal static class RibbonUi
    {
        public const string RibbonXml =
            "<customUI xmlns=\"http://schemas.microsoft.com/office/2006/01/customui\">" +
            "<ribbon><tabs>" +
            "<tab id=\"axiomOfficeTab\" label=\"Axiom Office\">" +
            "<group id=\"axiomOfficeGroup\" label=\"Local bridge\">" +
            "<button id=\"btnStatus\" label=\"Status\" size=\"large\" onAction=\"OnButtonAction\" tag=\"status\"/>" +
            "<button id=\"btnCopyUrl\" label=\"Copy API URL\" size=\"large\" onAction=\"OnButtonAction\" tag=\"copyUrl\"/>" +
            "<button id=\"btnOpenLog\" label=\"Open Log\" size=\"large\" onAction=\"OnButtonAction\" tag=\"openLog\"/>" +
            "</group>" +
            "<group id=\"axiomOfficeAiGroup\" label=\"AI\">" +
            "<button id=\"btnAskAi\" label=\"Ask AI...\" size=\"large\" onAction=\"OnButtonAction\" tag=\"askAi\"/>" +
            "<button id=\"btnSettings\" label=\"Settings\" size=\"large\" onAction=\"OnButtonAction\" tag=\"settings\"/>" +
            "</group>" +
            "</tab>" +
            "</tabs></ribbon>" +
            "</customUI>";

        public static void HandleButton(Connect connect, string tag)
        {
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
                case "settings":
                    using (var form = new Ai.SettingsForm())
                    {
                        form.ShowDialog();
                    }
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

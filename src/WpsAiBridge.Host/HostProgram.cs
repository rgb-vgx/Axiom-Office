using System;
using System.Threading;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Host
{
    internal sealed class AutomationAppHost : IAppHost
    {
        private readonly object _app;
        private readonly string _kind;
        private readonly bool _office;

        public AutomationAppHost(object app, string kind, bool office)
        {
            _app = app;
            _kind = kind;
            _office = office;
        }

        public object Application
        {
            get { return _app; }
        }

        public string AppKind
        {
            get { return _kind; }
        }

        public bool IsOfficeHost
        {
            get { return _office; }
        }
    }

    internal static class HostProgram
    {
        [STAThread]
        private static int Main(string[] args)
        {
            string kind = args.Length > 0 ? args[0].ToLowerInvariant() : "";

            if (kind == "llm-test")
            {
                return RunLlmTest();
            }

            string progId;
            string logicalKind;
            bool office;
            switch (kind)
            {
                case "wps":
                    progId = "KWPS.Application";
                    logicalKind = "wps";
                    office = false;
                    break;
                case "et":
                    progId = "KET.Application";
                    logicalKind = "et";
                    office = false;
                    break;
                case "wpp":
                    progId = "KWPP.Application";
                    logicalKind = "wpp";
                    office = false;
                    break;
                case "word":
                    progId = "Word.Application";
                    logicalKind = "wps";
                    office = true;
                    break;
                case "excel":
                    progId = "Excel.Application";
                    logicalKind = "et";
                    office = true;
                    break;
                case "ppt":
                case "powerpoint":
                    progId = "PowerPoint.Application";
                    logicalKind = "wpp";
                    office = true;
                    break;
                default:
                    Console.Error.WriteLine("usage: WpsAiBridge.Host.exe wps|et|wpp|word|excel|ppt [--visible]");
                    Console.Error.WriteLine("  wps|et|wpp : WPS Office components (KWPS/KET/KWPP.Application)");
                    Console.Error.WriteLine("  word|excel|ppt : Microsoft Office (Word/Excel/PowerPoint.Application)");
                    return 2;
            }

            bool visible = Array.IndexOf(args, "--visible") >= 0;
            Logger.Info("Companion host starting for " + kind + " (progid=" + progId + ", office=" + office + ", visible=" + visible + ")");

            Type progType = Type.GetTypeFromProgID(progId, true);
            if (progType == null)
            {
                Console.Error.WriteLine("ProgID not found: " + progId);
                return 3;
            }

            object app = Activator.CreateInstance(progType);
            dynamic dynamicApp = app;
            string resolvedName = "";
            try
            {
                resolvedName = (Convert.ToString(dynamicApp.Name) + " " + Convert.ToString(dynamicApp.Version)).Trim();
            }
            catch
            {
            }
            Logger.Info("Companion resolved application: " + resolvedName + " (progid=" + progId + ")");
            Console.WriteLine("Companion resolved application: " + resolvedName);
            try
            {
                dynamicApp.Visible = visible;
            }
            catch
            {
            }
            try
            {
                dynamicApp.DisplayAlerts = false;
            }
            catch
            {
            }

            var host = new AutomationAppHost(app, logicalKind, office);
            var bridge = new HttpBridge(host);
            int port = Config.PortForKind(logicalKind, office);
            try
            {
                bridge.Start();
                Console.WriteLine("WpsAiBridge.Host ready: kind=" + kind + " port=" + port);
                Logger.Info("Companion host listening: kind=" + kind + " port=" + port);
            }
            catch (Exception ex)
            {
                bridge = null;
                Console.WriteLine("Port " + port + " is already served (in-process add-in is active). Companion idle.");
                Logger.Error("Companion bind failed; assuming in-process add-in serves port " + port, ex);
            }

            var stop = new ManualResetEvent(false);
            Console.CancelKeyPress += delegate(object sender, ConsoleCancelEventArgs e)
            {
                e.Cancel = true;
                stop.Set();
            };
            stop.WaitOne();

            if (bridge != null)
            {
                bridge.Stop();
            }
            try
            {
                dynamicApp.Quit();
            }
            catch
            {
            }
            return 0;
        }

        private static int RunLlmTest()
        {
            Console.WriteLine("Provider: " + Config.LlmProvider);
            Console.WriteLine("Endpoint: " + Config.LlmEndpoint);
            Console.WriteLine("Model:    " + Config.LlmModel);
            Console.WriteLine("ApiKey:   " + (string.IsNullOrEmpty(Config.LlmApiKey) ? "(empty)" : "***set***"));
            Ai.LlmResult result = Ai.LlmClient.Chat("You are a connectivity test.", "Reply with a single word: OK");
            if (result.Ok)
            {
                Console.WriteLine("OK (" + result.Seconds.ToString("0.0") + "s): " + (result.Text ?? "").Trim());
                return 0;
            }
            Console.WriteLine("ERROR: " + result.Error);
            return 1;
        }
    }
}

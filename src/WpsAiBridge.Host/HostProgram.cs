using System;
using System.Threading;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Host
{
    internal sealed class AutomationAppHost : IAppHost
    {
        private readonly object _app;
        private readonly string _kind;

        public AutomationAppHost(object app, string kind)
        {
            _app = app;
            _kind = kind;
        }

        public object Application
        {
            get { return _app; }
        }

        public string AppKind
        {
            get { return _kind; }
        }
    }

    internal static class HostProgram
    {
        [STAThread]
        private static int Main(string[] args)
        {
            string kind = args.Length > 0 ? args[0].ToLowerInvariant() : "";
            string progId;
            switch (kind)
            {
                case "wps":
                    progId = "KWPS.Application";
                    break;
                case "et":
                    progId = "KET.Application";
                    break;
                case "wpp":
                    progId = "KWPP.Application";
                    break;
                default:
                    Console.Error.WriteLine("usage: WpsAiBridge.Host.exe wps|et|wpp [--visible]");
                    return 2;
            }

            bool visible = Array.IndexOf(args, "--visible") >= 0;
            Logger.Info("Companion host starting for " + kind + " (progid=" + progId + ", visible=" + visible + ")");

            Type progType = Type.GetTypeFromProgID(progId, true);
            if (progType == null)
            {
                Console.Error.WriteLine("ProgID not found: " + progId);
                return 3;
            }

            object app = Activator.CreateInstance(progType);
            dynamic dynamicApp = app;
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

            var host = new AutomationAppHost(app, kind);
            var bridge = new HttpBridge(host);
            int port = Config.PortForKind(kind);
            try
            {
                bridge.Start();
                Console.WriteLine("WpsAiBridge.Host ready: app=" + kind + " port=" + port);
                Logger.Info("Companion host listening: app=" + kind + " port=" + port);
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
    }
}

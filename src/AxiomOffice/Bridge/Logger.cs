using System;
using System.IO;
using System.Text;

namespace AxiomOffice.Bridge
{
    internal static class Logger
    {
        private static readonly object Sync = new object();
        private static readonly string LogDir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "AxiomOffice");
        private static readonly string LogPath = Path.Combine(LogDir, "bridge.log");

        public static string LogFilePath
        {
            get { return LogPath; }
        }

        public static void Info(string message)
        {
            Write("INFO", message, null);
        }

        public static void Error(string message, Exception ex)
        {
            Write("ERROR", message, ex);
        }

        private static void Write(string level, string message, Exception ex)
        {
            try
            {
                lock (Sync)
                {
                    Directory.CreateDirectory(LogDir);
                    var sb = new StringBuilder();
                    sb.Append(DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff"));
                    sb.Append(" [").Append(level).Append("]");
                    sb.Append(" [pid ").Append(System.Diagnostics.Process.GetCurrentProcess().Id).Append("] ");
                    sb.Append(message);
                    if (ex != null)
                    {
                        sb.Append(" => ").Append(ex.GetType().Name).Append(": ").Append(ex.Message);
                        if (!string.IsNullOrEmpty(ex.StackTrace))
                        {
                            sb.Append(Environment.NewLine).Append(ex.StackTrace);
                        }
                    }
                    File.AppendAllText(LogPath, sb.AppendLine().ToString(), Encoding.UTF8);
                }
            }
            catch
            {
            }
        }
    }
}

using System.Globalization;
using System.Text;

namespace AxiomOffice.Core.Logging;

// Log cua Core: %LOCALAPPDATA%\AxiomOffice\core.log, cung dinh dang bridge.log
// (thoi gian, muc, [pid N]); xoay khi > 10MB, giu 3 file (New_arch.md muc 8.9).
public static class CoreLog
{
    private static readonly Lock Gate = new();
    private static string? _path;
    private static long _maxBytes = 10L * 1024 * 1024;
    private static int _keep = 3;

    public static string? FilePath
    {
        get
        {
            lock (Gate)
            {
                return _path;
            }
        }
    }

    public static void Init(string path, long maxBytes = 10L * 1024 * 1024, int keep = 3)
    {
        lock (Gate)
        {
            _path = path;
            _maxBytes = maxBytes;
            _keep = Math.Max(1, keep);
        }
    }

    public static void Info(string message)
    {
        Write("INFO", message);
    }

    public static void Error(string message, Exception? exception = null)
    {
        Write("ERROR", exception == null ? message : message + ": " + exception.GetType().Name + ": " + exception.Message);
    }

    private static void Write(string level, string message)
    {
        string line = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff", CultureInfo.InvariantCulture)
            + " [" + level + "] [pid " + Environment.ProcessId + "] " + message;

        lock (Gate)
        {
            if (_path != null)
            {
                try
                {
                    RotateIfNeeded();
                    File.AppendAllText(_path, line + Environment.NewLine, new UTF8Encoding(false));
                }
                catch
                {
                    // Khong ghi duoc log (dia day, file bi khoa): bo qua, khong duoc lam Core dung.
                }
            }
        }

        // Khi chay tu terminal (khong bi redirect) thi in ra cho de theo doi; add-in khoi dong Core
        // voi output bi redirect nen khong ton them I/O.
        if (!Console.IsOutputRedirected)
        {
            Console.WriteLine(line);
        }
    }

    private static void RotateIfNeeded()
    {
        if (_path == null)
        {
            return;
        }

        var file = new FileInfo(_path);
        if (!file.Exists || file.Length < _maxBytes)
        {
            return;
        }

        // core.log -> core.log.1 -> ... -> core.log.(keep-1): keep la TONG so file giu lai.
        if (_keep <= 1)
        {
            File.Delete(_path);
            return;
        }

        string oldest = _path + "." + (_keep - 1);
        if (File.Exists(oldest))
        {
            File.Delete(oldest);
        }

        for (int i = _keep - 2; i >= 1; i--)
        {
            string source = _path + "." + i;
            if (File.Exists(source))
            {
                File.Move(source, _path + "." + (i + 1), overwrite: true);
            }
        }

        File.Move(_path, _path + ".1", overwrite: true);
    }
}

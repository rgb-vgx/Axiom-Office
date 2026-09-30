using System.Text.Json.Nodes;

namespace AxiomOffice.Core.Office;

// Mot bridge dang song (file trong %LOCALAPPDATA%\AxiomOffice\sessions\{pid}.json - muc 7.1).
public sealed record OfficeSession(
    int Pid,
    string App,
    string Family,
    int Port,
    string Host,
    string Version,
    double AgeSeconds,
    string? Document,
    string? DocumentPath,
    string File)
{
    public static OfficeSession? Parse(JsonNode? node, string file)
    {
        if (node == null)
        {
            return null;
        }

        int pid = node["pid"]?.GetValue<int>() ?? 0;
        int port = node["port"]?.GetValue<int>() ?? 0;
        if (pid <= 0 || port <= 0)
        {
            return null;
        }

        double lastSeenEpoch = node["lastSeenEpoch"]?.GetValue<double>() ?? 0;
        double age = lastSeenEpoch > 0
            ? Math.Max(0, DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() / 1000.0 - lastSeenEpoch)
            : 0;

        return new OfficeSession(
            Pid: pid,
            App: node["app"]?.GetValue<string>() ?? "",
            Family: node["family"]?.GetValue<string>() ?? "",
            Port: port,
            Host: node["host"]?.GetValue<string>() ?? "",
            Version: node["version"]?.GetValue<string>() ?? "",
            AgeSeconds: Math.Round(age, 1),
            Document: node["document"]?.GetValue<string>(),
            DocumentPath: node["documentPath"]?.GetValue<string>(),
            File: file);
    }

    public string Describe()
    {
        string document = string.IsNullOrEmpty(Document) ? "(khong co tai lieu)" : Document;
        return $"{App}/{Family} port {Port} pid {Pid} - {document}";
    }
}

// Doc thu muc session registry. Khong xoa file cua nguoi khac (Host.exe lam viec prune); chi loc
// theo tuoi va theo tien trinh con song.
public sealed class SessionDirectory(string directory)
{
    public const double StaleSeconds = 90;

    // Windows: %LOCALAPPDATA%\AxiomOffice\sessions (add-in). Linux: $XDG_RUNTIME_DIR/axiom-office/sessions
    // (khong co thi ~/.cache/axiom-office/sessions) - dung cho extension LibreOffice ghi.
    public static string DefaultDirectory
    {
        get
        {
            if (OperatingSystem.IsWindows())
            {
                return Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "AxiomOffice", "sessions");
            }

            string? runtime = Environment.GetEnvironmentVariable("XDG_RUNTIME_DIR");
            string root = string.IsNullOrWhiteSpace(runtime)
                ? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".cache")
                : runtime;
            return Path.Combine(root, "axiom-office", "sessions");
        }
    }

    public string Directory => directory;

    public IReadOnlyList<OfficeSession> List(bool includeStale = false)
    {
        var found = new List<OfficeSession>();
        string dir = directory;
        if (!System.IO.Directory.Exists(dir))
        {
            return found;
        }

        foreach (string path in System.IO.Directory.GetFiles(dir, "*.json"))
        {
            OfficeSession? session;
            try
            {
                session = OfficeSession.Parse(JsonNode.Parse(System.IO.File.ReadAllText(path)), path);
            }
            catch
            {
                // File dang duoc ghi lai: bo qua.
                continue;
            }

            if (session == null || !IsAlive(session.Pid))
            {
                continue;
            }

            if (!includeStale && session.AgeSeconds > StaleSeconds)
            {
                continue;
            }

            found.Add(session);
        }

        found.Sort((a, b) => a.Port.CompareTo(b.Port));
        return found;
    }

    public OfficeSession? Find(int port)
    {
        foreach (OfficeSession session in List())
        {
            if (session.Port == port)
            {
                return session;
            }
        }

        return null;
    }

    public static bool IsAlive(int pid)
    {
        if (pid <= 0)
        {
            return false;
        }

        try
        {
            using var process = System.Diagnostics.Process.GetProcessById(pid);
            return !process.HasExited;
        }
        catch (ArgumentException)
        {
            return false;
        }
        catch (Exception)
        {
            // Ton tai nhung khong mo duoc (quyen): coi nhu con song.
            return true;
        }
    }
}

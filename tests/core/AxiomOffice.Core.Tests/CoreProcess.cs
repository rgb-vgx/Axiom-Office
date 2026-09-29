using System.Diagnostics;
using System.Text;
using AxiomOffice.Core.Config;

namespace AxiomOffice.Core.Tests;

// Khoi dong AxiomOffice.Core.exe THAT (khong dung TestServer) de test dung vong doi da tai lieu hoa:
// Core ghi core.json -> doc port tu do -> goi HTTP. Moi test dung thu muc du lieu tam, port do he
// dieu hanh cap va ten mutex rieng nen khong dung vao Core that cua nguoi dung (New_arch.md muc 7.1).
internal sealed class CoreProcess : IDisposable
{
    public const string Token = "test-token-0123456789";

    private readonly Process _process;
    private readonly StringBuilder _output;
    private readonly string _mutexName;
    private readonly bool _keepDataDir;

    private CoreProcess(Process process, string dataDir, CoreFileInfo info, string exe, string mutexName, StringBuilder output)
    {
        _process = process;
        _output = output;
        _mutexName = mutexName;
        DataDir = dataDir;
        Info = info;
        ExePath = exe;
        _keepDataDir = Environment.GetEnvironmentVariable("AXIOM_TEST_KEEP") == "1";
    }

    public string DataDir { get; }

    public string ExePath { get; }

    public CoreFileInfo Info { get; }

    public int Port => Info.Port;

    public int Pid => Info.Pid;

    public string LogFile => Path.Combine(DataDir, "core.log");

    public string CoreJsonPath => Path.Combine(DataDir, "core.json");

    public bool HasExited => _process.HasExited;

    public string Output
    {
        get
        {
            lock (_output)
            {
                return _output.ToString();
            }
        }
    }

    public static CoreProcess Start(bool singleInstance = false, string? mutexName = null, int readyTimeoutMs = 40000)
    {
        string exe = LocateExe();
        string dir = Path.Combine(Path.GetTempPath(), "axiom-core-it-" + Guid.NewGuid().ToString("N")[..8]);
        Directory.CreateDirectory(dir);

        string mutex = mutexName ?? @"Local\AxiomOffice.Core.Test." + Guid.NewGuid().ToString("N")[..8];
        Process process = Launch(exe, dir, singleInstance, mutex, Token, out StringBuilder output);

        var watch = Stopwatch.StartNew();
        CoreFileInfo? info = null;
        while (watch.ElapsedMilliseconds < readyTimeoutMs)
        {
            if (process.HasExited)
            {
                throw new InvalidOperationException(
                    $"Core thoat som (exit {process.ExitCode}) khi test dang khoi dong. Output:\n{Capture(output)}\nLog:\n{ReadLog(dir)}");
            }

            info = CoreFile.Read(Path.Combine(dir, "core.json"));
            if (info != null && info.Port > 0)
            {
                break;
            }

            Thread.Sleep(100);
        }

        if (info == null)
        {
            try
            {
                process.Kill(entireProcessTree: true);
            }
            catch
            {
            }

            throw new InvalidOperationException($"Core khong ghi core.json trong {readyTimeoutMs}ms. Output:\n{Capture(output)}\nLog:\n{ReadLog(dir)}");
        }

        return new CoreProcess(process, dir, info, exe, mutex, output);
    }

    // Chay them mot tien trinh Core voi cung cau hinh (test "instance thu hai phai thoat ngay").
    public Process StartSecondInstanceAndWait(int timeoutMs = 15000)
    {
        Process second = Launch(ExePath, DataDir, singleInstance: true, _mutexName, Token, out _);
        if (!second.WaitForExit(timeoutMs))
        {
            try
            {
                second.Kill(entireProcessTree: true);
            }
            catch
            {
            }

            throw new InvalidOperationException("Instance thu hai khong thoat nhu mong doi.");
        }

        return second;
    }

    public HttpClient Client(bool withToken = false)
    {
        var client = new HttpClient { BaseAddress = new Uri($"http://127.0.0.1:{Port}"), Timeout = TimeSpan.FromSeconds(15) };
        if (withToken)
        {
            client.DefaultRequestHeaders.Add("X-Auth-Token", Token);
        }

        return client;
    }

    public void Dispose()
    {
        try
        {
            if (!_process.HasExited)
            {
                try
                {
                    using HttpClient client = Client(withToken: true);
                    client.PostAsync("/v1/admin/shutdown", null).GetAwaiter().GetResult();
                }
                catch
                {
                    // Core co the da tat hoac khong tra loi: phia duoi se kill.
                }

                if (!_process.WaitForExit(8000))
                {
                    _process.Kill(entireProcessTree: true);
                    _process.WaitForExit(5000);
                }
            }
        }
        catch
        {
        }

        _process.Dispose();

        if (!_keepDataDir)
        {
            try
            {
                Directory.Delete(DataDir, recursive: true);
            }
            catch
            {
            }
        }
    }

    // .NET runtime nam o vi tri nguoi dung (%LOCALAPPDATA%\Microsoft\dotnet) nen apphost can DOTNET_ROOT.
    public static string LocateDotnetRoot()
    {
        string?[] candidates =
        [
            Environment.GetEnvironmentVariable("DOTNET_ROOT"),
            Environment.GetEnvironmentVariable("DOTNET_ROOT_X64"),
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Microsoft", "dotnet"),
            @"C:\Program Files\dotnet",
        ];

        foreach (string? candidate in candidates)
        {
            if (!string.IsNullOrWhiteSpace(candidate) && Directory.Exists(candidate))
            {
                return candidate;
            }
        }

        throw new InvalidOperationException("Khong tim thay .NET runtime root. Cai .NET 10 SDK (New_arch.md muc 5.2) hoac dat DOTNET_ROOT.");
    }

    private static Process Launch(string exe, string dataDir, bool singleInstance, string mutexName, string token, out StringBuilder output)
    {
        var start = new ProcessStartInfo(exe)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            WorkingDirectory = dataDir,
        };
        start.Environment["AXIOM_CORE_DATA_DIR"] = dataDir;
        start.Environment["AXIOM_TOKEN"] = token;
        start.Environment["AXIOM_CORE_PORT"] = "0";
        start.Environment["AXIOM_CORE_SINGLE_INSTANCE"] = singleInstance ? "1" : "0";
        start.Environment["AXIOM_CORE_MUTEX_NAME"] = mutexName;
        start.Environment["DOTNET_ROOT"] = LocateDotnetRoot();

        var capture = new StringBuilder();
        var process = new Process { StartInfo = start };
        process.OutputDataReceived += (_, e) => Append(capture, e.Data);
        process.ErrorDataReceived += (_, e) => Append(capture, e.Data);
        if (!process.Start())
        {
            throw new InvalidOperationException("Khong khoi dong duoc " + exe);
        }

        process.BeginOutputReadLine();
        process.BeginErrorReadLine();
        output = capture;
        return process;
    }

    private static void Append(StringBuilder builder, string? line)
    {
        if (line == null)
        {
            return;
        }

        lock (builder)
        {
            builder.AppendLine(line);
        }
    }

    private static string Capture(StringBuilder builder)
    {
        lock (builder)
        {
            return builder.ToString();
        }
    }

    private static string ReadLog(string dir)
    {
        try
        {
            string path = Path.Combine(dir, "core.log");
            return File.Exists(path) ? File.ReadAllText(path) : "(khong co core.log)";
        }
        catch
        {
            return "(khong doc duoc core.log)";
        }
    }

    private static string LocateExe()
    {
        string? fromEnv = Environment.GetEnvironmentVariable("AXIOM_CORE_EXE");
        if (!string.IsNullOrWhiteSpace(fromEnv) && File.Exists(fromEnv))
        {
            return fromEnv;
        }

        DirectoryInfo? dir = new(AppContext.BaseDirectory);
        while (dir != null && !File.Exists(Path.Combine(dir.FullName, "scripts", "build.ps1")))
        {
            dir = dir.Parent;
        }

        if (dir == null)
        {
            throw new InvalidOperationException("Khong tim thay goc repo (scripts/build.ps1) tu " + AppContext.BaseDirectory);
        }

        string bin = Path.Combine(dir.FullName, "src", "AxiomOffice.Core", "bin");
        string[] found = Directory.Exists(bin)
            ? Directory.GetFiles(bin, "AxiomOffice.Core.exe", SearchOption.AllDirectories)
            : [];
        if (found.Length == 0)
        {
            throw new InvalidOperationException("Chua build AxiomOffice.Core.exe - chay scripts\\build.ps1 hoac dotnet build src/AxiomOffice.Core.");
        }

        return found.OrderByDescending(File.GetLastWriteTimeUtc).First();
    }
}

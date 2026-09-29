using AxiomOffice.Core.Logging;

namespace AxiomOffice.Core.Tests;

// core.log: dinh dang bridge.log, xoay khi > 10MB giu 3 file (New_arch.md muc 8.9).
// CoreLog la static nen cac test nay chay tuan tu trong cung collection.
[Collection("CoreLog")]
public class CoreLogTests : IDisposable
{
    private readonly string _dir = Path.Combine(Path.GetTempPath(), "axiom-core-log-" + Guid.NewGuid().ToString("N")[..8]);

    private string LogPath => Path.Combine(_dir, "core.log");

    public CoreLogTests()
    {
        Directory.CreateDirectory(_dir);
    }

    public void Dispose()
    {
        try
        {
            Directory.Delete(_dir, recursive: true);
        }
        catch
        {
        }

        GC.SuppressFinalize(this);
    }

    [Fact]
    public void Dong_log_co_thoi_gian_muc_va_pid()
    {
        CoreLog.Init(LogPath);
        CoreLog.Info("Agent Core ready");

        string text = File.ReadAllText(LogPath);

        Assert.Contains("[INFO]", text);
        Assert.Contains("[pid " + Environment.ProcessId + "]", text);
        Assert.Contains("Agent Core ready", text);
        Assert.Matches(@"^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} \[INFO\]", text);
    }

    [Fact]
    public void Loi_kem_exception_duoc_ghi_ro_loai()
    {
        CoreLog.Init(LogPath);
        CoreLog.Error("khong ghi duoc file", new IOException("dia day"));

        string text = File.ReadAllText(LogPath);
        Assert.Contains("[ERROR]", text);
        Assert.Contains("IOException: dia day", text);
    }

    [Fact]
    public void Xoay_file_khi_vuot_nguong_va_giu_so_file_theo_cau_hinh()
    {
        CoreLog.Init(LogPath, maxBytes: 300, keep: 2);
        for (int i = 0; i < 60; i++)
        {
            CoreLog.Info("dong so " + i + " " + new string('x', 40));
        }

        Assert.True(File.Exists(LogPath), "core.log phai ton tai");
        Assert.True(File.Exists(LogPath + ".1"), "phai co file xoay core.log.1");
        Assert.False(File.Exists(LogPath + ".2"), "keep = 2 nen khong giu them core.log.2");
        Assert.Equal(2, Directory.GetFiles(_dir).Length);
    }
}

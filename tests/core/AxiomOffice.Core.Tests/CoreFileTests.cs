using AxiomOffice.Core.Config;

namespace AxiomOffice.Core.Tests;

// core.json: add-in doc de tim Core (New_arch.md muc 7.1). Ghi atomic, file hong thi coi nhu khong co.
public class CoreFileTests : IDisposable
{
    private readonly string _dir = Path.Combine(Path.GetTempPath(), "axiom-core-test-" + Guid.NewGuid().ToString("N")[..8]);

    private string Path_ => Path.Combine(_dir, "core.json");

    public CoreFileTests()
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

    private static CoreFileInfo Sample(int port = 47840) => new(
        Pid: 4242,
        Port: port,
        Version: "1.0.0",
        Started: "2026-10-01T08:00:00.000Z",
        Protocol: 1,
        Exe: @"C:\thu muc co dau\AxiomOffice.Core.exe");

    [Fact]
    public void Ghi_roi_doc_lai_du_truong()
    {
        CoreFile.Write(Path_, Sample());

        CoreFileInfo? info = CoreFile.Read(Path_);

        Assert.NotNull(info);
        Assert.Equal(4242, info.Pid);
        Assert.Equal(47840, info.Port);
        Assert.Equal("1.0.0", info.Version);
        Assert.Equal(1, info.Protocol);
        Assert.Equal(@"C:\thu muc co dau\AxiomOffice.Core.exe", info.Exe);
    }

    [Fact]
    public void Ghi_lan_hai_thay_the_va_khong_de_lai_file_tam()
    {
        CoreFile.Write(Path_, Sample(47840));
        CoreFile.Write(Path_, Sample(47841));

        Assert.Equal(47841, CoreFile.Read(Path_)!.Port);
        Assert.False(File.Exists(Path_ + ".tmp"), "khong duoc de lai file .tmp");
        Assert.Single(Directory.GetFiles(_dir));
    }

    [Fact]
    public void Chua_co_file_thi_tra_null()
    {
        Assert.Null(CoreFile.Read(Path_));
    }

    [Fact]
    public void File_hong_thi_tra_null_khong_nem_loi()
    {
        File.WriteAllText(Path_, "{ khong phai json");

        Assert.Null(CoreFile.Read(Path_));
    }

    [Fact]
    public void Xoa_file_va_khong_loi_khi_file_khong_ton_tai()
    {
        CoreFile.Write(Path_, Sample());
        CoreFile.Delete(Path_);
        Assert.False(File.Exists(Path_));

        CoreFile.Delete(Path_); // lan hai: khong nem loi
        Assert.False(File.Exists(Path_));
    }
}

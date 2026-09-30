using AxiomOffice.Core.Config;
using AxiomOffice.Core.Memory;

namespace AxiomOffice.Core.Tests;

// SQLite: migration, hoi thoai, tin nhan, luot chay, audit tool call (New_arch.md muc 8.5.2).
public class StorageTests : IDisposable
{
    private readonly string _dir = Path.Combine(Path.GetTempPath(), "axiom-db-" + Guid.NewGuid().ToString("N")[..8]);
    private readonly CoreDb _db;

    public StorageTests()
    {
        Directory.CreateDirectory(_dir);
        CorePaths paths = CorePaths.Create(_dir);
        _db = new CoreDb(paths.DatabaseFile);
        _db.Migrate();
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
    public void Migration_tao_du_bang_va_idempotent()
    {
        _db.Migrate();
        _db.Migrate();

        using Microsoft.Data.Sqlite.SqliteConnection connection = _db.Open();
        using Microsoft.Data.Sqlite.SqliteCommand command = connection.CreateCommand();
        command.CommandText = "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name";
        var tables = new List<string>();
        using Microsoft.Data.Sqlite.SqliteDataReader reader = command.ExecuteReader();
        while (reader.Read())
        {
            tables.Add(reader.GetString(0));
        }

        Assert.Contains("conversations", tables);
        Assert.Contains("messages", tables);
        Assert.Contains("runs", tables);
        Assert.Contains("tool_calls", tables);
    }

    [Fact]
    public void Hoi_thoai_va_tin_nhan_theo_thu_tu()
    {
        var store = new ConversationStore(_db);
        string id = store.Create("wps", "office", @"c:\a.docx", "a.docx", "tao bang diem");

        Assert.Equal(1, store.AppendMessage(id, "user", "tao bang diem"));
        Assert.Equal(2, store.AppendMessage(id, "tool_summary", "et.writeRange (ok)"));
        Assert.Equal(3, store.AppendMessage(id, "assistant", "da xong"));

        IReadOnlyList<MessageRow> messages = store.Messages(id);
        Assert.Equal(3, messages.Count);
        Assert.Equal(["user", "tool_summary", "assistant"], messages.Select(m => m.Role));

        // limit lay cac tin MOI NHAT nhung van theo thu tu thoi gian.
        IReadOnlyList<MessageRow> lastTwo = store.Messages(id, limit: 2);
        Assert.Equal(2, lastTwo.Count);
        Assert.Equal("tool_summary", lastTwo[0].Role);
        Assert.Equal("assistant", lastTwo[1].Role);
    }

    [Fact]
    public void Tim_hoi_thoai_theo_tai_lieu_va_xoa()
    {
        var store = new ConversationStore(_db);
        string first = store.Create("wps", "office", @"c:\a.docx", "a.docx", "lan 1");
        store.Create("wps", "office", @"c:\b.docx", "b.docx", "tai lieu khac");
        string newest = store.Create("wps", "office", @"c:\a.docx", "a.docx", "lan 2");

        Assert.Equal(newest, store.LatestForDocument(@"c:\a.docx")!.Id);
        Assert.Single(store.List(@"c:\b.docx"));
        Assert.NotNull(store.Get(first));

        Assert.True(store.Rename(newest, summary: "tom tat moi"));
        Assert.Equal("tom tat moi", store.Get(newest)!.Summary);

        Assert.True(store.Delete(newest));
        Assert.Null(store.Get(newest));
        // Xoa hoi thoai xoa luon tin nhan (ON DELETE CASCADE).
        Assert.Empty(store.Messages(newest));
    }

    [Fact]
    public void Luot_chay_va_audit_tool_call()
    {
        var store = new ConversationStore(_db);
        var runs = new RunStore(_db);
        string conversationId = store.Create("wps", "office", null, null, null);

        runs.Insert("r_1", conversationId, "gpt-x");
        runs.AddToolCall("r_1", 1, "office_action", "et.writeRange", """{"range":"A1"}""", true, null, 42);
        runs.AddToolCall("r_1", 2, "office_action", "et.writeRange", new string('x', 5000), false, "loi dai " + new string('y', 900), 7);
        runs.Finish("r_1", "completed", 3, 1000, 250, null);

        RunRow? run = runs.Get("r_1");
        Assert.NotNull(run);
        Assert.Equal("completed", run!.Status);
        Assert.Equal(3, run.Rounds);
        Assert.Equal(1000, run.InputTokens);
        Assert.Equal(250, run.OutputTokens);

        IReadOnlyList<ToolCallRow> calls = runs.ToolCalls("r_1");
        Assert.Equal(2, calls.Count);
        Assert.Equal("et.writeRange", calls[0].Action);
        Assert.Equal(1, calls[0].Ok);
        Assert.Equal(42, calls[0].Ms);
        Assert.Equal(0, calls[1].Ok);
        Assert.NotNull(calls[1].Error);
        Assert.True(calls[1].Error!.Length <= 503, "loi duoc cat ngan khi ghi audit");
    }

    [Fact]
    public void CoreStores_bao_memory_khong_dung_duoc_khi_db_hong()
    {
        // Duong dan la file da ton tai nhung khong phai SQLite -> migration loi, Core van chay.
        string bad = Path.Combine(_dir, "bad.db");
        File.WriteAllText(bad, "khong phai sqlite");
        CorePaths paths = CorePaths.Create(Path.Combine(_dir, "badcase"));
        Directory.CreateDirectory(Path.Combine(_dir, "badcase"));
        File.Copy(bad, paths.DatabaseFile);

        var stores = new Api.CoreStores(paths);

        Assert.False(stores.Available);
        Assert.NotNull(stores.Error);
        Assert.Equal("unavailable", stores.Status(memoryEnabled: true));
        Assert.Equal("off", stores.Status(memoryEnabled: false));
    }

    [Theory]
    [InlineData("C:\\Docs\\Bao Cao.DOCX", @"c:\docs\bao cao.docx", "C:\\Docs\\Bao Cao.DOCX")]
    [InlineData("c:/docs/bao-cao.docx", @"c:\docs\bao-cao.docx", "c:/docs/bao-cao.docx")]
    [InlineData(" /home/an/Bao Cao.odt ", @"\home\an\bao cao.odt", "/home/an/Bao Cao.odt")]
    [InlineData("   ", null, null)]
    [InlineData(null, null, null)]
    public void DocumentKey_chuan_hoa_duong_dan(string? input, string? windows, string? unix)
    {
        // Windows khong phan biet hoa/thuong -> chuan hoa; Linux phan biet -> giu nguyen (chi bo khoang trang).
        Assert.Equal(OperatingSystem.IsWindows() ? windows : unix, Agent.Orchestrator.DocumentKey(input));
    }
}

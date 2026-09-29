using System.Text.Json.Nodes;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Memory;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Office;
using AxiomOffice.Core.Tools;
using Microsoft.Data.Sqlite;

namespace AxiomOffice.Core.Tests;

// Memory dai han (New_arch.md muc 8.5, 11): migration, FTS5 bo dau, chong trung hash, ADD + link + history trong
// transaction, linked id bia bi bo, loc nhay cam, bo qua trich xuat voi lenh thao tac thuan, sigmoid theo do dai
// truy van, cham diem co/khong embedding, entity boost, het han an, xoa mem/khoi phuc.
public sealed class MemoryTests : IDisposable
{
    private const string Doc = @"c:\bao-cao\ke-hoach.docx";
    private readonly string _dir = Path.Combine(Path.GetTempPath(), "axiom-mem-" + Guid.NewGuid().ToString("N")[..8]);
    private readonly CoreDb _db;
    private readonly SqliteMemoryStore _store;

    public MemoryTests()
    {
        Directory.CreateDirectory(_dir);
        _db = new CoreDb(CorePaths.Create(_dir).DatabaseFile);
        _db.Migrate();
        _store = new SqliteMemoryStore(_db);
    }

    public void Dispose()
    {
        SqliteConnection.ClearAllPools();
        try
        {
            Directory.Delete(_dir, recursive: true);
        }
        catch (IOException)
        {
        }
    }

    private MemoryItem Add(string text, string scope = MemoryScopes.User, string? key = null, string[]? entities = null, string[]? links = null, string? expires = null)
    {
        MemoryOpResult result = _store.AddBatch([new NewMemory(scope, key, text, "other", entities, links, expires)], MemoryActors.Extract, "r_1")[0];
        Assert.Equal("ADD", result.Event);
        return result.Item!;
    }

    [Fact]
    public void Migration_schema_2_co_bang_memory_va_nang_cap_tu_schema_1()
    {
        string oldPath = Path.Combine(_dir, "old.db");
        using (var connection = new SqliteConnection("Data Source=" + oldPath))
        {
            connection.Open();
            using SqliteCommand command = connection.CreateCommand();
            command.CommandText = "CREATE TABLE conversations (id TEXT PRIMARY KEY); PRAGMA user_version = 1;";
            command.ExecuteNonQuery();
        }

        var db = new CoreDb(oldPath);
        db.Migrate();
        db.Migrate();

        using SqliteConnection open = db.Open();
        using SqliteCommand tables = open.CreateCommand();
        tables.CommandText = "SELECT name FROM sqlite_master WHERE type IN ('table') ORDER BY name";
        var names = new List<string>();
        using (SqliteDataReader reader = tables.ExecuteReader())
        {
            while (reader.Read())
            {
                names.Add(reader.GetString(0));
            }
        }

        Assert.Contains("memories", names);
        Assert.Contains("memories_fts", names);
        Assert.Contains("memory_links", names);
        Assert.Contains("memory_history", names);
        Assert.Contains("memory_embeddings", names);
        using SqliteCommand version = open.CreateCommand();
        version.CommandText = "PRAGMA user_version";
        Assert.Equal(CoreDb.SchemaVersion, Convert.ToInt32(version.ExecuteScalar()));
    }

    [Theory]
    [InlineData("Hợp đồng số 12", "hop dong so 12")]
    [InlineData("  Lưu, ĐƯỜNG   dẫn!  ", "luu duong dan")]
    [InlineData("Trưởng phòng Kế toán", "truong phong ke toan")]
    public void Chuan_hoa_bo_dau_ke_ca_d(string input, string expected)
    {
        Assert.Equal(expected, MemoryText.Normalize(input));
    }

    [Fact]
    public void Fts_tim_khong_dau_khop_co_dau()
    {
        MemoryItem contract = Add("Hợp đồng thuê văn phòng hết hạn tháng 12");
        Add("Thích font Times New Roman 13");

        IReadOnlyDictionary<long, double> scores = _store.KeywordScores(MemoryText.FtsQuery(MemoryText.QueryTerms("hop dong"))!);

        Assert.Equal([contract.RowId], scores.Keys);
    }

    [Fact]
    public void Trung_hash_trong_lo_va_voi_memory_cu_khong_tao_ban_moi()
    {
        MemoryItem first = Add("Người ký công văn: Nguyễn Văn A");

        IReadOnlyList<MemoryOpResult> results = _store.AddBatch(
        [
            new NewMemory(MemoryScopes.User, null, "người ký CÔNG VĂN: nguyen van a."),
            new NewMemory(MemoryScopes.User, null, "Thích căn đều đoạn văn"),
            new NewMemory(MemoryScopes.User, null, "thich can deu doan van"),
            new NewMemory(MemoryScopes.Document, Doc, "Người ký công văn: Nguyễn Văn A"),
        ], MemoryActors.Extract, "r_2");

        Assert.Equal(["DUPLICATE", "ADD", "DUPLICATE", "ADD"], results.Select(r => r.Event));
        Assert.Equal(first.Id, results[0].Item!.Id);
        Assert.Equal(1, _store.Get(first.Id)!.Hits);
        Assert.Equal(3, _store.List().Count);
    }

    [Fact]
    public void Add_ghi_link_history_va_bo_link_bia()
    {
        MemoryItem old = Add("Chức vụ người dùng: Trưởng phòng Kế toán");

        MemoryItem changed = Add("Chức vụ đổi từ Trưởng phòng Kế toán sang Phó giám đốc từ 10/2026", links: [old.Id, "m_khong_ton_tai"]);

        Assert.Equal([old.Id], changed.Links);
        MemoryHistoryEntry entry = Assert.Single(_store.History(changed.Id));
        Assert.Equal("ADD", entry.Event);
        Assert.Equal(MemoryActors.Extract, entry.Actor);
        Assert.Equal("r_1", entry.RunId);
        Assert.NotNull(_store.Get(old.Id));
        Assert.Null(_store.Get(old.Id)!.DeletedAt);
    }

    [Theory]
    [InlineData("Số CCCD của tôi là 012345678901")]
    [InlineData("Mật khẩu email là abc123")]
    [InlineData("Thẻ 4111 1111 1111 1111 dùng thanh toán")]
    [InlineData("Key sk-abcdefghijklmnopqrstuvwxyz123456")]
    public void Thong_tin_nhay_cam_khong_duoc_luu(string text)
    {
        Assert.True(MemoryText.IsSensitive(text));
        MemoryOpResult result = _store.AddBatch([new NewMemory(MemoryScopes.User, null, text)], MemoryActors.Agent, null)[0];
        Assert.Equal("SKIPPED", result.Event);
        Assert.Equal("sensitive", result.Reason);
        Assert.False(MemoryText.IsSensitive("Người ký công văn: Nguyễn Văn A, số điện thoại phòng 3825"));
    }

    [Fact]
    public void Extractor_loc_confidence_do_dai_nhay_cam_ngay_va_id_bia()
    {
        var temp = new Dictionary<string, string> { ["0"] = "m_that" };
        string output = """
            ```json
            {"facts": [
              {"text": "Chức vụ đổi từ Trưởng phòng sang Phó giám đốc", "scope": "user", "category": "identity", "confidence": 0.9, "entities": ["Phó giám đốc"], "linkedIds": ["0", "7", "m_bia"], "expiresAt": "2026-13-40"},
              {"text": "Có thể thích màu xanh", "confidence": 0.4},
              {"text": "", "confidence": 0.9},
              {"text": "Mật khẩu là 123456", "confidence": 0.95},
              {"text": "Đã xong mục 1-3, còn thiếu phần kinh phí", "category": "progress", "confidence": 0.8, "expiresAt": "2026-12-31"}
            ]}
            ```
            """;

        IReadOnlyList<NewMemory>? facts = MemoryExtractor.ParseFacts(output, temp, Doc, out string? error);

        Assert.Null(error);
        Assert.Equal(2, facts!.Count);
        Assert.Equal(["m_that"], facts[0].LinkedIds);
        Assert.Null(facts[0].ExpiresAt);
        Assert.Equal(MemoryScopes.User, facts[0].Scope);
        Assert.Equal(MemoryScopes.Document, facts[1].Scope);
        Assert.Equal(Doc, facts[1].ScopeKey);
        Assert.Equal("2026-12-31", facts[1].ExpiresAt);
        Assert.Null(MemoryExtractor.ParseFacts("khong phai json", temp, Doc, out string? bad));
        Assert.NotNull(bad);
    }

    [Theory]
    [InlineData("In đậm dòng này", true)]
    [InlineData("In đậm dòng đầu tiên", true)]
    [InlineData("Căn giữa tiêu đề và đổi màu chữ sang xanh", true)]
    [InlineData("Tạo bảng điểm 5 học sinh có cột trung bình", true)]
    [InlineData("Tôi là trưởng phòng Kế toán, công văn ký tên Nguyễn Văn A", false)]
    [InlineData("Soạn công văn gửi Sở Tài chính về kế hoạch quý 4", false)]
    [InlineData("Từ nay luôn dùng font Times New Roman 13 nhé", false)]
    [InlineData("ok", true)]
    public void Bo_qua_trich_xuat_voi_lenh_thao_tac_thuan(string prompt, bool skip)
    {
        Assert.Equal(skip, MemoryExtractor.ShouldSkip(prompt, out _));
    }

    [Theory]
    [InlineData(2, 5, 0.7)]
    [InlineData(5, 7, 0.6)]
    [InlineData(8, 9, 0.5)]
    [InlineData(12, 10, 0.5)]
    [InlineData(20, 12, 0.5)]
    public void Sigmoid_theo_do_dai_truy_van_nhu_mem0(int terms, double midpoint, double steepness)
    {
        Assert.Equal((midpoint, steepness), MemoryRetriever.Bm25Params(terms));
        Assert.Equal(0.5, MemoryRetriever.NormalizeBm25(midpoint, terms), 6);
    }

    [Fact]
    public void Entity_boost_giam_khi_thuc_the_pho_bien()
    {
        Assert.Equal(0.5, MemoryRetriever.EntityBoost(1), 6);
        Assert.True(MemoryRetriever.EntityBoost(10) < MemoryRetriever.EntityBoost(2));
        Assert.True(MemoryRetriever.EntityBoost(100) < 0.25);
    }

    [Fact]
    public void Ngu_canh_uu_tien_memory_khop_bo_memory_khong_lien_quan_va_het_han()
    {
        Add("Người ký công văn: Nguyễn Văn A, Trưởng phòng Kế toán", entities: ["Nguyễn Văn A"]);
        Add("Thích xem phim hành động cuối tuần");
        Add("Hạn nộp báo cáo quý 3 là 15/10", expires: "2020-01-01");
        Add("Đã xong mục 1-3 của kế hoạch", MemoryScopes.Document, Doc);
        Add("Ghi chú của tài liệu khác", MemoryScopes.Document, @"c:\khac.docx");
        var retriever = new MemoryRetriever(_store);

        MemoryContext context = retriever.BuildContext("Soạn công văn gửi Sở Tài chính", Doc, new DateOnly(2026, 9, 30));

        Assert.Contains(context.Lines, l => l.Contains("Nguyễn Văn A"));
        Assert.Contains(context.Lines, l => l.Contains("Đã xong mục 1-3"));
        Assert.DoesNotContain(context.Lines, l => l.Contains("phim"));
        Assert.DoesNotContain(context.Lines, l => l.Contains("Hạn nộp"));
        Assert.DoesNotContain(context.Lines, l => l.Contains("tài liệu khác"));
        Assert.Equal(context.Lines.Count, context.Ids.Count);
    }

    [Fact]
    public void Ghim_luon_vao_ngu_canh_va_ban_chuyen_doi_moi_dung_truoc()
    {
        MemoryItem pinned = Add("Luôn ký tên kèm con dấu đỏ");
        _store.Update(pinned.Id, null, null, null, false, true, null);
        MemoryItem old = Add("Chức vụ ký công văn: Trưởng phòng Kế toán");
        Add("Chức vụ ký công văn đổi từ Trưởng phòng Kế toán sang Phó giám đốc từ 10/2026", links: [old.Id]);

        MemoryContext context = new MemoryRetriever(_store).BuildContext("chức vụ ký công văn", null, new DateOnly(2026, 10, 5));

        Assert.StartsWith("[" + MemoryRetriever.ShortId(pinned.Id) + "]", context.Lines[0]);
        int changed = context.Lines.ToList().FindIndex(l => l.Contains("đổi từ"));
        int stale = context.Lines.ToList().FindIndex(l => l.Contains("Trưởng phòng Kế toán") && !l.Contains("đổi từ"));
        Assert.True(changed >= 0 && stale >= 0 && changed < stale, string.Join(" | ", context.Lines));
    }

    [Fact]
    public void Co_embedding_thi_semantic_la_tin_hieu_chinh_va_mau_so_tinh_ca_semantic()
    {
        MemoryItem close = Add("Sếp trực tiếp là bà Trần Thị B");
        MemoryItem far = Add("Thích uống cà phê sữa");
        var retriever = new MemoryRetriever(_store);
        var semantic = new Dictionary<string, double> { [close.Id] = 0.9, [far.Id] = 0.05 };

        IReadOnlyList<MemoryHit> hits = retriever.Score(_store.Active(null, new DateOnly(2026, 9, 30)), "quản lý của tôi là ai", semantic);

        MemoryHit hit = Assert.Single(hits);
        Assert.Equal(close.Id, hit.Item.Id);
        Assert.Equal(0.9 / 2.0, hit.Score, 3);
    }

    [Fact]
    public void Xoa_mem_an_khoi_tim_kiem_khoi_phuc_duoc_va_lich_su_do_nguoi_dung()
    {
        MemoryItem item = Add("Hợp đồng thuê văn phòng hết hạn tháng 12");
        string query = MemoryText.FtsQuery(MemoryText.QueryTerms("hop dong"))!;

        Assert.True(_store.Delete(item.Id, "khong can nua"));
        Assert.Empty(_store.KeywordScores(query));
        Assert.Empty(_store.Active(null, new DateOnly(2026, 9, 30)));
        Assert.True(_store.Restore(item.Id));
        Assert.Single(_store.KeywordScores(query));

        MemoryItem updated = _store.Update(item.Id, "Hợp đồng thuê văn phòng hết hạn 31/12/2026", null, "2027-01-31", false, null, "cap nhat")!;
        Assert.Equal("2027-01-31", updated.ExpiresAt);
        Assert.Equal(["ADD", "DELETE", "RESTORE", "UPDATE", "EXPIRES"], _store.History(item.Id).Select(h => h.Event));
        Assert.All(_store.History(item.Id).Skip(1), h => Assert.Equal(MemoryActors.User, h.Actor));
        Assert.Equal(1, _store.Purge(null));
        Assert.Empty(_store.List(includeDeleted: true));
    }

    [Fact]
    public async Task Tool_remember_ghi_memory_va_phat_memory_written()
    {
        var events = new List<(string Type, JsonNode? Data)>();
        using var service = Service();
        var config = new CoreConfig { Token = "t" };
        var context = new RunContext
        {
            RunId = "r_9",
            ConversationId = "c_9",
            Office = new OfficeSession(1, "wps", "office", 47831, "WINWORD.EXE", "1.0.0", 0, "a.docx", Doc, "f.json"),
            Config = config,
            Bridge = new BridgeClient(config, new HttpClient()),
            Event = (type, data) => events.Add((type, data)),
        };

        ToolResult saved = await new RememberTool(service, Doc).InvokeAsync(
            new JsonObject { ["scope"] = "user", ["text"] = "Cơ quan: Sở GD&ĐT Hà Nội", ["category"] = "identity" }, context, CancellationToken.None);
        ToolResult again = await new RememberTool(service, Doc).InvokeAsync(
            new JsonObject { ["scope"] = "user", ["text"] = "co quan: so gd&dt ha noi" }, context, CancellationToken.None);
        ToolResult secret = await new RememberTool(service, Doc).InvokeAsync(
            new JsonObject { ["scope"] = "user", ["text"] = "Mật khẩu wifi là 12345678" }, context, CancellationToken.None);
        ToolResult recall = await new RecallTool(service, Doc).InvokeAsync(new JsonObject { ["query"] = "co quan" }, context, CancellationToken.None);

        Assert.True(saved.Ok);
        Assert.Contains("\"DUPLICATE\"", again.Json);
        Assert.False(secret.Ok);
        (string type, JsonNode? data) = Assert.Single(events);
        Assert.Equal("memory.written", type);
        Assert.Equal("Cơ quan: Sở GD&ĐT Hà Nội", data!["text"]!.GetValue<string>());
        Assert.Contains("Sở GD&ĐT Hà Nội", recall.Json);
        Assert.Equal(MemoryActors.Agent, _store.List().Single().Source);
    }

    [Fact]
    public void Tat_memory_thi_khong_doc_khong_trich_xuat()
    {
        var runs = new RunStore(_db);
        runs.Insert("r_off", null, "m");
        using var service = Service(new CoreConfig { MemoryEnabled = false });

        Assert.False(service.Enabled);
        Assert.Equal("skipped", service.QueueExtraction(new ExtractionJob("r_off", "c", "Tôi là trưởng phòng Kế toán của công ty", "ok", null)));
        Assert.Equal("skipped", runs.Get("r_off")!.MemoryStatus);
    }

    private MemoryService Service(CoreConfig? config = null)
    {
        CoreConfig current = config ?? new CoreConfig();
        return new MemoryService(_store, new ConversationStore(_db), new RunStore(_db), () => current,
            c => new ModelClient(new HttpClient(), "openai", "", "", ""), new HttpClient(), available: true);
    }
}

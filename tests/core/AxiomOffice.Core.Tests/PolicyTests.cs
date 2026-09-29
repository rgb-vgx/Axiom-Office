using System.Text.Json.Nodes;
using AxiomOffice.Core.Agent;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Office;
using AxiomOffice.Core.Tools;

namespace AxiomOffice.Core.Tests;

// Policy xac nhan + ConfirmationBroker (New_arch.md muc 8.6, 11).
public class PolicyTests
{
    private static Task<PolicyDecision> Evaluate(string action, JsonNode? parameters = null, string prompt = "tao bang", int? length = null)
    {
        return PolicyEngine.EvaluateAsync(action, parameters, prompt, _ => Task.FromResult(length), CancellationToken.None);
    }

    [Theory]
    [InlineData("writer.save", "Soạn công văn")]
    [InlineData("et.saveAs", "Tạo bảng điểm")]
    [InlineData("wpp.exportPdf", "Làm slide")]
    public async Task Luu_xuat_khi_nguoi_dung_khong_yeu_cau_thi_hoi(string action, string prompt)
    {
        PolicyDecision decision = await Evaluate(action, new JsonObject { ["path"] = @"C:\khong-ton-tai\x.docx" }, prompt);
        Assert.True(decision.NeedsConfirmation);
        Assert.Contains("lưu", decision.Reason);
    }

    [Theory]
    [InlineData("writer.save", "Lưu file giúp mình")]
    [InlineData("et.saveAs", "save as bang-diem.xlsx")]
    [InlineData("wpp.exportPdf", "Xuất PDF bài này")]
    [InlineData("writer.exportPdf", "xuat pdf")]
    public async Task Nguoi_dung_yeu_cau_luu_xuat_thi_khong_hoi(string action, string prompt)
    {
        Assert.False((await Evaluate(action, new JsonObject { ["path"] = @"C:\khong-ton-tai\x.pdf" }, prompt)).NeedsConfirmation);
    }

    [Fact]
    public async Task SaveAs_ghi_de_file_da_co_thi_hoi_du_nguoi_dung_yeu_cau_luu()
    {
        string existing = Path.GetTempFileName();
        try
        {
            PolicyDecision decision = await Evaluate("writer.saveAs", new JsonObject { ["path"] = existing }, "Lưu thành file khác");
            Assert.True(decision.NeedsConfirmation);
            Assert.Contains("Ghi đè", decision.Reason);
        }
        finally
        {
            File.Delete(existing);
        }
    }

    [Fact]
    public async Task Xoa_slide_va_replaceAll_tai_lieu_dai_thi_hoi()
    {
        Assert.True((await Evaluate("wpp.deleteSlide")).NeedsConfirmation);
        Assert.True((await Evaluate("writer.replaceAll", length: 25_000)).NeedsConfirmation);
        Assert.False((await Evaluate("writer.replaceAll", length: 500)).NeedsConfirmation);
        Assert.False((await Evaluate("writer.replaceAll", length: null)).NeedsConfirmation);
        Assert.False((await Evaluate("et.writeRange")).NeedsConfirmation);
    }

    [Fact]
    public async Task Broker_dong_y_tu_choi_het_gio_va_huy()
    {
        var events = new RunEventStream();
        var broker = new ConfirmationBroker();

        Task<bool> approve = broker.RequestAsync(events, "wpp.deleteSlide", "Xoá slide", "{}", TimeSpan.FromSeconds(30), CancellationToken.None);
        string firstId = await PendingId(events, 0);
        Assert.True(broker.Resolve(firstId, true));
        Assert.True(await approve);

        Task<bool> reject = broker.RequestAsync(events, "wpp.deleteSlide", "Xoá slide", "{}", TimeSpan.FromSeconds(30), CancellationToken.None);
        Assert.True(broker.Resolve(await PendingId(events, 1), false));
        Assert.False(await reject);

        Assert.False(await broker.RequestAsync(events, "wpp.deleteSlide", "Xoá slide", "{}", TimeSpan.FromMilliseconds(50), CancellationToken.None));

        using var cancel = new CancellationTokenSource(50);
        Assert.False(await broker.RequestAsync(events, "wpp.deleteSlide", "Xoá slide", "{}", TimeSpan.FromSeconds(30), cancel.Token));

        Assert.False(broker.Resolve("cf_khong_co", true));
        Assert.Equal(0, broker.PendingCount);
        string[] resolvedBy = events.Since(0).Where(e => e.Type == "confirm.resolved").Select(e => e.Data!["by"]!.GetValue<string>()).ToArray();
        Assert.Equal(["user", "user", "timeout", "cancelled"], resolvedBy);
    }

    [Fact]
    public async Task OfficeAction_bi_tu_choi_tra_user_declined_va_khong_goi_bridge()
    {
        var handler = new ScriptedHandler((_, _) => ScriptedHandler.Json("""{"ok":true,"result":{}}"""));
        var config = new CoreConfig { Token = "t" };
        var catalog = new OfficeCommandCatalog("1.0.0", [new OfficeCommand("wpp.deleteSlide", "wpp", true, "xoa", [])]);
        var context = new RunContext
        {
            RunId = "r",
            ConversationId = "c",
            Office = new OfficeSession(1, "wpp", "office", 47833, "POWERPNT.EXE", "1.0.0", 0, "a.pptx", @"C:\a.pptx", "f.json"),
            Config = config,
            Bridge = new BridgeClient(config, new HttpClient(handler)),
            Prompt = "xoa slide cuoi",
            Confirm = (_, _, _, _) => Task.FromResult(false),
        };

        ToolResult result = await new OfficeActionTool(catalog, "wpp").InvokeAsync(new JsonObject { ["action"] = "wpp.deleteSlide" }, context, CancellationToken.None);

        Assert.False(result.Ok);
        Assert.Contains("user declined", result.Json);
        Assert.Equal(0, handler.Count);
    }

    private static async Task<string> PendingId(RunEventStream events, int index)
    {
        for (int i = 0; i < 100; i++)
        {
            RunEvent[] pending = events.Since(0).Where(e => e.Type == "confirm.required").ToArray();
            if (pending.Length > index)
            {
                return pending[index].Data!["confirmationId"]!.GetValue<string>();
            }

            await Task.Delay(10);
        }

        throw new TimeoutException("confirm.required not published");
    }
}

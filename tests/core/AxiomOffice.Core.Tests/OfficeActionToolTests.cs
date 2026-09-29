using System.Text.Json.Nodes;
using AxiomOffice.Core.Config;
using AxiomOffice.Core.Office;
using AxiomOffice.Core.Tools;

namespace AxiomOffice.Core.Tests;

// Tool office_action: allowlist theo ForAgent va theo loai app (New_arch.md muc 8.3, 8.6).
public class OfficeActionToolTests
{
    private static OfficeCommandCatalog Catalog()
    {
        return new OfficeCommandCatalog("1.0.0",
        [
            new OfficeCommand("writer.getText", "wps", true, "doc text", [new OfficeCommandParam("maxChars", false, null)]),
            new OfficeCommand("writer.closeAll", "wps", false, "dong het", []),
            new OfficeCommand("et.writeRange", "et", true, "ghi vung",
                [new OfficeCommandParam("range", true, "top-left cell e.g. 'A1'"), new OfficeCommandParam("values", true, "2D array of rows")]),
            new OfficeCommand("ai.ask", null, false, "agent long nhau", []),
        ]);
    }

    private static RunContext Context(int port = 47831, BridgeClient? bridge = null)
    {
        var config = new CoreConfig { Token = "t" };
        var http = new HttpClient(new ScriptedHandler((_, _) => ScriptedHandler.Json("{}")));
        return new RunContext
        {
            RunId = "r_1",
            ConversationId = "c_1",
            Office = new OfficeSession(1, "wps", "office", port, "WINWORD.EXE", "1.0.0", 0, "a.docx", @"C:\a.docx", "f.json"),
            Config = config,
            Bridge = bridge ?? new BridgeClient(config, http),
        };
    }

    [Fact]
    public void Mo_ta_tool_liet_ke_lenh_agent_cua_dung_app_va_tham_so()
    {
        var tool = new OfficeActionTool(Catalog(), "wps");

        Assert.Contains("writer.getText {maxChars}", tool.Description);
        Assert.DoesNotContain("et.writeRange", tool.Description);
        Assert.DoesNotContain("writer.closeAll", tool.Description);
        Assert.DoesNotContain("ai.ask", tool.Description);
    }

    [Fact]
    public void Mo_ta_tool_hien_hint_cua_tham_so()
    {
        var tool = new OfficeActionTool(Catalog(), "et");

        Assert.Contains("et.writeRange {range (top-left cell e.g. 'A1'),values (2D array of rows)}", tool.Description);
    }

    [Fact]
    public async Task Lenh_ngoai_danh_sach_bi_tu_choi()
    {
        var tool = new OfficeActionTool(Catalog(), "wps");
        var context = Context();

        ToolResult refused = await tool.InvokeAsync(new JsonObject { ["action"] = "writer.closeAll" }, context, CancellationToken.None);
        ToolResult unknown = await tool.InvokeAsync(new JsonObject { ["action"] = "nosuch.action" }, context, CancellationToken.None);
        ToolResult missing = await tool.InvokeAsync(new JsonObject(), context, CancellationToken.None);
        ToolResult wrongApp = await tool.InvokeAsync(new JsonObject { ["action"] = "et.writeRange" }, context, CancellationToken.None);

        Assert.False(refused.Ok);
        Assert.Contains("not an available action", refused.Json);
        Assert.False(unknown.Ok);
        Assert.False(missing.Ok);
        Assert.Contains("missing 'action'", missing.Json);
        Assert.False(wrongApp.Ok);
    }

    [Fact]
    public async Task Lenh_hop_le_di_qua_bridge_va_tra_nguyen_van_ket_qua()
    {
        var handler = new ScriptedHandler((request, body) =>
        {
            Assert.Equal("/cmd", request.RequestUri!.AbsolutePath);
            Assert.Equal("t", request.Headers.GetValues("X-Auth-Token").First());
            Assert.Contains("\"action\":\"writer.getText\"", body);
            return ScriptedHandler.Json("""{"ok":true,"result":{"text":"xin chao"}}""");
        });
        var config = new CoreConfig { Token = "t" };
        var context = Context(bridge: new BridgeClient(config, new HttpClient(handler)));
        var tool = new OfficeActionTool(Catalog(), "wps");

        ToolResult result = await tool.InvokeAsync(
            new JsonObject { ["action"] = "writer.getText", ["params"] = new JsonObject { ["maxChars"] = 100 } },
            context, CancellationToken.None);

        Assert.True(result.Ok);
        Assert.Equal("writer.getText", result.Action);
        Assert.Contains("xin chao", result.Json);
    }

    [Theory]
    [InlineData("et_writeRange", "et.writeRange")]
    [InlineData("ET.WRITERANGE", "et.writeRange")]
    [InlineData(" et.writeRange ", "et.writeRange")]
    [InlineData("writer_closeAll", "writer_closeAll")]
    [InlineData("nosuch_action", "nosuch_action")]
    public void Ten_lenh_viet_sai_nhe_duoc_quy_ve_ten_dung(string input, string expected)
    {
        Assert.Equal(expected, OfficeActionTool.CanonicalAction(input, ["et.writeRange", "et.readRange"]));
    }

    [Fact]
    public void Params_dang_chuoi_json_duoc_parse_thanh_object()
    {
        JsonNode? parsed = OfficeActionTool.NormalizeParams(JsonValue.Create("""{"rows":3,"cols":"2"}"""));

        Assert.IsType<JsonObject>(parsed);
        Assert.Equal(3, parsed!["rows"]!.GetValue<int>());
        Assert.Null(OfficeActionTool.NormalizeParams(JsonValue.Create("  ")));
        Assert.Equal("khong phai json", OfficeActionTool.NormalizeParams(JsonValue.Create("khong phai json"))!.GetValue<string>());
        Assert.Null(OfficeActionTool.NormalizeParams(null));
    }

    [Fact]
    public async Task Params_chuoi_json_gui_sang_bridge_la_object()
    {
        string? sent = null;
        var handler = new ScriptedHandler((_, body) =>
        {
            sent = body;
            return ScriptedHandler.Json("""{"ok":true,"result":{}}""");
        });
        var config = new CoreConfig { Token = "t" };
        var context = Context(bridge: new BridgeClient(config, new HttpClient(handler)));
        var tool = new OfficeActionTool(Catalog(), "wps");

        await tool.InvokeAsync(
            new JsonObject { ["action"] = "writer.getText", ["params"] = """{"maxChars":50}""" },
            context, CancellationToken.None);

        Assert.Contains("\"params\":{\"maxChars\":50}", sent);
    }

    [Theory]
    [InlineData(1, 20, false)]
    [InlineData(1, 21, true)]
    [InlineData(18, 23, true)]
    [InlineData(22, 30, false)]
    [InlineData(35, 41, true)]
    [InlineData(41, 45, false)]
    public void Tom_tat_chi_khi_vuot_moc_moi(int firstSeq, int lastSeq, bool expected)
    {
        Assert.Equal(expected, AxiomOffice.Core.Agent.Orchestrator.ShouldSummarize(firstSeq, lastSeq));
    }

    [Fact]
    public void Registry_tim_tool_theo_ten()
    {
        var registry = new ToolRegistry([new OfficeActionTool(Catalog(), "wps")]);

        Assert.NotNull(registry.Find("office_action"));
        Assert.Null(registry.Find("khong_co"));
        Assert.Single(registry.All);
    }
}

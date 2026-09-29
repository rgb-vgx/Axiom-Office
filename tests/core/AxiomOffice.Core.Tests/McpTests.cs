using AxiomOffice.Core.Mcp;

namespace AxiomOffice.Core.Tests;

// MCP client (New_arch.md muc 8.7): doc mcp.json, server built-in office, ten tool, loc tool live.
public sealed class McpTests : IDisposable
{
    private readonly string _host = Path.Combine(Path.GetTempPath(), "axiom-host-" + Guid.NewGuid().ToString("N")[..8] + ".exe");

    public McpTests()
    {
        File.WriteAllText(_host, "gia lap exe");
    }

    public void Dispose()
    {
        File.Delete(_host);
    }

    [Fact]
    public void Office_built_in_tin_cay_va_server_mcp_json()
    {
        IReadOnlyList<McpServerConfig> configs = McpManager.LoadConfigs("""
            {"mcpServers": {
              "ngoai": {"command": "python", "args": ["s.py"], "env": {"K": "v"}},
              "tin": {"url": "http://127.0.0.1:9/mcp", "trusted": true, "headers": {"Authorization": "Bearer x"}},
              "tat": {"command": "x", "disabled": true},
              "rong": {}
            }}
            """, _host, out string? error);

        Assert.Null(error);
        Assert.Equal(["office", "ngoai", "tin"], configs.Select(c => c.Name));
        McpServerConfig office = configs[0];
        Assert.True(office.BuiltIn && office.Trusted);
        Assert.Equal(["mcp"], office.Args);
        Assert.Contains("excel_read", office.AllowedTools!);
        Assert.DoesNotContain("word_command", office.AllowedTools!);
        Assert.DoesNotContain("word_save", office.AllowedTools!);
        Assert.False(configs[1].Trusted);
        Assert.Equal("v", configs[1].Env["K"]);
        Assert.True(configs[2].Trusted);
        Assert.Equal("http://127.0.0.1:9/mcp", configs[2].Url);
    }

    [Fact]
    public void Tat_office_bang_disabled_va_json_loi_van_giu_office()
    {
        Assert.Empty(McpManager.LoadConfigs("""{"mcpServers": {"office": {"disabled": true}}}""", _host, out _));
        IReadOnlyList<McpServerConfig> broken = McpManager.LoadConfigs("{khong phai json", _host, out string? error);
        Assert.NotNull(error);
        Assert.Equal(["office"], broken.Select(c => c.Name));
        Assert.Empty(McpManager.LoadConfigs(null, Path.Combine(Path.GetTempPath(), "khong-co-host.exe"), out _));
    }

    [Theory]
    [InlineData("mau", "add", "mcp__mau__add")]
    [InlineData("my server", "do.thing", "mcp__my_server__do_thing")]
    public void Ten_tool_an_toan(string server, string tool, string expected)
    {
        Assert.Equal(expected, McpTool.ToolName(server, tool));
        Assert.True(McpTool.ToolName(new string('s', 40), new string('t', 40)).Length <= 64);
    }

    [Fact]
    public void Tool_chi_doc_cua_lan_file_la_tap_con_cua_tool_duoc_mo()
    {
        Assert.True(McpManager.OfficeReadOnlyTools.IsSubsetOf(McpManager.OfficeFileTools));
    }
}

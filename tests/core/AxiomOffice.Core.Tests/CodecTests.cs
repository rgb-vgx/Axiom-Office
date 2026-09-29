using System.Net;
using System.Text.Json.Nodes;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Tests;

// Codec cua hai giao thuc: dung body, doc response, ghep lich su (New_arch.md muc 8.2, 15.3).
public class CodecTests
{
    private static readonly ModelTool Tool = new(
        "office_action",
        "doc va sua tai lieu",
        new JsonObject { ["type"] = "object", ["properties"] = new JsonObject { ["action"] = new JsonObject { ["type"] = "string" } } });

    [Fact]
    public void OpenAi_dung_body_co_tools_va_khong_co_khi_tat()
    {
        var codec = new OpenAiCodec();
        List<JsonNode> turns = [new JsonObject { ["role"] = "user", ["content"] = "xin chao" }];

        JsonObject withTools = codec.BuildRequest("gpt-x", "system", turns, [Tool], toolsEnabled: true, maxTokens: 4096);
        JsonObject withoutTools = codec.BuildRequest("gpt-x", "system", turns, [Tool], toolsEnabled: false, maxTokens: 4096);

        Assert.Equal("gpt-x", withTools["model"]!.GetValue<string>());
        Assert.Equal("system", withTools["messages"]![0]!["content"]!.GetValue<string>());
        Assert.Equal("function", withTools["tools"]![0]!["type"]!.GetValue<string>());
        Assert.Equal("office_action", withTools["tools"]![0]!["function"]!["name"]!.GetValue<string>());
        Assert.Null(withoutTools["tools"]);
        Assert.Equal(4096, withTools["max_tokens"]!.GetValue<int>());
    }

    [Fact]
    public void OpenAi_doc_tool_call_va_cau_tra_loi()
    {
        var codec = new OpenAiCodec();

        ModelTurn? withCall = codec.Parse(ScriptedHandler.OpenAiToolCall("et.writeRange", """{"range":"A1"}"""), out string? error);
        Assert.Null(error);
        Assert.NotNull(withCall);
        Assert.Single(withCall!.ToolCalls);
        Assert.Equal("et.writeRange", withCall.ToolCalls[0].Name);
        Assert.Equal("""{"range":"A1"}""", withCall.ToolCalls[0].ArgumentsJson);
        Assert.Equal(100, withCall.InputTokens);
        Assert.Equal(20, withCall.OutputTokens);

        ModelTurn? text = codec.Parse(ScriptedHandler.OpenAiText("Da xong"), out error);
        Assert.NotNull(text);
        Assert.Equal("Da xong", text!.Text);
        Assert.Empty(text.ToolCalls);

        Assert.Null(codec.Parse("{ khong phai json", out error));
        Assert.NotNull(error);
    }

    [Fact]
    public void OpenAi_ghep_assistant_va_tool_result_dung_dinh_dang()
    {
        var codec = new OpenAiCodec();
        var turns = new List<JsonNode>();
        ModelTurn turn = codec.Parse(ScriptedHandler.OpenAiToolCall("et.readRange", "{}"), out _)!;

        codec.AppendAssistant(turns, turn);
        codec.AppendToolResults(turns, [new ToolCallResult("call_1", "office_action", """{"ok":true}""", true, 12)]);

        // assistant giu nguyen tool_calls de lan sau gui lai y nguyen.
        Assert.Equal("assistant", turns[0]!["role"]!.GetValue<string>());
        Assert.NotNull(turns[0]!["tool_calls"]);
        Assert.Equal("tool", turns[1]!["role"]!.GetValue<string>());
        Assert.Equal("call_1", turns[1]!["tool_call_id"]!.GetValue<string>());
        Assert.Equal("""{"ok":true}""", turns[1]!["content"]!.GetValue<string>());
    }

    [Fact]
    public void Anthropic_dung_body_system_rieng_va_input_schema()
    {
        var codec = new AnthropicCodec();
        List<JsonNode> turns = [new JsonObject { ["role"] = "user", ["content"] = "xin chao" }];

        JsonObject body = codec.BuildRequest("claude-x", "system prompt", turns, [Tool], toolsEnabled: true, maxTokens: 4096);

        Assert.Equal("claude-x", body["model"]!.GetValue<string>());
        Assert.Equal("system prompt", body["system"]!.GetValue<string>());
        Assert.Equal(4096, body["max_tokens"]!.GetValue<int>());
        Assert.Equal("office_action", body["tools"]![0]!["name"]!.GetValue<string>());
        Assert.NotNull(body["tools"]![0]!["input_schema"]);
        // Anthropic khong co message role=system trong messages.
        Assert.All(body["messages"]!.AsArray(), m => Assert.NotEqual("system", m!["role"]!.GetValue<string>()));
    }

    [Fact]
    public void Anthropic_doc_text_va_tool_use_trong_content_blocks()
    {
        var codec = new AnthropicCodec();

        ModelTurn? withCall = codec.Parse(ScriptedHandler.AnthropicToolCall("wpp.addSlide", """{"layout":12}"""), out string? error);
        Assert.Null(error);
        Assert.NotNull(withCall);
        Assert.Equal("Dang lam...", withCall!.Text);
        Assert.Single(withCall.ToolCalls);
        Assert.Equal("wpp.addSlide", withCall.ToolCalls[0].Name);
        Assert.Equal("""{"layout":12}""", withCall.ToolCalls[0].ArgumentsJson);
        Assert.Equal(120, withCall.InputTokens);

        ModelTurn? text = codec.Parse(ScriptedHandler.AnthropicText("Xong"), out error);
        Assert.NotNull(text);
        Assert.Equal("Xong", text!.Text);
        Assert.Empty(text.ToolCalls);

        Assert.Null(codec.Parse("{}", out error));
        Assert.NotNull(error);
    }

    [Fact]
    public void Anthropic_ghep_tool_result_thanh_mot_message_user()
    {
        var codec = new AnthropicCodec();
        var turns = new List<JsonNode>();
        ModelTurn turn = codec.Parse(ScriptedHandler.AnthropicToolCall("et.readRange", "{}"), out _)!;

        codec.AppendAssistant(turns, turn);
        codec.AppendToolResults(turns, [
            new ToolCallResult("toolu_1", "office_action", """{"ok":true}""", true, 5),
            new ToolCallResult("toolu_2", "office_action", """{"ok":false}""", false, 6),
        ]);

        Assert.Equal("assistant", turns[0]!["role"]!.GetValue<string>());
        Assert.Equal("user", turns[1]!["role"]!.GetValue<string>());
        JsonArray blocks = turns[1]!["content"]!.AsArray();
        Assert.Equal(2, blocks.Count);
        Assert.Equal("tool_result", blocks[0]!["type"]!.GetValue<string>());
        Assert.Equal("toolu_1", blocks[0]!["tool_use_id"]!.GetValue<string>());
        Assert.Equal("toolu_2", blocks[1]!["tool_use_id"]!.GetValue<string>());
    }

    [Theory]
    [InlineData("https://api.openai.com/v1", "/chat/completions", "https://api.openai.com/v1/chat/completions")]
    [InlineData("https://api.openai.com/v1/chat/completions", "/chat/completions", "https://api.openai.com/v1/chat/completions")]
    [InlineData("http://localhost:11434/v1/", "/chat/completions", "http://localhost:11434/v1/chat/completions")]
    public void BuildUrl_khong_noi_lap_hau_to(string endpoint, string suffix, string expected)
    {
        Assert.Equal(expected, ModelClient.BuildUrl(endpoint, suffix));
    }

    [Theory]
    [InlineData("api.example.com/v1", "http://api.example.com/v1")]
    [InlineData("https://api.example.com/v1", "https://api.example.com/v1")]
    [InlineData("  ", "")]
    public void NormalizeEndpoint_them_http_khi_thieu(string endpoint, string expected)
    {
        Assert.Equal(expected, ModelClient.NormalizeEndpoint(endpoint));
    }
}

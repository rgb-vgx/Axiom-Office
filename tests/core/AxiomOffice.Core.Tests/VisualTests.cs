using System.Text.Json.Nodes;
using AxiomOffice.Core.Models;
using AxiomOffice.Core.Tools;

namespace AxiomOffice.Core.Tests;

// QA thi giac (New_arch.md muc 8.4.6): anh chup gui cho model dung dinh dang tung nha cung cap.
public class VisualTests
{
    private const string DataUrl = "data:image/png;base64,iVBORw0KGgo=";

    [Fact]
    public void OpenAi_gui_anh_bang_tin_user_image_url_sau_ket_qua_tool()
    {
        var turns = new List<JsonNode>();
        new OpenAiCodec().AppendToolResults(turns,
        [
            new ToolCallResult("c1", "look_at_document", """{"ok":true}""", true, 0, DataUrl),
            new ToolCallResult("c2", "office_action", """{"ok":true}""", true, 0),
        ]);

        Assert.Equal(["tool", "tool", "user"], turns.Select(t => t["role"]!.GetValue<string>()));
        JsonArray content = turns[2]["content"]!.AsArray();
        Assert.Equal(DataUrl, content[1]!["image_url"]!["url"]!.GetValue<string>());
        Assert.DoesNotContain("base64", turns[0]["content"]!.GetValue<string>());
    }

    [Fact]
    public void Anthropic_gui_anh_trong_tool_result()
    {
        var turns = new List<JsonNode>();
        new AnthropicCodec().AppendToolResults(turns, [new ToolCallResult("t1", "look_at_document", """{"ok":true}""", true, 0, DataUrl)]);

        JsonNode block = turns[0]["content"]![0]!;
        Assert.Equal("tool_result", block["type"]!.GetValue<string>());
        JsonNode image = block["content"]![1]!;
        Assert.Equal("image", image["type"]!.GetValue<string>());
        Assert.Equal("image/png", image["source"]!["media_type"]!.GetValue<string>());
        Assert.Equal("iVBORw0KGgo=", image["source"]!["data"]!.GetValue<string>());
    }

    [Theory]
    [InlineData("data:image/jpeg;base64,/9j/", true, "image/jpeg")]
    [InlineData("iVBORw0KGgo=", false, "")]
    [InlineData("data:;base64,xx", false, "")]
    public void Tach_data_url(string value, bool ok, string mediaType)
    {
        Assert.Equal(ok, ImageData.TrySplit(value, out string type, out _));
        Assert.Equal(mediaType, type);
    }
}

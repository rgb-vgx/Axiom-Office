using AxiomOffice.Core.Agent;
using AxiomOffice.Core.Memory;

namespace AxiomOffice.Core.Tests;

// Ngan sach ngu canh (New_arch.md muc 8.5.9): lay cac luot gan nhat vua ngan sach, phan cu hon dua vao
// tom tat; gom cac dong tom tat tool thanh danh sach hanh dong.
public class ContextAssemblerTests
{
    private static MessageRow Message(int seq, string role, string content)
    {
        return new MessageRow(seq, seq, role, content, "2026-10-01T00:00:00.000Z");
    }

    private static ConversationRow Conversation(string? summary)
    {
        return new ConversationRow("c_1", @"c:\a.docx", "a.docx", "wps", "office", "tieu de", summary, "t0", "t1");
    }

    [Fact]
    public void Giu_dung_thu_tu_va_bo_tom_tat_tool_khoi_luot()
    {
        var assembler = new ContextAssembler(budgetTokens: 8000);
        var messages = new List<MessageRow>
        {
            Message(1, "user", "tao bang"),
            Message(2, "tool_summary", "et.writeRange (ok)"),
            Message(3, "assistant", "da tao xong"),
        };

        AssembledContext context = assembler.Assemble(Conversation(null), messages);

        Assert.Equal(2, context.PriorTurns.Count);
        Assert.Equal("user", context.PriorTurns[0].Role);
        Assert.Equal("assistant", context.PriorTurns[1].Role);
        Assert.Single(context.RecentActionLines);
        Assert.Equal("et.writeRange (ok)", context.RecentActionLines[0]);
        Assert.Null(context.Summary);
    }

    [Fact]
    public void Vuot_ngan_sach_thi_chi_giu_phan_moi_va_dung_tom_tat()
    {
        var assembler = new ContextAssembler(budgetTokens: 30);
        var messages = new List<MessageRow>();
        for (int i = 1; i <= 10; i++)
        {
            messages.Add(Message(i, i % 2 == 1 ? "user" : "assistant", new string('x', 60) + " tin " + i));
        }

        AssembledContext context = assembler.Assemble(Conversation("tom tat cu"), messages);

        Assert.Equal("tom tat cu", context.Summary);
        Assert.True(context.PriorTurns.Count < 10, "phai cat bot luot cu");
        Assert.True(context.EstimatedTokens <= 30, "khong duoc vuot ngan sach");
        Assert.All(context.PriorTurns.Skip(1), turn => Assert.NotEqual("...", turn.Content));
    }

    [Fact]
    public void Bo_luot_assistant_mo_dau_khi_bi_cat_le()
    {
        var assembler = new ContextAssembler(budgetTokens: 25);
        var messages = new List<MessageRow>
        {
            Message(1, "user", new string('a', 200)),
            Message(2, "assistant", new string('b', 60)),
            Message(3, "user", "cau hoi moi"),
        };

        AssembledContext context = assembler.Assemble(Conversation(null), messages);

        Assert.NotEmpty(context.PriorTurns);
        Assert.Equal("user", context.PriorTurns[0].Role);
    }

    [Fact]
    public void Chi_lay_toi_da_10_dong_tom_tat_hanh_dong()
    {
        var assembler = new ContextAssembler();
        var messages = new List<MessageRow>();
        for (int i = 1; i <= 15; i++)
        {
            messages.Add(Message(i, "tool_summary", "action " + i + " (ok)"));
        }

        AssembledContext context = assembler.Assemble(Conversation(null), messages, maxActionLines: 10);

        Assert.Equal(10, context.RecentActionLines.Count);
        // Lay 10 dong moi nhat, van theo thu tu thoi gian.
        Assert.Equal("action 6 (ok)", context.RecentActionLines[0]);
        Assert.Equal("action 15 (ok)", context.RecentActionLines[^1]);
    }

    [Theory]
    [InlineData("", 0)]
    [InlineData("abcd", 1)]
    [InlineData("1234567890123456", 4)]
    public void Uoc_luong_token_theo_ky_tu_chia_4(string text, int expected)
    {
        Assert.Equal(expected, ContextAssembler.EstimateTokens(text));
    }
}

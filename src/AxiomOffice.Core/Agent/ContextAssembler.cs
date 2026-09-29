using AxiomOffice.Core.Memory;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Agent;

// Ngu canh dua vao prompt: tom tat (neu co) + cac luot user/assistant gan nhat trong ngan sach token,
// va danh sach hanh dong da chay (tom tat tool) (New_arch.md muc 8.5.9).
public sealed record AssembledContext(
    string? Summary,
    IReadOnlyList<ConversationTurn> PriorTurns,
    IReadOnlyList<string> RecentActionLines,
    int EstimatedTokens);

public sealed class ContextAssembler(int budgetTokens = 8000)
{
    public int BudgetTokens { get; } = budgetTokens;

    public static int EstimateTokens(string text)
    {
        // Uoc luong ky tu/4 (muc 8.5.9) - du dung cho tieng Viet va tieng Anh.
        return (text.Length + 3) / 4;
    }

    public AssembledContext Assemble(ConversationRow? conversation, IReadOnlyList<MessageRow> messages, int maxActionLines = 10)
    {
        var turns = new List<ConversationTurn>();
        var actions = new List<string>();
        int tokens = 0;
        bool overBudget = false;

        for (int i = messages.Count - 1; i >= 0; i--)
        {
            MessageRow message = messages[i];
            if (message.Role == "tool_summary")
            {
                if (actions.Count < maxActionLines)
                {
                    actions.Add(message.Content);
                }

                continue;
            }

            if (message.Role != "user" && message.Role != "assistant")
            {
                continue;
            }

            int cost = EstimateTokens(message.Content) + 4;
            if (!overBudget && tokens + cost > BudgetTokens)
            {
                // Het ngan sach: phan con lai chi con trong ban tom tat.
                overBudget = true;
            }

            if (overBudget)
            {
                continue;
            }

            tokens += cost;
            turns.Add(new ConversationTurn(message.Role, message.Content));
        }

        turns.Reverse();
        // Giao thuc Anthropic yeu cau message dau tien la user: bo luot assistant mo dau neu bi cat le.
        if (turns.Count > 0 && turns[0].Role != "user")
        {
            turns.RemoveAt(0);
        }

        actions.Reverse();
        return new AssembledContext(overBudget ? conversation?.Summary : conversation?.Summary, turns, actions, tokens);
    }

    // Cau hoi tom tat khi hoi thoai vuot ngan sach (goi mot lan cuoi luot chay, muc 8.5.9).
    public static string SummaryPrompt(int messageCount)
    {
        return $"Summarize the conversation above in at most 120 words, in the user's language. "
            + $"Keep: what the user asked for, what was already done to the document, decisions, and open items. "
            + $"The conversation has {messageCount} messages; the older ones are being dropped from context.";
    }
}

using System.Text.RegularExpressions;

namespace AxiomOffice.Core.Models;

// Xu ly van ban tra loi cua model truoc khi dua cho nguoi dung.
public static partial class ModelText
{
    // Mot so model (Gemma 4 qua endpoint OpenAI-compatible cua Google, DeepSeek/Qwen...) tra ca phan suy nghi
    // trong content: <thought>...</thought> / <think>...</think>. Bo di o cau tra loi cuoi; ban tin assistant
    // gui lai cho model trong vong lap van giu nguyen (codec khong doi).
    public static string? StripThoughts(string? text)
    {
        if (string.IsNullOrEmpty(text))
        {
            return text;
        }

        string stripped = ThoughtBlock().Replace(text, "");
        return stripped.Trim();
    }

    [GeneratedRegex(@"<(thought|think|thinking)>[\s\S]*?</\1>", RegexOptions.IgnoreCase)]
    private static partial Regex ThoughtBlock();
}

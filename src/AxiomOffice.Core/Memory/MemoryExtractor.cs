using System.Globalization;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using System.Text.RegularExpressions;
using AxiomOffice.Core.Models;

namespace AxiomOffice.Core.Memory;

// Dau vao cua mot lan trich xuat sau run (New_arch.md muc 8.5.5). KHONG chua noi dung tai lieu hay ket qua
// getText/readRange - chi tin nhan cua nguoi dung, cau tra loi cuoi va tin nhan gan nhat.
public sealed record ExtractionInput(
    string Prompt,
    string Reply,
    IReadOnlyList<(string Role, string Content)> Recent,
    IReadOnlyList<MemoryItem> Related,
    DateOnly ObservedOn,
    string? DocumentKey);

// Trich xuat CHI-ADD (hoc mem0 2.2.1): mot lenh goi LLM, tra fact moi; C# kiem tra lai moi thu.
public static partial class MemoryExtractor
{
    public const double MinConfidence = 0.6;
    public const int MaxFacts = 5;
    public const int MinPromptLength = 15;
    public const int MaxRecentMessages = 10;
    public const int MaxRelatedMemories = 10;

    private static readonly Lazy<string> SystemPromptText = new(() =>
    {
        using Stream stream = typeof(MemoryExtractor).Assembly.GetManifestResourceStream("AxiomOffice.Core.Memory.Prompts.extract.txt")
            ?? throw new InvalidOperationException("missing embedded extract.txt");
        using var reader = new StreamReader(stream, Encoding.UTF8);
        return reader.ReadToEnd();
    });

    public static string SystemPrompt => SystemPromptText.Value;

    // Bo qua (khong ton lenh goi LLM) khi prompt qua ngan hoac chi la lenh thao tac thuan khong mang thong tin
    // ve nguoi dung / to chuc / tien do (vd "in dam dong nay").
    public static bool ShouldSkip(string prompt, out string reason)
    {
        string text = (prompt ?? "").Trim();
        if (text.Length < MinPromptLength)
        {
            reason = "prompt too short";
            return true;
        }

        string normalized = MemoryText.Normalize(text);
        if (InfoMarker().IsMatch(normalized))
        {
            reason = "";
            return false;
        }

        if (OperationVerb().IsMatch(normalized) && text.Length < 160)
        {
            reason = "pure document operation";
            return true;
        }

        reason = "";
        return false;
    }

    // Tu chi thao tac tren tai lieu (da chuan hoa khong dau).
    [GeneratedRegex(@"^(in dam|in nghieng|gach chan|can (giua|trai|phai|deu)|xoa|chen|them|doi mau|to mau|sua|dinh dang|luu|mo|dong|hoan tac|undo|copy|sao chep|di chuyen|thay|tao|ve|ke|lam|viet|dien|ghi|doc|tom tat|dich|sap xep|loc|tinh|chuyen|doi)\b")]
    private static partial Regex OperationVerb();

    // Dau hieu co thong tin dang nho: ve ban than, to chuc, thoi quen, tien do.
    [GeneratedRegex(@"\b(toi la|minh la|em la|chung toi|co quan|cong ty|to chuc|don vi|phong ban|chuc vu|truong phong|pho giam doc|giam doc|nguoi ky|ky ten|luon|thuong|thich|uu tien|mac dinh|lan sau|tu nay|nho rang|ghi nho|da xong|con thieu|tien do|han nop|deadline|email|so dien thoai)\b")]
    private static partial Regex InfoMarker();

    // Noi dung gui LLM + bang id tam -> id that (model khong thay id that: chong bia id).
    public static string BuildUserMessage(ExtractionInput input, out Dictionary<string, string> tempIds)
    {
        tempIds = new Dictionary<string, string>(StringComparer.Ordinal);
        var builder = new StringBuilder();
        builder.Append("Ngày quan sát: ").Append(input.ObservedOn.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture)).Append('\n');
        builder.Append("Tài liệu đang mở: ").Append(input.DocumentKey == null ? "(chưa lưu)" : Path.GetFileName(input.DocumentKey)).Append("\n\n");

        builder.Append("Ghi nhớ liên quan (id tạm: nội dung):\n");
        int index = 0;
        foreach (MemoryItem item in input.Related.Take(MaxRelatedMemories))
        {
            string temp = index.ToString(CultureInfo.InvariantCulture);
            tempIds[temp] = item.Id;
            builder.Append(temp).Append(": ").Append(item.Text).Append('\n');
            index++;
        }

        if (index == 0)
        {
            builder.Append("(không có)\n");
        }

        builder.Append("\nTin nhắn gần nhất trong hội thoại:\n");
        foreach ((string role, string content) in input.Recent.TakeLast(MaxRecentMessages))
        {
            builder.Append(role == "user" ? "Người dùng: " : "Trợ lý: ").Append(ModelClient.Truncate(content, 500)).Append('\n');
        }

        builder.Append("\nLượt vừa xong:\nNgười dùng: ").Append(ModelClient.Truncate(input.Prompt, 2000)).Append('\n');
        builder.Append("Trợ lý: ").Append(ModelClient.Truncate(input.Reply, 1000)).Append('\n');
        builder.Append("\nTrả về JSON {\"facts\": [...]}.");
        return builder.ToString();
    }

    // Kiem tra dau ra cua LLM (muc 8.5.5): bo fact confidence < 0.6, rong, > 300 ky tu, nhay cam; bo moi
    // linkedIds khong nam trong bang id tam (van giu fact); expiresAt phai la YYYY-MM-DD.
    public static IReadOnlyList<NewMemory>? ParseFacts(string? output, IReadOnlyDictionary<string, string> tempIds, string? documentKey, out string? error)
    {
        error = null;
        JsonNode? root;
        try
        {
            root = JsonNode.Parse(StripFence(output ?? ""));
        }
        catch (JsonException ex)
        {
            error = "invalid JSON: " + ex.Message;
            return null;
        }

        if (root?["facts"] is not JsonArray facts)
        {
            error = "missing 'facts' array";
            return null;
        }

        var result = new List<NewMemory>();
        foreach (JsonNode? fact in facts)
        {
            if (fact is not JsonObject obj || result.Count >= MaxFacts)
            {
                continue;
            }

            string text = (Str(obj["text"]) ?? "").Trim();
            double confidence = Num(obj["confidence"]) ?? 0.7;
            if (text.Length == 0 || text.Length > MemoryText.MaxFactLength || confidence < MinConfidence || MemoryText.IsSensitive(text))
            {
                continue;
            }

            string? category = Str(obj["category"]);
            string scope = Str(obj["scope"]) == MemoryScopes.Document || (Str(obj["scope"]) == null && category is "progress")
                ? MemoryScopes.Document
                : MemoryScopes.User;
            if (scope == MemoryScopes.Document && documentKey == null)
            {
                scope = MemoryScopes.User;
            }

            var linked = new List<string>();
            if (obj["linkedIds"] is JsonArray ids)
            {
                foreach (JsonNode? id in ids)
                {
                    string? temp = id is JsonValue value ? value.ToString() : null;
                    if (temp != null && tempIds.TryGetValue(temp, out string? real))
                    {
                        linked.Add(real);
                    }
                }
            }

            var entities = new List<string>();
            if (obj["entities"] is JsonArray list)
            {
                entities.AddRange(list.Select(Str).Where(e => !string.IsNullOrWhiteSpace(e))!);
            }

            result.Add(new NewMemory(scope, scope == MemoryScopes.Document ? documentKey : null, text, category, entities, linked,
                MemoryText.ValidDate(Str(obj["expiresAt"])), confidence));
        }

        return result;
    }

    // Goi LLM (timeout 30s); JSON loi thi thu lai 1 lan voi "chi tra JSON".
    public static async Task<(IReadOnlyList<NewMemory>? Facts, string? Error)> ExtractAsync(ModelClient model, ExtractionInput input, CancellationToken cancel)
    {
        string user = BuildUserMessage(input, out Dictionary<string, string> tempIds);
        string? lastError = null;
        for (int attempt = 0; attempt < 2; attempt++)
        {
            using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancel);
            timeout.CancelAfter(TimeSpan.FromSeconds(30));
            string message = attempt == 0 ? user : user + "\n\nCHỈ trả về một đối tượng JSON hợp lệ, không có chữ nào khác.";
            (string? text, string? error) = await model.ChatAsync(SystemPrompt, message, timeout.Token).ConfigureAwait(false);
            if (text == null)
            {
                return (null, error ?? "no reply");
            }

            IReadOnlyList<NewMemory>? facts = ParseFacts(text, tempIds, input.DocumentKey, out lastError);
            if (facts != null)
            {
                return (facts, null);
            }
        }

        return (null, lastError);
    }

    private static string StripFence(string text)
    {
        string trimmed = text.Trim();
        if (trimmed.StartsWith("```", StringComparison.Ordinal))
        {
            int firstLine = trimmed.IndexOf('\n');
            int lastFence = trimmed.LastIndexOf("```", StringComparison.Ordinal);
            if (firstLine > 0 && lastFence > firstLine)
            {
                trimmed = trimmed[(firstLine + 1)..lastFence];
            }
        }

        int start = trimmed.IndexOf('{');
        int end = trimmed.LastIndexOf('}');
        return start >= 0 && end > start ? trimmed[start..(end + 1)] : trimmed;
    }

    private static string? Str(JsonNode? node)
    {
        return node is JsonValue value && value.TryGetValue(out string? text) ? text : null;
    }

    private static double? Num(JsonNode? node)
    {
        if (node is not JsonValue value)
        {
            return null;
        }

        if (value.TryGetValue(out double number))
        {
            return number;
        }

        return value.TryGetValue(out string? text) && double.TryParse(text, NumberStyles.Float, CultureInfo.InvariantCulture, out double parsed) ? parsed : null;
    }
}

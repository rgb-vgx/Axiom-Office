using System.Globalization;
using System.Security.Cryptography;
using System.Text;
using System.Text.RegularExpressions;

namespace AxiomOffice.Core.Memory;

// Xu ly text cho memory (New_arch.md muc 8.5.5 - 8.5.7): chuan hoa (chu thuong, bo dau tieng Viet ke ca
// 'đ', bo dau cau), hash chong trung, dung truy van FTS5, loc thong tin nhay cam.
public static partial class MemoryText
{
    public const int MaxFactLength = 300;
    public const int MaxQueryTerms = 12;

    // Chu thuong, bo dau (NFD bo dau ket hop; 'đ' khong tach duoc nen doi tay), bo dau cau, gop khoang trang.
    public static string Normalize(string? text)
    {
        if (string.IsNullOrWhiteSpace(text))
        {
            return "";
        }

        string decomposed = text.ToLowerInvariant().Replace('đ', 'd').Replace('Đ', 'd').Normalize(NormalizationForm.FormD);
        var builder = new StringBuilder(decomposed.Length);
        bool space = false;
        foreach (char c in decomposed)
        {
            UnicodeCategory category = CharUnicodeInfo.GetUnicodeCategory(c);
            if (category == UnicodeCategory.NonSpacingMark)
            {
                continue;
            }

            if (char.IsLetterOrDigit(c))
            {
                builder.Append(c);
                space = false;
            }
            else if (!space && builder.Length > 0)
            {
                builder.Append(' ');
                space = true;
            }
        }

        return builder.ToString().Trim().Normalize(NormalizationForm.FormC);
    }

    public static string Hash(string text)
    {
        byte[] digest = SHA256.HashData(Encoding.UTF8.GetBytes(Normalize(text)));
        return Convert.ToHexString(digest).ToLowerInvariant();
    }

    // Tu dung (da chuan hoa) - bo khi dung truy van tu prompt.
    private static readonly HashSet<string> StopWords = new(StringComparer.Ordinal)
    {
        // tieng Viet
        "va", "voi", "cua", "cho", "la", "thi", "ma", "nhung", "cac", "mot", "nhieu", "nay", "do", "kia", "dang",
        "da", "se", "duoc", "bi", "tai", "trong", "ngoai", "tren", "duoi", "den", "tu", "ve", "theo", "nhu", "hay",
        "hoac", "neu", "khi", "de", "vao", "ra", "lai", "cung", "rat", "qua", "nua", "roi", "a", "nhe", "oi", "giup",
        "minh", "toi", "ban", "hay", "vui", "long", "xin", "can", "phai", "co", "khong", "gi", "nao", "sao", "the",
        "lam", "viet", "tao", "them", "sua", "bo", "o", "day", "ay", "vay", "thoi",
        // tieng Anh
        "the", "a", "an", "and", "or", "of", "to", "in", "on", "for", "with", "is", "are", "be", "this", "that",
        "it", "my", "me", "i", "you", "please", "can", "make", "add", "from", "at", "by", "as",
    };

    public static IReadOnlyList<string> QueryTerms(string? text)
    {
        return Normalize(text)
            .Split(' ', StringSplitOptions.RemoveEmptyEntries)
            .Where(w => w.Length > 1 && !StopWords.Contains(w))
            .Distinct(StringComparer.Ordinal)
            .Take(MaxQueryTerms)
            .ToList();
    }

    // "w1" OR "w2" ... (moi tu trong nhay kep: khong de ky tu dac biet cua FTS5 lam hong truy van).
    public static string? FtsQuery(IReadOnlyList<string> terms)
    {
        return terms.Count == 0 ? null : string.Join(" OR ", terms.Select(t => "\"" + t.Replace("\"", "") + "\""));
    }

    // Thong tin nhay cam khong duoc luu vao memory du LLM noi gi (muc 8.5.5): day 9-12 chu so lien (CCCD,
    // tai khoan), so the, mat khau, API key/token.
    public static bool IsSensitive(string text)
    {
        return LongDigits().IsMatch(text) || CardNumber().IsMatch(text) || Secret().IsMatch(text) || ApiKey().IsMatch(text);
    }

    [GeneratedRegex(@"(?<!\d)\d{9,12}(?!\d)")]
    private static partial Regex LongDigits();

    [GeneratedRegex(@"(?<!\d)(?:\d[ -]?){13,19}(?!\d)")]
    private static partial Regex CardNumber();

    [GeneratedRegex(@"(mật\s*khẩu|mat\s*khau|password|passwd|pass\s*:|m[aậ]t\s*m[aã]\s*pin|\bpin\s*:|otp)", RegexOptions.IgnoreCase)]
    private static partial Regex Secret();

    [GeneratedRegex(@"\b(sk-[A-Za-z0-9_-]{16,}|AIza[0-9A-Za-z_-]{20,}|AQ\.[0-9A-Za-z_-]{20,}|ghp_[A-Za-z0-9]{20,}|xox[bp]-[A-Za-z0-9-]{10,})")]
    private static partial Regex ApiKey();

    // "YYYY-MM-DD" hop le hoac null.
    public static string? ValidDate(string? value)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return null;
        }

        return DateOnly.TryParseExact(value.Trim(), "yyyy-MM-dd", CultureInfo.InvariantCulture, DateTimeStyles.None, out DateOnly date)
            ? date.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture)
            : null;
    }
}

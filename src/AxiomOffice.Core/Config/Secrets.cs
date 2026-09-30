using System.Security.Cryptography;
using System.Text;

namespace AxiomOffice.Core.Config;

// LlmApiKey trong HKCU duoc add-in ma hoa bang DPAPI (CurrentUser, khong entropy) duoi dang
// "dpapi:<base64>". Core giai ma y nguyen (New_arch.md muc 7.6). Gia tri khong ma hoa duoc giu nguyen.
public static class Secrets
{
    private const string Prefix = "dpapi:";

    public static string Unprotect(string? raw)
    {
        if (string.IsNullOrEmpty(raw))
        {
            return "";
        }

        if (!raw.StartsWith(Prefix, StringComparison.Ordinal))
        {
            return raw;
        }

        if (!OperatingSystem.IsWindows())
        {
            // Khoa DPAPI chi giai ma duoc tren dung may Windows cua nguoi dung: coi nhu chua cau hinh.
            return "";
        }

        try
        {
            byte[] encrypted = Convert.FromBase64String(raw[Prefix.Length..]);
            byte[] plain = ProtectedData.Unprotect(encrypted, null, DataProtectionScope.CurrentUser);
            return Encoding.UTF8.GetString(plain);
        }
        catch
        {
            // Khoa cua nguoi dung khac / du lieu hong: coi nhu chua cau hinh, khong lam Core dung.
            return "";
        }
    }

    public static string Protect(string value)
    {
        if (string.IsNullOrEmpty(value))
        {
            return "";
        }

        if (!OperatingSystem.IsWindows())
        {
            // Linux: config.json quyen 0600 cua nguoi dung la lop bao ve (LibreOffice_arch.md muc 9).
            return value;
        }

        byte[] encrypted = ProtectedData.Protect(Encoding.UTF8.GetBytes(value), null, DataProtectionScope.CurrentUser);
        return Prefix + Convert.ToBase64String(encrypted);
    }
}

using System.Diagnostics;
using System.Security.Cryptography;
using System.Text;

namespace AxiomOffice.Core.Config;

// LlmApiKey duoc ma hoa truoc khi ghi cau hinh:
//   - Windows: DPAPI (CurrentUser, khong entropy) -> "dpapi:<base64>" (add-in ghi, New_arch.md muc 7.6).
//   - Linux: libsecret qua `secret-tool` (neu co) -> "libsecret:<ten khoa>"; khong co thi giu gia tri
//     thuong trong config.json quyen 0600 (LibreOffice_arch.md muc 9).
// Gia tri khong ma hoa van duoc chap nhan; giai ma that bai (khoa cua may khac, keyring khoa) coi nhu chua
// cau hinh chu khong lam Core dung.
public static class Secrets
{
    private const string DpapiPrefix = "dpapi:";
    public const string LibsecretPrefix = "libsecret:";
    public const string ServiceName = "axiom-office";

    public static string Unprotect(string? raw)
    {
        if (string.IsNullOrEmpty(raw))
        {
            return "";
        }

        if (raw.StartsWith(LibsecretPrefix, StringComparison.Ordinal))
        {
            return LibsecretLookup(raw[LibsecretPrefix.Length..]) ?? "";
        }

        if (!raw.StartsWith(DpapiPrefix, StringComparison.Ordinal))
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
            byte[] encrypted = Convert.FromBase64String(raw[DpapiPrefix.Length..]);
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
            // Linux: khong co keyring thi config.json quyen 0600 cua nguoi dung la lop bao ve.
            return value;
        }

        byte[] encrypted = ProtectedData.Protect(Encoding.UTF8.GetBytes(value), null, DataProtectionScope.CurrentUser);
        return DpapiPrefix + Convert.ToBase64String(encrypted);
    }

    // Doc khoa tu libsecret. null = may khong co secret-tool / khong doc duoc (keyring khoa, chua luu).
    private static string? LibsecretLookup(string key)
    {
        if (OperatingSystem.IsWindows() || string.IsNullOrWhiteSpace(key))
        {
            return null;
        }

        try
        {
            using Process? process = Process.Start(new ProcessStartInfo("secret-tool")
            {
                ArgumentList = { "lookup", "service", ServiceName, "key", key },
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                RedirectStandardInput = true,
                UseShellExecute = false,
            });
            if (process is null)
            {
                return null;
            }

            string value = process.StandardOutput.ReadToEnd();
            process.StandardError.ReadToEnd();
            if (!process.WaitForExit(5000))
            {
                try
                {
                    process.Kill(true);
                }
                catch
                {
                    // Bo qua: chi la doc khoa.
                }
                return null;
            }

            return process.ExitCode == 0 ? value.TrimEnd('\n', '\r') : null;
        }
        catch
        {
            // Khong co secret-tool (hoac khong chay duoc): coi nhu chua cau hinh.
            return null;
        }
    }
}

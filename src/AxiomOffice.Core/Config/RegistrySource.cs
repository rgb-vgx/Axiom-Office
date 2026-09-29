using Microsoft.Extensions.Configuration;

namespace AxiomOffice.Core.Config;

// Nguon doc gia tri tu HKCU\Software\AxiomOffice (khoa dung chung voi add-in). Tach interface de
// test khong phai dong vao registry that (New_arch.md muc 7.6).
public interface IRegistrySource
{
    string? GetString(string name);

    int? GetInt(string name);
}

// Doc HKCU\Software\AxiomOffice. Moi loi truy cap (khoa khong ton tai, bi chan) tra ve null.
public sealed class RegistrySource(string keyPath) : IRegistrySource
{
    public string? GetString(string name)
    {
        try
        {
            using var key = Microsoft.Win32.Registry.CurrentUser.OpenSubKey(keyPath);
            return key?.GetValue(name)?.ToString();
        }
        catch
        {
            return null;
        }
    }

    public int? GetInt(string name)
    {
        string? raw = GetString(name);
        return int.TryParse(raw, out int value) ? value : null;
    }
}

using System.Reflection;

namespace AxiomOffice.Core;

// Version + giao thuc cua Core. Version lay tu assembly (build.ps1 truyen -p:Version=<version DLL>
// nen /health cua Core va bridge luon cung so). Protocol tang khi doi hop dong Core API (muc 7.3).
public static class CoreVersion
{
    public const int Protocol = 1;

    public static readonly string Value = Resolve();

    private static string Resolve()
    {
        Assembly assembly = typeof(CoreVersion).Assembly;
        string? informational = assembly.GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion;
        if (!string.IsNullOrEmpty(informational))
        {
            // Bo hau to "+<commit>" ma SDK tu them khi build tu git.
            int plus = informational.IndexOf('+');
            return plus > 0 ? informational[..plus] : informational;
        }
        Version? version = assembly.GetName().Version;
        return version == null ? "0.0.0" : version.ToString(3);
    }
}

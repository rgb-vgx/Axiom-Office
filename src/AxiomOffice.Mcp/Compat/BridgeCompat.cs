using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.Json;

namespace AxiomOffice.Bridge
{
    // Ban .NET 10 cua cac kieu ma tool MCP dung chung voi AxiomOffice.Host (net48): ban Windows lay tu
    // add-in (HKCU, %LOCALAPPDATA%), ban nay doc config.json + thu muc du lieu theo XDG tren Linux
    // (LibreOffice_arch.md muc 4) - cung vi tri voi Agent Core va extension LibreOffice.

    internal static class DataPaths
    {
        private static string Home(string name, string fallback)
        {
            string value = Environment.GetEnvironmentVariable(name);
            return string.IsNullOrEmpty(value) ? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), fallback) : value;
        }

        // Windows: %LOCALAPPDATA%\AxiomOffice (nhu add-in); Linux: $XDG_DATA_HOME/axiom-office.
        public static string Data
        {
            get
            {
                if (OperatingSystem.IsWindows())
                {
                    string local = Environment.GetEnvironmentVariable("LOCALAPPDATA");
                    return Path.Combine(string.IsNullOrEmpty(local) ? Environment.GetFolderPath(Environment.SpecialFolder.UserProfile) : local, "AxiomOffice");
                }
                return Path.Combine(Home("XDG_DATA_HOME", ".local/share"), "axiom-office");
            }
        }

        // Windows: %LOCALAPPDATA%\AxiomOffice (add-in ghi session o day); Linux: $XDG_RUNTIME_DIR/axiom-office.
        public static string Runtime
        {
            get
            {
                if (OperatingSystem.IsWindows())
                {
                    return Data;
                }
                string runtime = Environment.GetEnvironmentVariable("XDG_RUNTIME_DIR");
                return Path.Combine(string.IsNullOrEmpty(runtime) ? Home("HOME", ".") + "/.cache" : runtime, "axiom-office");
            }
        }

        public static string ConfigFile
        {
            get { return Path.Combine(Home("XDG_CONFIG_HOME", ".config"), "axiom-office", "config.json"); }
        }
    }

    // Cau hinh: HKCU\Software\AxiomOffice (Windows) / ~/.config/axiom-office/config.json (Linux).
    internal static class Config
    {
        private const string KeyPath = @"Software\AxiomOffice";
        private static readonly Lazy<Dictionary<string, object>> Json = new Lazy<Dictionary<string, object>>(ReadJson);

        private static Dictionary<string, object> ReadJson()
        {
            var map = new Dictionary<string, object>(StringComparer.OrdinalIgnoreCase);
            try
            {
                if (!File.Exists(DataPaths.ConfigFile))
                {
                    return map;
                }
                using (JsonDocument document = JsonDocument.Parse(File.ReadAllText(DataPaths.ConfigFile, Encoding.UTF8)))
                {
                    foreach (JsonProperty property in document.RootElement.EnumerateObject())
                    {
                        map[property.Name] = property.Value.ValueKind == JsonValueKind.String
                            ? property.Value.GetString()
                            : property.Value.Clone();
                    }
                }
            }
            catch (Exception)
            {
                // File dang ghi do / hong: coi nhu chua cau hinh, khong lam MCP dung.
            }
            return map;
        }

        private static string Value(string name)
        {
            if (OperatingSystem.IsWindows())
            {
                try
                {
                    using (Microsoft.Win32.RegistryKey key = Microsoft.Win32.Registry.CurrentUser.OpenSubKey(KeyPath))
                    {
                        object value = key == null ? null : key.GetValue(name);
                        if (value != null)
                        {
                            return Convert.ToString(value, CultureInfo.InvariantCulture);
                        }
                    }
                }
                catch (Exception)
                {
                }
            }
            object found;
            if (!Json.Value.TryGetValue(name, out found) || found == null)
            {
                return null;
            }
            if (found is JsonElement element)
            {
                return element.ValueKind == JsonValueKind.Number ? element.GetRawText() : element.ToString();
            }
            return Convert.ToString(found, CultureInfo.InvariantCulture);
        }

        private static int Int(string name, int fallback)
        {
            int parsed;
            return int.TryParse(Value(name), NumberStyles.Integer, CultureInfo.InvariantCulture, out parsed) ? parsed : fallback;
        }

        public static string Token
        {
            get { return Value("Token") ?? ""; }
        }

        // Port cua bridge: Windows co hai ho (WPS: Port, Microsoft Office: PortOffice), LibreOffice: PortLibreOffice.
        // Tren Linux chi co LibreOffice nen moi ho deu tro ve do.
        public static int PortForKind(string kind, bool office)
        {
            int basePort;
            if (OperatingSystem.IsWindows() && !office)
            {
                basePort = Int("Port", 47821);
            }
            else if (OperatingSystem.IsWindows())
            {
                basePort = Int("PortOffice", 47831);
            }
            else
            {
                basePort = Int("PortLibreOffice", 47851);
            }
            switch (kind)
            {
                case "et":
                    return basePort + 1;
                case "wpp":
                    return basePort + 2;
                default:
                    return basePort;
            }
        }
    }

    // Nhat ky: cung file bridge.log voi extension/add-in (xem log cua MCP canh log cua bridge).
    internal static class Logger
    {
        private static readonly object Sync = new object();

        public static void Info(string message)
        {
            Write("INFO", message, null);
        }

        public static void Error(string message, Exception error = null)
        {
            Write("ERROR", message, error);
        }

        private static void Write(string level, string message, Exception error)
        {
            string line = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss.fff", CultureInfo.InvariantCulture) +
                          " [" + level + "] [mcp " + Environment.ProcessId + "] " + message +
                          (error == null ? "" : ": " + error.GetType().Name + ": " + error.Message);
            lock (Sync)
            {
                try
                {
                    Directory.CreateDirectory(DataPaths.Data);
                    File.AppendAllText(Path.Combine(DataPaths.Data, "bridge.log"), line + Environment.NewLine, new UTF8Encoding(false));
                }
                catch (Exception)
                {
                    // Khong ghi duoc log thi thoi: MCP khong duoc phep chet vi log.
                }
            }
        }
    }

    // Thu muc session cua bridge dang song (Agent Core va MCP doc cung cho nay).
    internal static class SessionRegistry
    {
        public static string Directory
        {
            get { return Path.Combine(DataPaths.Runtime, "sessions"); }
        }
    }

    internal static class CommandDispatcher
    {
        // Nhu add-in: phien ban cua chinh assembly (serverInfo.version trong initialize).
        public static readonly string BridgeVersion = typeof(CommandDispatcher).Assembly.GetName().Version.ToString(3);
    }

    internal sealed class CommandParam
    {
        public readonly string Name;
        public readonly string Hint;

        public CommandParam(string name, string hint)
        {
            Name = name;
            Hint = hint;
        }
    }

    internal sealed class CommandInfo
    {
        public readonly string Name;
        public readonly string Kind;
        public readonly CommandParam[] Params;

        public CommandInfo(string name, string kind, CommandParam[] parameters)
        {
            Name = name;
            Kind = kind;
            Params = parameters ?? new CommandParam[0];
        }

        // "et.readRange {range,sheet}" - cung dinh dang voi add-in (CommandCatalog.Signatures).
        public string Signature
        {
            get
            {
                return Name + " {" + string.Join(",", Params.Select(p => string.IsNullOrEmpty(p.Hint) ? p.Name : p.Name + " (" + p.Hint + ")").ToArray()) + "}";
            }
        }
    }

    // Danh sach lenh bridge: ban Windows lay tu CommandCatalog cua add-in, ban nay doc file nhung san
    // (live-commands.json, sinh tu registry cua extension LibreOffice bang scripts/generate_mcp_commands.py).
    internal static class CommandCatalog
    {
        private static readonly Lazy<List<CommandInfo>> Items = new Lazy<List<CommandInfo>>(Load);

        public static IEnumerable<CommandInfo> All
        {
            get { return Items.Value; }
        }

        public static string Signatures(IEnumerable<CommandInfo> commands)
        {
            return string.Join("; ", commands.Select(c => c.Signature).ToArray());
        }

        private static List<CommandInfo> Load()
        {
            var list = new List<CommandInfo>();
            try
            {
                using (Stream stream = typeof(CommandCatalog).Assembly.GetManifestResourceStream("AxiomOffice.Mcp.live-commands.json"))
                {
                    if (stream == null)
                    {
                        return list;
                    }
                    using (JsonDocument document = JsonDocument.Parse(stream))
                    {
                        foreach (JsonElement item in document.RootElement.EnumerateArray())
                        {
                            var parameters = new List<CommandParam>();
                            JsonElement paramsElement;
                            if (item.TryGetProperty("params", out paramsElement))
                            {
                                foreach (JsonElement parameter in paramsElement.EnumerateArray())
                                {
                                    JsonElement hint;
                                    parameters.Add(new CommandParam(parameter.GetProperty("name").GetString(),
                                        parameter.TryGetProperty("hint", out hint) && hint.ValueKind == JsonValueKind.String ? hint.GetString() : null));
                                }
                            }
                            JsonElement kind;
                            list.Add(new CommandInfo(item.GetProperty("name").GetString(),
                                item.TryGetProperty("kind", out kind) && kind.ValueKind == JsonValueKind.String ? kind.GetString() : null,
                                parameters.ToArray()));
                        }
                    }
                }
            }
            catch (Exception error)
            {
                Logger.Error("khong doc duoc live-commands.json", error);
            }
            return list;
        }
    }
}

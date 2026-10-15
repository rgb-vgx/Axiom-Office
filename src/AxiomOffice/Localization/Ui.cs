using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text;
using System.Web.Script.Serialization;
using AxiomOffice.Bridge;

namespace AxiomOffice
{
    // Ngôn ngữ giao diện (người dùng 16/10/2026: "UI có button chọn ngôn ngữ hiển thị cho các nút"). Chuỗi gốc là
    // tiếng Việt và chính nó là khoá dịch: Ui.T("Trò chuyện mới") -> "New chat". Bản dịch là JSON {"chuỗi gốc":
    // "bản dịch", "_name": "English"}: nhúng trong DLL (src/AxiomOffice/lang/*.json, build.ps1) và tệp lang\<mã>.json
    // cạnh DLL (thêm ngôn ngữ không cần build lại; tệp ngoài đè bản nhúng). Thiếu bản dịch -> giữ tiếng Việt.
    // Chỉ đổi chữ của add-in; câu trả lời của AI vẫn theo ngôn ngữ người dùng gõ.
    internal static class Ui
    {
        public const string SourceCode = "vi";
        private const string SourceName = "Tiếng Việt";
        private const string ResourcePrefix = "AxiomOffice.lang.";

        private static readonly object Sync = new object();
        private static string _code;
        private static Dictionary<string, string> _table;

        // Changed: người dùng đổi ngôn ngữ (ribbon, wizard) - pane và ribbon vẽ lại chữ.
        public static event EventHandler Changed;

        // Mã ngôn ngữ đang dùng ("vi", "en"...).
        public static string Code
        {
            get
            {
                Ensure();
                return _code;
            }
        }

        // Lựa chọn của người dùng: "auto" = theo ngôn ngữ Windows, "vi", "en"... Chưa chọn = tiếng Việt như trước khi
        // có tính năng này (máy Windows tiếng Anh của người dùng Việt không tự đổi sang tiếng Anh).
        public const string Auto = "auto";

        public static string Setting
        {
            get
            {
                string setting = (Config.UiLanguage ?? "").Trim().ToLowerInvariant();
                return setting.Length == 0 ? SourceCode : setting;
            }
        }

        public static string T(string text)
        {
            if (string.IsNullOrEmpty(text))
            {
                return text;
            }
            Ensure();
            Dictionary<string, string> table = _table;
            string translated;
            if (table != null && table.TryGetValue(text, out translated) && !string.IsNullOrEmpty(translated))
            {
                return translated;
            }
            return text;
        }

        // Các ngôn ngữ có sẵn: (mã, tên) - tiếng Việt trước, rồi theo tên.
        public static List<KeyValuePair<string, string>> Languages()
        {
            var result = new List<KeyValuePair<string, string>> { new KeyValuePair<string, string>(SourceCode, SourceName) };
            foreach (KeyValuePair<string, string> language in Sources()
                .Select(source => new KeyValuePair<string, string>(source.Key, NameOf(source.Key)))
                .OrderBy(language => language.Value, StringComparer.CurrentCultureIgnoreCase))
            {
                result.Add(language);
            }
            return result;
        }

        // Đổi ngôn ngữ ("auto" = theo Windows) rồi báo cho pane/ribbon.
        public static void Set(string setting)
        {
            setting = (setting ?? "").Trim().ToLowerInvariant();
            Config.WriteString("UiLanguage", setting);
            Load();
            Logger.Info("Ui: language setting=" + setting + " -> " + _code);
            EventHandler handler = Changed;
            if (handler != null)
            {
                try
                {
                    handler(null, EventArgs.Empty);
                }
                catch (Exception ex)
                {
                    Logger.Error("Ui.Changed handler failed", ex);
                }
            }
        }

        private static void Ensure()
        {
            if (_code == null)
            {
                Load();
            }
        }

        private static void Load()
        {
            lock (Sync)
            {
                string code = Resolve(Setting);
                Dictionary<string, string> table = code == SourceCode ? null : ReadTable(code);
                if (code != SourceCode && table == null)
                {
                    code = SourceCode;
                }
                _table = table;
                _code = code;
            }
        }

        // "auto": ngôn ngữ Windows nếu có bản dịch; Windows tiếng Việt -> tiếng Việt; ngôn ngữ khác chưa có bản dịch ->
        // tiếng Anh (người không đọc được tiếng Việt dễ dùng hơn).
        private static string Resolve(string setting)
        {
            Dictionary<string, Func<string>> sources = Sources();
            if (setting != Auto)
            {
                return setting == SourceCode || sources.ContainsKey(setting) ? setting : SourceCode;
            }
            string windows = CultureInfo.CurrentUICulture.TwoLetterISOLanguageName.ToLowerInvariant();
            if (windows == SourceCode || sources.ContainsKey(windows))
            {
                return windows;
            }
            return sources.ContainsKey("en") ? "en" : SourceCode;
        }

        private static string NameOf(string code)
        {
            Dictionary<string, string> table = ReadTable(code);
            string name;
            if (table != null && table.TryGetValue("_name", out name) && !string.IsNullOrEmpty(name))
            {
                return name;
            }
            return code;
        }

        private static Dictionary<string, string> ReadTable(string code)
        {
            Func<string> source;
            if (!Sources().TryGetValue(code, out source))
            {
                return null;
            }
            try
            {
                var raw = new JavaScriptSerializer { MaxJsonLength = int.MaxValue }.Deserialize<Dictionary<string, object>>(source());
                var table = new Dictionary<string, string>(StringComparer.Ordinal);
                foreach (KeyValuePair<string, object> entry in raw)
                {
                    table[entry.Key] = Convert.ToString(entry.Value, CultureInfo.InvariantCulture);
                }
                return table;
            }
            catch (Exception ex)
            {
                Logger.Error("Ui: cannot read language '" + code + "'", ex);
                return null;
            }
        }

        // Nguồn bản dịch theo mã: tài nguyên nhúng AxiomOffice.lang.<mã>.json, rồi tệp lang\<mã>.json cạnh DLL (đè).
        private static Dictionary<string, Func<string>> Sources()
        {
            var sources = new Dictionary<string, Func<string>>(StringComparer.OrdinalIgnoreCase);
            Assembly assembly = typeof(Ui).Assembly;
            foreach (string name in assembly.GetManifestResourceNames())
            {
                if (name.StartsWith(ResourcePrefix, StringComparison.Ordinal) && name.EndsWith(".json", StringComparison.OrdinalIgnoreCase))
                {
                    string resource = name;
                    string code = name.Substring(ResourcePrefix.Length, name.Length - ResourcePrefix.Length - 5).ToLowerInvariant();
                    sources[code] = () =>
                    {
                        using (Stream stream = assembly.GetManifestResourceStream(resource))
                        using (var reader = new StreamReader(stream, Encoding.UTF8))
                        {
                            return reader.ReadToEnd();
                        }
                    };
                }
            }
            try
            {
                string folder = Path.Combine(Path.GetDirectoryName(assembly.Location) ?? "", "lang");
                if (Directory.Exists(folder))
                {
                    foreach (string path in Directory.GetFiles(folder, "*.json"))
                    {
                        string file = path;
                        string code = Path.GetFileNameWithoutExtension(path).ToLowerInvariant();
                        if (code != SourceCode)
                        {
                            sources[code] = () => File.ReadAllText(file, Encoding.UTF8);
                        }
                    }
                }
            }
            catch (Exception ex)
            {
                Logger.Error("Ui: cannot list lang folder", ex);
            }
            return sources;
        }
    }
}

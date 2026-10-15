using System;
using System.Collections.Generic;
using AxiomOffice.Bridge;

namespace AxiomOffice.Ai
{
    // Nguồn tìm kiếm web cho menu "Tìm web" trên ribbon (16/10/2026): Tavily (tích hợp trong Core)
    // và các MCP server được đánh dấu "role": "web-search" trong mcp.json — coi Tavily như một nguồn
    // ngang hàng với server ngoài. Chọn một nguồn = CHỈ nguồn đó chạy: ghi TavilyEnabled (HKCU) và
    // cờ "disabled" của từng server trong mcp.json; Core đọc lại ở lượt AI kế tiếp.
    internal static class SearchSources
    {
        // Giá trị "role" trong mcp.json đánh dấu server là nguồn tìm kiếm web (hiện trong menu).
        public const string WebSearchRole = "web-search";

        internal sealed class Item
        {
            public string Id;      // "off" | "tavily" | "mcp:<tên server>"
            public string Label;   // chữ hiện trong menu (đã dịch)
        }

        // Danh sách mục: Tắt, Tavily, rồi từng server đánh dấu role (theo tên).
        public static List<Item> Items()
        {
            var items = new List<Item>
            {
                new Item { Id = "off", Label = Ui.T("Tắt (không tìm web)") },
                new Item { Id = "tavily", Label = Ui.T("Tavily (tích hợp)") },
            };
            string error;
            Dictionary<string, Dictionary<string, object>> servers = McpFile.Load(out error);
            if (servers != null)
            {
                var names = new List<string>();
                foreach (KeyValuePair<string, Dictionary<string, object>> pair in servers)
                {
                    if (pair.Key != "office" && McpFile.Str(pair.Value, "role") == WebSearchRole)
                    {
                        names.Add(pair.Key);
                    }
                }
                names.Sort(StringComparer.Ordinal);
                foreach (string name in names)
                {
                    items.Add(new Item { Id = "mcp:" + name, Label = name });
                }
            }
            return items;
        }

        // Mục đang chọn theo cấu hình thật: Tavily nếu cờ bật; nếu không, server tìm kiếm đang bật
        // đầu tiên; còn lại là "Tắt".
        public static int SelectedIndex()
        {
            List<Item> items = Items();
            if (Config.TavilyEnabled)
            {
                return IndexOf(items, "tavily");
            }
            string error;
            Dictionary<string, Dictionary<string, object>> servers = McpFile.Load(out error);
            if (servers != null)
            {
                for (int i = 0; i < items.Count; i++)
                {
                    if (!items[i].Id.StartsWith("mcp:", StringComparison.Ordinal))
                    {
                        continue;
                    }
                    Dictionary<string, object> entry;
                    if (servers.TryGetValue(items[i].Id.Substring(4), out entry) && !McpFile.Flag(entry, "disabled"))
                    {
                        return i;
                    }
                }
            }
            return 0;
        }

        // Chọn nguồn: chỉ nguồn này chạy (nguồn khác bị tắt). needsKey: chọn Tavily mà chưa có khoá.
        public static bool Activate(int index, out bool needsKey, out string error)
        {
            needsKey = false;
            error = null;
            List<Item> items = Items();
            if (index < 0 || index >= items.Count)
            {
                error = "invalid index";
                return false;
            }
            string id = items[index].Id;

            Dictionary<string, Dictionary<string, object>> servers = McpFile.Load(out error);
            if (servers == null)
            {
                return false;
            }

            bool tavily = id == "tavily";
            if (!Config.WriteDword("TavilyEnabled", tavily ? 1 : 0))
            {
                error = "cannot write HKCU\\Software\\AxiomOffice";
                return false;
            }
            if (tavily)
            {
                needsKey = Config.TavilyApiKey.Length == 0;
            }

            foreach (KeyValuePair<string, Dictionary<string, object>> pair in servers)
            {
                if (pair.Key == "office" || McpFile.Str(pair.Value, "role") != WebSearchRole)
                {
                    continue;
                }
                if (id == "mcp:" + pair.Key)
                {
                    pair.Value.Remove("disabled");
                }
                else
                {
                    pair.Value["disabled"] = true;
                }
            }
            if (!McpFile.Save(servers, out error))
            {
                return false;
            }
            Logger.Info("Ribbon: nguồn tìm web = " + id);
            return true;
        }

        private static int IndexOf(List<Item> items, string id)
        {
            for (int i = 0; i < items.Count; i++)
            {
                if (items[i].Id == id)
                {
                    return i;
                }
            }
            return 0;
        }
    }
}

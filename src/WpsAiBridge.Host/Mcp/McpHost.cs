using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using WpsAiBridge.Bridge;

namespace WpsAiBridge.Host.Mcp
{
    // WpsAiBridge.Host.exe mcp [all|word|excel|ppt] [--list]
    // Thay 3 server Python (tools/word-mcp, excel-mcp, ppt-mcp) bằng một exe không cần cài gì thêm.
    internal static class McpHost
    {
        public static int Run(string[] args)
        {
            string group = args.Skip(1).FirstOrDefault(a => !a.StartsWith("--", StringComparison.Ordinal)) ?? "all";
            group = group.ToLowerInvariant();
            List<McpTool> tools = Catalog(group);
            if (tools == null)
            {
                Console.Error.WriteLine("usage: WpsAiBridge.Host.exe mcp [all|word|excel|ppt] [--list]");
                return 2;
            }
            string name = group == "all" ? "office-tools" : group + "-tools";
            var server = new McpServer(name, tools);

            if (args.Contains("--list"))
            {
                foreach (McpTool tool in server.Tools)
                {
                    Console.WriteLine(tool.Name);
                }
                Console.WriteLine(server.ToolCount + " tools");
                return 0;
            }

            // stdout chỉ dành cho JSON-RPC: mọi Console.Write khác đi sang stderr.
            var output = new StreamWriter(Console.OpenStandardOutput(), new UTF8Encoding(false)) { AutoFlush = false, NewLine = "\n" };
            var input = new StreamReader(Console.OpenStandardInput(), new UTF8Encoding(false));
            Console.SetOut(Console.Error);
            try
            {
                return server.Run(input, output);
            }
            catch (Exception ex)
            {
                Logger.Error("MCP server crashed", ex);
                return 1;
            }
        }

        public static List<McpTool> Catalog(string group)
        {
            var tools = new List<McpTool>();
            switch (group)
            {
                case "word":
                    tools.AddRange(WordFiles.Tools());
                    tools.AddRange(LiveTools.Word());
                    break;
                case "excel":
                    tools.AddRange(ExcelFiles.Tools());
                    tools.AddRange(LiveTools.Excel());
                    break;
                case "ppt":
                case "powerpoint":
                    tools.AddRange(PptFiles.Tools());
                    tools.AddRange(LiveTools.Ppt());
                    break;
                case "all":
                    tools.AddRange(WordFiles.Tools());
                    tools.AddRange(LiveTools.Word());
                    tools.AddRange(ExcelFiles.Tools());
                    tools.AddRange(LiveTools.Excel());
                    tools.AddRange(PptFiles.Tools());
                    tools.AddRange(LiveTools.Ppt());
                    break;
                default:
                    return null;
            }
            tools.Add(LiveTools.Sessions());
            return tools;
        }
    }

    // Template docx/pptx nhúng trong exe (lấy từ python-docx / python-pptx, giấy phép MIT).
    internal static class Templates
    {
        public static byte[] Docx
        {
            get { return Load("WpsAiBridge.Mcp.default.docx"); }
        }

        public static byte[] Pptx
        {
            get { return Load("WpsAiBridge.Mcp.default.pptx"); }
        }

        private static byte[] Load(string name)
        {
            using (Stream stream = typeof(Templates).Assembly.GetManifestResourceStream(name))
            {
                if (stream == null)
                {
                    throw new InvalidOperationException("embedded template missing: " + name + " (rebuild with scripts\\build.ps1)");
                }
                using (var memory = new MemoryStream())
                {
                    stream.CopyTo(memory);
                    return memory.ToArray();
                }
            }
        }
    }
}

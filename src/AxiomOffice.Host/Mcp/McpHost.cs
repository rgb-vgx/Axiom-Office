using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Text;
using System.Threading;
using AxiomOffice.Bridge;

namespace AxiomOffice.Host.Mcp
{
    // AxiomOffice.Host.exe mcp [all|word|excel|ppt] [--list]
    // Thay 3 server Python (tools/word-mcp, excel-mcp, ppt-mcp) bằng một exe không cần cài gì thêm.
    internal static class McpHost
    {
        public static int Run(string[] args)
        {
            // SHIM: có Agent Core (bản Go) cạnh đây thì chuyển tiếp sang nó, để chỉ còn MỘT bản MCP
            // (core-go/internal/mcpserver). Người dùng đã cấu hình MCP client trỏ vào Host.exe không phải
            // đổi gì, và hai bản không còn dịp lệch nhau.
            //
            // Gói cài có thể KHÔNG kèm Core (lúc build máy không có Go - scripts/build.ps1 bỏ qua), nên
            // thiếu Core thì chạy bản C# như cũ: không để ai bị kẹt.
            // `--in-process` để ép chạy bản C# khi cần so sánh/debug.
            string core = CoreExecutable();
            bool inProcess = args.Contains("--in-process", StringComparer.Ordinal);
            if (core != null && !inProcess)
            {
                return Forward(core, args);
            }

            string group = args.Skip(1).FirstOrDefault(a => !a.StartsWith("--", StringComparison.Ordinal)) ?? "all";
            group = group.ToLowerInvariant();
            List<McpTool> tools = Catalog(group);
            if (tools == null)
            {
                Console.Error.WriteLine("usage: AxiomOffice.Host.exe mcp [all|word|excel|ppt] [--list]");
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

        // CoreExecutable: AxiomOffice.Core.exe nằm cạnh Host.exe, hoặc null nếu gói cài không kèm.
        private static string CoreExecutable()
        {
            try
            {
                string here = Path.GetDirectoryName(typeof(McpHost).Assembly.Location);
                if (string.IsNullOrEmpty(here))
                {
                    return null;
                }
                string exe = Path.Combine(here, "AxiomOffice.Core.exe");
                return File.Exists(exe) ? exe : null;
            }
            catch (Exception)
            {
                return null;
            }
        }

        // Forward: nối thẳng stdin/stdout/stderr sang tiến trình con, theo BYTE, không giải mã rồi mã lại.
        // Giao thức MCP ở đây là JSON theo dòng trên stdout; một lần mã hoá sai là hỏng cả phiên, mà
        // chuyển byte thì không có gì để sai.

        private static int Forward(string core, string[] args)
        {
            var start = new ProcessStartInfo(core)
            {
                UseShellExecute = false,
                RedirectStandardInput = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                CreateNoWindow = true,
                WorkingDirectory = Path.GetDirectoryName(core),
            };
            // Gui LAI CA args, ke ca args[0]="mcp": Core nhan dien subcommand bang chinh os.Args[1], nen bo
            // chu "mcp" di thi no chay nhu mot Core binh thuong - bind port roi ngoi mai, khong tra loi
            // MCP. (Da mac dung loi nay: shim treo, khong in gi, va bo lai mot tien trinh Core mo côi.)
            foreach (string arg in args)
            {
                if (arg == "--in-process")
                {
                    continue;   // cờ của shim, không phải của Core
                }
                start.Arguments += (start.Arguments.Length > 0 ? " " : "") + Quote(arg);
            }

            using (Process child = Process.Start(start))
            {
                Thread toChild = Pump(Console.OpenStandardInput(), child.StandardInput.BaseStream);
                Thread fromChild = Pump(child.StandardOutput.BaseStream, Console.OpenStandardOutput());
                Thread errors = Pump(child.StandardError.BaseStream, Console.OpenStandardError());
                child.WaitForExit();

                // Cho stdout/stderr của con chảy hết trước khi trả mã thoát, không thì khung cuối bị cụt.
                fromChild.Join(5000);
                errors.Join(5000);
                try
                {
                    child.StandardInput.Close();
                }
                catch (Exception)
                {
                }
                toChild.Join(1000);
                return child.ExitCode;
            }
        }

        private static Thread Pump(Stream from, Stream to)
        {
            var thread = new Thread(delegate()
            {
                try
                {
                    var buffer = new byte[8192];
                    int read;
                    while ((read = from.Read(buffer, 0, buffer.Length)) > 0)
                    {
                        to.Write(buffer, 0, read);
                        to.Flush();
                    }
                }
                catch (Exception)
                {
                    // đầu kia đóng: hết việc
                }
            });
            thread.IsBackground = true;
            thread.Start();
            return thread;
        }

        private static string Quote(string value)
        {
            if (value.Length > 0 && value.IndexOfAny(new[] { ' ', '\t', '"' }) < 0)
            {
                return value;
            }
            return "\"" + value.Replace("\"", "\\\"") + "\"";
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
            get { return Load("AxiomOffice.Mcp.default.docx"); }
        }

        public static byte[] Pptx
        {
            get { return Load("AxiomOffice.Mcp.default.pptx"); }
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

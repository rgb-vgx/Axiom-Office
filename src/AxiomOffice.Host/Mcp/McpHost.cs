using System;
using System.Diagnostics;
using System.IO;
using System.Threading;

namespace AxiomOffice.Host.Mcp
{
    // AxiomOffice.Host.exe mcp [all|word|excel|ppt] [--list]
    //
    // CHUYỂN TIẾP sang Agent Core (bản Go). MCP server chỉ còn MỘT bản cài đặt là
    // core-go/internal/mcpserver, dùng chung Windows lẫn Linux.
    //
    // Tệp này từng có bản C# đầy đủ làm dự phòng (10 tệp nữa, ~5.570 dòng: McpServer, WordFiles,
    // ExcelFiles, PptFiles, LiveTools, Cells, OoxmlPackage, XlsxBook, XlsxStyles, BridgeClient +
    // template docx/pptx nhúng). Đã bỏ, vì: gói cài không kèm Core thì cũng không có backend nào khác
    // cho add-in, mà bản dự phòng đó lại KHÔNG được CI kiểm ở đâu cả — hai bản cùng một hợp đồng 50
    // tool mà chỉ một bản có người canh là cách chắc chắn nhất để chúng lệch nhau.
    //
    // Người đã cấu hình MCP client trỏ vào Host.exe không phải đổi gì.
    internal static class McpHost
    {
        public static int Run(string[] args)
        {
            string core = CoreExecutable();
            if (core == null)
            {
                Console.Error.WriteLine(
                    "AxiomOffice.Core.exe không nằm cạnh AxiomOffice.Host.exe nên không có MCP server để chạy.\n"
                    + "MCP server là lệnh con `mcp` của Agent Core (Go). Hãy cài lại gói đầy đủ, hoặc trỏ MCP\n"
                    + "client thẳng vào: AxiomOffice.Core.exe mcp all");
                return 2;
            }
            return Forward(core, args);
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
    }
}

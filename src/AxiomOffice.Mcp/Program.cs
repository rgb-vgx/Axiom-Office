using System;
using System.Linq;
using System.Text;
using AxiomOffice.Host.Mcp;

namespace AxiomOffice.Mcp
{
    // axiom-office-mcp [all|word|excel|ppt] [--list]
    //
    // MCP server qua stdio: 20 tool file (docx/xlsx/pptx/csv) + tool live gọi bridge của LibreOffice
    // (hoặc của add-in trên Windows). Tương đương `AxiomOffice.Host.exe mcp` nhưng chạy trên Linux.
    internal static class Program
    {
        private static int Main(string[] args)
        {
            // stdout chi danh cho JSON-RPC (McpHost.Run chuyen Console.Write sang stderr).
            Console.OutputEncoding = new UTF8Encoding(false);
            string[] forwarded = new[] { "mcp" }.Concat(args ?? new string[0]).ToArray();
            return McpHost.Run(forwarded);
        }
    }
}

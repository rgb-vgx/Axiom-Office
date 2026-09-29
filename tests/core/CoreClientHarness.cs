// Test that duong pane -> Agent Core: goi CoreClient.Run (code C# cua add-in) voi Excel that, Core that, LLM that.
// Can: da build (scripts\build.ps1), Core dang chay (core.json), khong co Excel nao dang mo, co cau hinh LLM.
// Bien dich chung voi nguon add-in (CoreClient la internal) - xem lenh csc trong scripts\build.ps1, them file nay,
// /target:exe - roi chay exe. In "PASS CoreClient.Run qua Core that" khi thanh cong.
using System;
using System.Diagnostics;
using System.IO;
using System.Net;
using System.Text;
using System.Threading;
using AxiomOffice.Ai;
using AxiomOffice.Bridge;
using Microsoft.Win32;

static class CoreClientHarness
{
    const int Port = 47832;

    static int Main()
    {
        string exe = (string)Registry.GetValue(@"HKEY_LOCAL_MACHINE\Software\Microsoft\Windows\CurrentVersion\App Paths\excel.exe", "", null);
        Process excel = Process.Start(exe, "/x");
        string ownSession = null;
        try
        {
            string excelSession = Path.Combine(SessionRegistry.Directory, excel.Id + ".json");
            var watch = Stopwatch.StartNew();
            while (!File.Exists(excelSession) && watch.ElapsedMilliseconds < 60000)
            {
                Thread.Sleep(300);
            }
            if (!File.Exists(excelSession))
            {
                Console.WriteLine("FAIL khong thay session cua Excel pid " + excel.Id);
                return 1;
            }
            Console.WriteLine("Excel pid " + excel.Id + " san sang");
            Console.WriteLine("newWorkbook: " + Cmd("{\"action\":\"et.newWorkbook\"}"));

            // CoreClient tim port bridge theo session file cua CHINH tien trinh (trong add-in la Excel).
            int self = Process.GetCurrentProcess().Id;
            ownSession = Path.Combine(SessionRegistry.Directory, self + ".json");
            File.WriteAllText(ownSession, File.ReadAllText(excelSession));

            int events = 0;
            LlmResult result = CoreClient.Instance.Run(null, "Ghi chữ HARNESS_OK vào ô A1 của sheet đang mở. Không làm gì khác.", null,
                delegate(CoreEvent e) { events++; Console.WriteLine("  event " + e.Type + " " + (e.Action ?? "")); },
                CancellationToken.None);

            if (result == null)
            {
                Console.WriteLine("FAIL Run tra null (fallback): " + CoreClient.Instance.LastError);
                return 1;
            }
            string a1 = Cmd("{\"action\":\"et.readRange\",\"params\":{\"range\":\"A1\"}}");
            Console.WriteLine("ok=" + result.Ok + " viaCore=" + result.ViaCore + " conv=" + result.ConversationId
                + " events=" + events + " rounds=" + result.Rounds + " seconds=" + result.Seconds.ToString("0.0")
                + " err=" + result.Error + " text=" + result.Text);
            Console.WriteLine("A1: " + a1);
            bool pass = result.Ok && result.ViaCore && !string.IsNullOrEmpty(result.ConversationId) && events > 0 && a1.Contains("HARNESS_OK");
            Console.WriteLine(pass ? "PASS CoreClient.Run qua Core that" : "FAIL");
            return pass ? 0 : 1;
        }
        finally
        {
            if (ownSession != null)
            {
                File.Delete(ownSession);
            }
            try
            {
                if (!excel.HasExited)
                {
                    Cmd("{\"action\":\"et.closeAll\",\"params\":{\"save\":false}}");
                    excel.CloseMainWindow();
                    if (!excel.WaitForExit(15000))
                    {
                        excel.Kill();
                    }
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine("dong Excel: " + ex.Message);
                try { excel.Kill(); } catch (Exception) { }
            }
        }
    }

    static string Cmd(string json)
    {
        try
        {
            using (var client = new WebClient())
            {
                client.Encoding = Encoding.UTF8;
                client.Headers["X-Auth-Token"] = Config.Token;
                client.Headers["Content-Type"] = "application/json";
                return client.UploadString("http://127.0.0.1:" + Port + "/cmd", json);
            }
        }
        catch (WebException ex)
        {
            return "ERR " + ex.Message;
        }
    }
}

// Test thị giác cho wizard thiết lập của add-in (net48, WinForms): mở form thật, nhảy qua 5 bước rồi lưu
// ảnh từng bước ra PNG. Không cần Office/WPS vì form chỉ dùng host cho nút "Thử ngay" (truyền null).
//
//   powershell -ExecutionPolicy Bypass -File tests\ui\shots.ps1 [-OutDir <thư mục>]
//
// Mục đích: sau mỗi lần sửa bố cục/màu/chữ của wizard, render lại để so trước-sau (ảnh nằm trong
// tests/ui/out/, không commit). Biên dịch chung với mã nguồn add-in nên luôn khớp bản đang có.
using System;
using System.Drawing;
using System.Drawing.Imaging;
using System.IO;
using System.Reflection;
using System.Threading;
using System.Windows.Forms;
using AxiomOffice.Ai;

internal static class SetupWizardShots
{
    [STAThread]
    private static void Main(string[] args)
    {
        string outDir = args.Length > 0 ? args[0] : ".";
        Directory.CreateDirectory(outDir);

        Application.EnableVisualStyles();
        Application.SetCompatibleTextRenderingDefault(false);

        var form = new SetupWizardForm(null);
        form.StartPosition = FormStartPosition.Manual;
        form.Location = new Point(0, 0);
        form.Show();
        Pump();

        MethodInfo showStep = typeof(SetupWizardForm).GetMethod("ShowStep", BindingFlags.NonPublic | BindingFlags.Instance);
        MethodInfo setProbe = typeof(SetupWizardForm).GetMethod("SetProbe", BindingFlags.NonPublic | BindingFlags.Instance);
        string[] names = { "1-welcome", "2-checks", "3-connect", "4-features", "5-done" };
        for (int step = 0; step < names.Length; step++)
        {
            showStep.Invoke(form, new object[] { step });
            Pump();
            Grab(form, Path.Combine(outDir, names[step] + ".png"));
        }

        // Bước "Kết nối" với dữ liệu đã điền + kết quả lỗi: đây là trạng thái người dùng gặp nhiều nhất khi
        // địa chỉ hoặc khoá sai, nên phải render riêng để soi phần thông báo lỗi.
        showStep.Invoke(form, new object[] { 2 });
        Pump();
        SetText(form, "_endpoint", "http://may-chu-cong-ty/v1");
        SetText(form, "_model", "gpt-4o-mini");
        setProbe.Invoke(form, new object[]
        {
            "Máy chủ AI từ chối khoá truy cập (401). Có thể khoá sai, hết hạn, hoặc dán thiếu ký tự.", "danger",
        });
        Pump();
        Grab(form, Path.Combine(outDir, "3-connect-filled.png"));

        form.Close();
    }

    private static void SetText(object form, string field, string value)
    {
        FieldInfo info = form.GetType().GetField(field, BindingFlags.NonPublic | BindingFlags.Instance);
        object control = info.GetValue(form);
        control.GetType().GetProperty("Text").SetValue(control, value, null);
    }

    private static void Pump()
    {
        for (int i = 0; i < 12; i++)
        {
            Application.DoEvents();
            Thread.Sleep(40);
        }
    }

    private static void Grab(Form form, string path)
    {
        form.Refresh();
        Pump();
        using (var bitmap = new Bitmap(form.Width, form.Height))
        {
            form.DrawToBitmap(bitmap, new Rectangle(0, 0, form.Width, form.Height));
            bitmap.Save(path, ImageFormat.Png);
        }

        Console.WriteLine("  " + path + "  (" + form.Width + "x" + form.Height + ")");
    }
}

# Dùng Microsoft Office thật (COM) để:
#   -Make  : tạo real_word.docx / real_excel.xlsx / real_ppt.pptx (bảng gộp ô, hyperlink, shared formula, ghi chú...)
#            làm mẫu cho test_mcp_host.py so parity.
#   -Verify: mở các file do MCP C# ghi ra (cs_*.docx/xlsx/pptx, py_book.xlsx đã sửa) bằng Office và đọc lại;
#            Office mở được qua automation (không cần repair) là file hợp lệ.
# Cách dùng: powershell -ExecutionPolicy Bypass -File tests\mcp-host\office_roundtrip.ps1 -Dir <thư_mục_output> -Make|-Verify
param([Parameter(Mandatory = $true)][string]$Dir, [switch]$Make, [switch]$Verify)

$ErrorActionPreference = "Stop"
$Dir = (Resolve-Path $Dir).Path

# Máy có WPS có thể đăng ký đè ProgID Word/Excel/PowerPoint.Application (New-Object ra WPS), còn Office
# chỉ vào ROT sau khi mất focus (KB238610). Nên: chạy exe của Office rồi lấy object model từ cửa sổ
# tài liệu qua AccessibleObjectFromWindow(OBJID_NATIVEOM).
if (-not ("OfficeAttach" -as [type])) {
Add-Type @"
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
public static class OfficeAttach {
    delegate bool EnumProc(IntPtr h, IntPtr l);
    [DllImport("user32.dll")] static extern bool EnumWindows(EnumProc cb, IntPtr l);
    [DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr p, EnumProc cb, IntPtr l);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetClassName(IntPtr h, StringBuilder s, int n);
    [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
    [DllImport("oleacc.dll")] static extern int AccessibleObjectFromWindow(IntPtr h, uint id, ref Guid iid, [MarshalAs(UnmanagedType.IDispatch)] out object o);
    static string Cls(IntPtr h) { var sb = new StringBuilder(256); GetClassName(h, sb, 256); return sb.ToString(); }
    public static object Window(int pid, string childClass) {
        var tops = new List<IntPtr>();
        EnumWindows((h, l) => { uint p; GetWindowThreadProcessId(h, out p); if (p == pid) tops.Add(h); return true; }, IntPtr.Zero);
        foreach (IntPtr top in tops) {
            IntPtr found = IntPtr.Zero;
            EnumChildWindows(top, (h, l) => { if (Cls(h) == childClass) { found = h; return false; } return true; }, IntPtr.Zero);
            if (found == IntPtr.Zero) continue;
            Guid iid = new Guid("00020400-0000-0000-C000-000000000046");
            object o;
            if (AccessibleObjectFromWindow(found, 0xFFFFFFF0, ref iid, out o) == 0 && o != null) return o;
        }
        return null;
    }
}
"@
}

function Release($obj) { if ($obj) { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($obj) } }

# Lưu qua lệnh *.saveAs của bridge (chạy trong app): gọi SaveAs2 late-bound từ PowerShell có thể treo.
function Save-ViaBridge([int]$port, [string]$action, [string]$path) {
    $headers = @{ "X-Auth-Token" = (Get-ItemProperty "HKCU:\Software\WpsAiBridge").Token }
    $body = @{ action = $action; params = @{ path = $path } } | ConvertTo-Json -Compress
    $reply = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:$port/cmd" -Headers $headers -ContentType "application/json" -Body $body -TimeoutSec 60
    if (-not $reply.ok) { throw "$action failed: $($reply.error)" }
}

$script:OfficeProcess = $null
function Stop-Office { if ($script:OfficeProcess) { Stop-Process -Id $script:OfficeProcess.Id -Force -ErrorAction SilentlyContinue; $script:OfficeProcess = $null } }

function Get-Office([string]$exe, [string[]]$arguments, [string]$childClass) {
    $process = Start-Process $exe -ArgumentList $arguments -PassThru
    $script:OfficeProcess = $process
    $deadline = (Get-Date).AddSeconds(60)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 800
        $window = [OfficeAttach]::Window($process.Id, $childClass)
        if ($window) {
            $app = $window.Application
            Write-Host ("{0}: {1} {2}" -f $exe, $app.Name, $app.Version)
            return $app
        }
    }
    Stop-Office
    throw "cannot attach to $exe"
}

if ($Make -and -not (Test-Path (Join-Path $Dir "real_word.docx"))) {
    $word = Get-Office "winword.exe" @("/q", "/w") "_WwG"
    try {
        $word.DisplayAlerts = 0
        $doc = $word.ActiveDocument
        $sel = $word.Selection
        $sel.Style = $doc.Styles.Item(-2)            # Heading 1
        $sel.TypeText("Báo cáo thật từ Word"); $sel.TypeParagraph()
        $sel.Style = $doc.Styles.Item(-1)            # Normal
        $sel.TypeText("Đoạn thường có "); $sel.Font.Bold = 1; $sel.TypeText("chữ đậm"); $sel.Font.Bold = 0; $sel.TypeText(" và liên kết ")
        [void]$doc.Hyperlinks.Add($sel.Range, "https://example.com", [Type]::Missing, [Type]::Missing, "example.com")
        $sel.TypeParagraph()
        $table = $doc.Tables.Add($sel.Range, 3, 3)
        $table.Cell(1, 1).Range.Text = "Gộp ngang"; $table.Cell(1, 1).Merge($table.Cell(1, 2))
        $table.Cell(2, 1).Range.Text = "An"; $table.Cell(2, 2).Range.Text = "9.5"
        $table.Cell(2, 3).Range.Text = "Gộp dọc"; $table.Cell(2, 3).Merge($table.Cell(3, 3))
        [void]$sel.EndKey(6)
        $sel.TypeParagraph(); $sel.TypeText("Mục 1"); $sel.Range.ListFormat.ApplyBulletDefault(); $sel.TypeParagraph(); $sel.TypeText("Mục 2")
        Save-ViaBridge 47831 "writer.saveAs" (Join-Path $Dir "real_word.docx")
    } finally { Release $word; Stop-Office }
}

if ($Make -and -not (Test-Path (Join-Path $Dir "real_excel.xlsx"))) {
    # Excel mở màn hình Start (chưa có cửa sổ EXCEL7) nếu không có workbook: mở sẵn một workbook trống.
    $seed = Join-Path $Dir "seed.xlsx"
    $exe = Join-Path (Split-Path -Parent (Split-Path -Parent $PSScriptRoot)) "src\WpsAiBridge\bin\Release\WpsAiBridge.Host.exe"
    $request = @{ jsonrpc = "2.0"; id = 1; method = "tools/call"; params = @{ name = "excel_create"; arguments = @{ path = $seed; sheets = @(@{ name = "Sheet1"; values = @() }) } } } | ConvertTo-Json -Compress -Depth 8
    $request | & $exe mcp excel | Out-Null
    $excel = Get-Office "excel.exe" @("/x", "`"$seed`"") "EXCEL7"
    try {
        $excel.DisplayAlerts = $false
        $wb = $excel.ActiveWorkbook
        $ws = $wb.Worksheets.Item(1); $ws.Name = "Bảng"
        $ws.Range("A1").Value2 = "STT"; $ws.Range("B1").Value2 = "Giá trị"; $ws.Range("C1").Value2 = "Gấp đôi"; $ws.Range("D1").Value2 = "Ngày"
        for ($i = 2; $i -le 8; $i++) { $ws.Cells.Item($i, 1).Value2 = $i - 1; $ws.Cells.Item($i, 2).Formula = [string](($i - 1) * 1.5) }   # gán Double qua IDispatch lỗi cast; Formula nhận chuỗi số
        $ws.Range("C2").Formula = "=B2*2"; [void]$ws.Range("C2:C8").FillDown()   # Excel lưu thành shared formula
        $ws.Range("D2").Value2 = 45306; $ws.Range("D2").NumberFormat = "dd/mm/yyyy"
        $ws.Range("A10").Value2 = "Tổng"; $ws.Range("B10").Formula = "=SUM(B2:B8)"
        $extra = $wb.Worksheets.Add(); $extra.Name = "Phụ"
        $ws.Activate()
        Save-ViaBridge 47832 "et.saveAs" (Join-Path $Dir "real_excel.xlsx")
    } finally { Release $excel; Stop-Office }
}

if ($Make -and -not (Test-Path (Join-Path $Dir "real_ppt.pptx"))) {
    $ppt = Get-Office "powerpnt.exe" @("/n") "mdiClass"
    try {
        $pres = $ppt.ActivePresentation
        $s1 = $pres.Slides.Add(1, 1); $s1.Shapes.Item(1).TextFrame.TextRange.Text = "Giới thiệu"; $s1.Shapes.Item(2).TextFrame.TextRange.Text = "Phụ đề thật"
        $s2 = $pres.Slides.Add(2, 2); $s2.Shapes.Item(1).TextFrame.TextRange.Text = "Nội dung"; $s2.Shapes.Item(2).TextFrame.TextRange.Text = "Ý một`rÝ hai"
        $s2.NotesPage.Shapes.Placeholders.Item(2).TextFrame.TextRange.Text = "Ghi chú của slide 2"
        Save-ViaBridge 47833 "wpp.saveAs" (Join-Path $Dir "real_ppt.pptx")
    } finally { Release $ppt; Stop-Office }
}
if ($Make) { "samples: " + ((Get-ChildItem $Dir -Filter "real_*" | ForEach-Object Name) -join ", ") }

if ($Verify) {
    $word = Get-Office "winword.exe" @("/q", "/w") "_WwG"
    try {
        $word.DisplayAlerts = 0
        foreach ($name in "cs_word.docx", "real_word.docx") {
            $doc = $word.Documents.Open((Join-Path $Dir $name), $false, $true)
            "Word  {0}: paragraphs={1} tables={2} first='{3}'" -f $name, $doc.Paragraphs.Count, $doc.Tables.Count, $doc.Paragraphs.Item(1).Range.Text.Trim()
            $doc.Close(0)
        }
    } finally { Release $word; Stop-Office }

    $excel = Get-Office "excel.exe" @("/x", "`"$(Join-Path $Dir 'seed.xlsx')`"") "EXCEL7"
    try {
        $excel.DisplayAlerts = $false
        foreach ($name in "cs_book.xlsx", "py_book.xlsx", "real_excel.xlsx") {
            $path = Join-Path $Dir $name
            if (-not (Test-Path $path)) { continue }
            $wb = $excel.Workbooks.Open($path, 0, $true)
            $names = @(); foreach ($s in $wb.Worksheets) { $names += $s.Name }
            $first = $wb.Worksheets.Item(1)
            "Excel {0}: sheets=[{1}] A1='{2}' C2='{3}' E1='{4}' tables={5}" -f $name, ($names -join ', '), $first.Range("A1").Text, $first.Range("C2").Text, $first.Range("E1").Text, $first.ListObjects.Count
            $wb.Close($false)
        }
    } finally { Release $excel; Stop-Office }

    $ppt = Get-Office "powerpnt.exe" @("/n") "mdiClass"
    try {
        foreach ($name in "cs_deck.pptx", "real_ppt.pptx") {
            $pres = $ppt.Presentations.Open((Join-Path $Dir $name), -1, 0, 0)
            "PPT   {0}: slides={1} title1='{2}'" -f $name, $pres.Slides.Count, $pres.Slides.Item(1).Shapes.Item(1).TextFrame.TextRange.Text
            $pres.Close()
        }
    } finally { Release $ppt; Stop-Office }
}

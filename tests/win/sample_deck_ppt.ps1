# Dựng bộ slide mẫu theo skills/trinh-bay-chuyen-nghiep/references/bo-cuc-slide.md trên PowerPoint THẬT.
#
# Mỗi mẫu một slide (bìa, chia mục, gạch đầu dòng, ba con số, bảng, hai cột, ảnh), hộp chữ dựng đúng như
# wpp.addText trên Windows (toạ độ số nguyên, tắt tự giãn, đặt lại chiều cao). Soát từng slide theo luật
# của CommandDispatcher.Checks.cs (overflow / offslide / overlap 10% / dense 600) rồi xuất PNG 1280x720
# vào %TEMP%xiom-sample-deck để nhìn bằng mắt.
#
#     powershell -NoProfile -ExecutionPolicy Bypass -File tests\win\sample_deck_ppt.ps1
#
# Phải ĐÓNG PowerPoint trước (lý do ở đầu measure_slide_grid_ppt.ps1). Kết quả 13/10/2026: 7 slide, 0 lỗi.
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [Text.Encoding]::UTF8
if (Get-Process POWERPNT -ErrorAction SilentlyContinue) { 'PowerPoint đang chạy - đóng hết PowerPoint rồi chạy lại.'; exit 2 }
Add-Type -AssemblyName System.Drawing
$dir = Join-Path $env:TEMP 'axiom-sample-deck'
$out = $dir
New-Item -ItemType Directory -Force $out | Out-Null
function Bgr($hex) { $r = [Convert]::ToInt32($hex.Substring(1, 2), 16); $g = [Convert]::ToInt32($hex.Substring(3, 2), 16); $b = [Convert]::ToInt32($hex.Substring(5, 2), 16); return $r + 256 * $g + 65536 * $b }
$primary = Bgr '#1F4E79'; $accent = Bgr '#2E75B6'; $text = Bgr '#262626'; $muted = Bgr '#595959'

$app = New-Object -ComObject PowerPoint.Application
$pres = $app.Presentations.Add(0)
$W = [double]$pres.PageSetup.SlideWidth; $H = [double]$pres.PageSetup.SlideHeight

function Add-Text($slide, $l, $t, $w, $h, $s, $size, $bold, $color, $align) {
    # Đúng như WppAddText mới (int, tắt tự giãn, đặt lại Height).
    $sh = $slide.Shapes.AddTextbox(1, [int]$l, [int]$t, [int]$w, [int]$h)
    $sh.TextFrame.AutoSize = 0
    $tr = $sh.TextFrame.TextRange
    $tr.Text = $s
    $tr.Font.Size = $size
    if ($bold) { $tr.Font.Bold = -1 }
    $tr.Font.Color.RGB = $color
    if ($align -eq 'center') { $tr.ParagraphFormat.Alignment = 2 }
    $sh.Height = [int]$h
}
function New-Slide { return $pres.Slides.Add($pres.Slides.Count + 1, 12) }
function Grid-Title($slide, $s) { Add-Text $slide 40 30 ($W - 80) 80 $s 28 $true $primary 'left' }
function Grid-Source($slide, $s) { Add-Text $slide 40 ($H - 36) ($W - 80) 24 $s 14 $false $muted 'left' }

function Check($slide) {
    $issues = @(); $boxes = @(); $chars = 0
    foreach ($sh in $slide.Shapes) {
        $th = 0; $c = 0
        try { if ($sh.HasTextFrame -eq -1 -and $sh.TextFrame.HasText -eq -1) { $th = $sh.TextFrame.TextRange.BoundHeight; $c = $sh.TextFrame.TextRange.Text.Length } } catch {}
        $chars += $c
        $b = @{ n = $sh.Name; l = $sh.Left; t = $sh.Top; w = $sh.Width; h = $sh.Height; th = $th; c = $c }
        $boxes += $b
        if ($c -gt 0 -and $th -gt $b.h + 2) { $issues += "overflow $($b.n) ($([math]::Round($th,1)) > $([math]::Round($b.h,1)))" }
        $bottom = $b.t + [math]::Max($b.h, $th)
        if ($b.l -lt -2 -or $b.t -lt -2 -or $b.l + $b.w -gt $W + 2 -or $bottom -gt $H + 2) { $issues += "offslide $($b.n)" }
    }
    for ($i = 0; $i -lt $boxes.Count; $i++) { for ($j = $i + 1; $j -lt $boxes.Count; $j++) {
        $a = $boxes[$i]; $c2 = $boxes[$j]
        $ow = [math]::Min($a.l + $a.w, $c2.l + $c2.w) - [math]::Max($a.l, $c2.l)
        $oh = [math]::Min($a.t + $a.h, $c2.t + $c2.h) - [math]::Max($a.t, $c2.t)
        if ($ow -gt 0 -and $oh -gt 0) { $sm = [math]::Min($a.w * $a.h, $c2.w * $c2.h); if ($ow * $oh -gt 0.1 * $sm) { $issues += "overlap $($a.n) / $($c2.n)" } }
    } }
    if ($chars -gt 600) { $issues += "dense $chars" }
    return "chars=$chars " + ($(if ($issues.Count) { $issues -join '; ' } else { 'OK' }))
}

try {
    # 1. Bìa
    $s = New-Slide
    Add-Text $s 40 ([int]($H * 0.34)) ($W - 80) 110 'Báo cáo kết quả kinh doanh quý III năm 2026 của toàn công ty' 40 $true $primary 'center'
    Add-Text $s 40 ([int]($H * 0.34) + 122) ($W - 80) 70 "Phòng Kế hoạch – Tài chính`nTrình bày: Nguyễn Văn An · 13/10/2026" 24 $false $muted 'center'
    # 2. Chia mục
    $s = New-Slide
    Add-Text $s 40 (($H - 110) / 2) ($W - 80) 110 'Phần 2. Kết quả theo kênh bán hàng' 40 $true $primary 'center'
    # 3. Tiêu đề + gạch đầu dòng
    $s = New-Slide
    Grid-Title $s 'Kênh trực tuyến tăng 32%, bù được phần giảm của kênh truyền thống'
    Add-Text $s 40 130 ($W - 80) ($H - 170) ((1..5 | % { "• Kênh trực tuyến đóng góp 1,8 tỷ đồng, chiếm 41% tổng doanh thu tháng $_" }) -join "`n") 20 $false $text 'left'
    Grid-Source $s 'Nguồn: sổ bán hàng tháng 09/2026, đối chiếu báo cáo tài chính'
    # 4. Ba con số
    $s = New-Slide
    Grid-Title $s 'Ba chỉ số chính của quý đều vượt kế hoạch'
    $cw = [int](($W - 80 - 40) / 3)
    $nums = @(@('1,25 tỷ', 'Doanh thu quý III, vượt 8% kế hoạch'), @('+32%', 'Tăng trưởng kênh trực tuyến so với cùng kỳ'), @('4.120', 'Khách hàng mới trong quý, cao nhất năm'))
    for ($i = 0; $i -lt 3; $i++) {
        $l = 40 + $i * ($cw + 20)
        Add-Text $s $l ($H / 2 - 20) $cw 60 $nums[$i][0] 40 $true $accent 'center'
        Add-Text $s $l ($H / 2 + 45) $cw 60 $nums[$i][1] 20 $false $text 'center'
    }
    Grid-Source $s 'Nguồn: báo cáo quản trị quý III/2026'
    # 5. Bảng
    $s = New-Slide
    Grid-Title $s 'Miền Bắc dẫn đầu tăng trưởng, miền Trung đi ngang'
    $rows = @(@('Chỉ tiêu', 'Kỳ này', 'Kỳ trước', 'Thay đổi'), @('Miền Bắc', '520 tr', '430 tr', '+21%'), @('Miền Trung', '310 tr', '312 tr', '−1%'), @('Miền Nam', '420 tr', '380 tr', '+11%'), @('Trực tuyến', '512 tr', '388 tr', '+32%'), @('Đại lý', '190 tr', '205 tr', '−7%'), @('Tổng', '1.952 tr', '1.715 tr', '+14%'))
    $tb = $s.Shapes.AddTable(7, 4, 40, 130, [int]($W - 80), [int]($H - 170))
    for ($r = 0; $r -lt 7; $r++) { for ($c = 0; $c -lt 4; $c++) { $tb.Table.Cell($r + 1, $c + 1).Shape.TextFrame.TextRange.Text = $rows[$r][$c] } }
    Grid-Source $s 'Nguồn: sổ bán hàng các chi nhánh, quý III/2026'
    # 6. Hai cột
    $s = New-Slide
    Grid-Title $s 'Chuyển 20% ngân sách quảng cáo sang kênh trực tuyến'
    Add-Text $s 40 130 ($W / 2 - 50) ($H - 170) "Hiện tại`n• Quảng cáo báo in chiếm 60% ngân sách`n• Chi phí mỗi khách mới 410 nghìn`n• Khó đo hiệu quả theo tuần" 20 $false $text 'left'
    Add-Text $s ($W / 2 + 10) 130 ($W / 2 - 50) ($H - 170) "Đề xuất`n• Chuyển 20% sang quảng cáo số`n• Mục tiêu 250 nghìn mỗi khách mới`n• Báo cáo hiệu quả hằng tuần" 20 $false $text 'left'
    Grid-Source $s 'Nguồn: phòng Marketing, số liệu 9 tháng đầu năm 2026'
    # 7. Ảnh 16:9 rộng tối đa
    $s = New-Slide
    Grid-Title $s 'Giao diện cửa hàng trực tuyến mới ra mắt tháng 9'
    $png = Join-Path $dir 'shot.png'
    $bmp = New-Object System.Drawing.Bitmap 1600, 900; $gfx = [System.Drawing.Graphics]::FromImage($bmp); $gfx.Clear([System.Drawing.Color]::LightSteelBlue); $gfx.DrawEllipse((New-Object System.Drawing.Pen ([System.Drawing.Color]::Navy), 12), 500, 150, 600, 600); $gfx.Dispose(); $bmp.Save($png); $bmp.Dispose()
    $iw = 620   # bo-cuc-slide.md mục 6: 16:9 rộng tối đa 620 (cao 349)
    $pic = $s.Shapes.AddPicture($png, 0, -1, [int](($W - $iw) / 2), 130, -1, -1); $pic.LockAspectRatio = -1; $pic.Width = $iw
    $pic.Top = [int](130 + (370 - $pic.Height) / 2)
    Grid-Source $s 'Ảnh chụp màn hình trang chủ, 30/09/2026'

    $i = 0
    foreach ($sl in $pres.Slides) { $i++; "slide $i : " + (Check $sl); $sl.Export((Join-Path $out "s$i.png"), 'PNG', 1280, 720) }
    "PNG ở $out"
}
catch { 'LOI: ' + $_.Exception.Message + ' dong ' + $_.InvocationInfo.ScriptLineNumber }
finally { $pres.Saved = -1; try { $app.Quit() } catch {} }

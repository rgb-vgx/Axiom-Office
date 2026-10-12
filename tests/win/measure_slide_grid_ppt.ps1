# Đo lưới slide của skills/trinh-bay-chuyen-nghiep trên PowerPoint THẬT (Windows, COM).
#
# Bản đối ứng của tests/lo/measure_slide_grid.py: cùng câu mẫu, cùng lưới, nhưng hộp chữ dựng ĐÚNG như
# wpp.addText trên Windows (Shapes.AddTextbox, toạ độ số nguyên, chỉ đặt cỡ chữ). In ra số đo để chép vào
# skills/trinh-bay-chuyen-nghiep/references/co-so-do-duoc.md.
# Không phải test tự động: in số đo, người đọc so với bảng trong co-so-do-duoc.md.
#
#     powershell -NoProfile -ExecutionPolicy Bypass -File tests\win\measure_slide_grid_ppt.ps1
#
# Phải ĐÓNG PowerPoint trước khi chạy - script từ chối nếu POWERPNT.EXE đang chạy. Lý do: nó mở bài mới
# KHÔNG cửa sổ rồi bỏ đi bằng Quit, vì Presentation.Close() trên bài không cửa sổ làm POWERPNT.EXE sập
# (RPC_E_SERVERFAULT, đo 13/10/2026). Chạy chung phiên với người dùng thì hoặc làm sập bài họ đang mở,
# hoặc để lại bài rác vô hình trong phiên của họ - cả hai đều không được.
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [Text.Encoding]::UTF8

$Title = 'Kênh trực tuyến tăng 32%, bù được phần giảm của kênh truyền thống'
$Bullet = '{0}. Kênh trực tuyến đóng góp 1,8 tỷ đồng, chiếm 41% tổng doanh thu của tháng'
$Source = 'Nguồn: sổ bán hàng tháng 09/2026, đối chiếu báo cáo tài chính'
$Sample = 'Kênh bán hàng trực tuyến tăng 32% so với cùng kỳ năm ngoái, bù được phần giảm của kênh truyền thống trong tháng'
$Tolerance = 2.0   # LayoutTolerance của CommandDispatcher.Checks.cs

if (Get-Process POWERPNT -ErrorAction SilentlyContinue) {
    Write-Output 'PowerPoint đang chạy - đóng hết PowerPoint rồi chạy lại (xem ghi chú đầu file).'
    exit 2
}
$app = New-Object -ComObject PowerPoint.Application
$pres = $app.Presentations.Add(0)

function New-Box($slide, $left, $top, $width, $height, $text, $size) {
    # Đúng thứ tự của WppAddText: AddTextbox(horizontal, int...) -> tắt tự giãn -> Text -> Font.Size -> Height.
    $shape = $slide.Shapes.AddTextbox(1, [int]$left, [int]$top, [int]$width, [int]$height)
    $shape.TextFrame.AutoSize = 0
    $shape.TextFrame.TextRange.Text = $text
    $shape.TextFrame.TextRange.Font.Size = $size
    $shape.Height = [int]$height
    return $shape
}

function New-GrowBox($slide, $left, $top, $width, $text, $size) {
    # Hộp tự giãn (mặc định của AddTextbox): chỉ để ĐO chiều cao chữ cần, không phải cách wpp dựng hộp.
    $shape = $slide.Shapes.AddTextbox(1, [int]$left, [int]$top, [int]$width, 20)
    $shape.TextFrame.TextRange.Text = $text
    $shape.TextFrame.TextRange.Font.Size = $size
    return $shape
}

function Test-Overflow($shape) {
    # Luật overflow của CommandDispatcher.Checks.cs: BoundHeight > Height + LayoutTolerance.
    return $shape.TextFrame.TextRange.BoundHeight -gt $shape.Height + $Tolerance
}

function Fit-Line($slide, $width, $size, $text) {
    # Dò nhị phân: chuỗi dài nhất (cắt từ $text) còn nằm trên MỘT dòng.
    $shape = New-GrowBox $slide 0 0 $width 'x' $size
    $low = 1; $high = $text.Length
    while ($low -lt $high) {
        $mid = [int][math]::Ceiling(($low + $high) / 2)
        $shape.TextFrame.TextRange.Text = $text.Substring(0, $mid)
        if ($shape.TextFrame.TextRange.Lines().Count -le 1) { $low = $mid } else { $high = $mid - 1 }
    }
    $shape.Delete()
    return $low
}

try {
    $W = [double]$pres.PageSetup.SlideWidth
    $H = [double]$pres.PageSetup.SlideHeight
    $slide = $pres.Slides.Add(1, 12)
    Write-Output ('== 1. Slide mặc định: {0} x {1}pt' -f $W, $H)

    $probe = New-Box $slide 40 30 ($W - 80) 80 "a`nb" 18
    $tf = $probe.TextFrame
    Write-Output ('== 2. Hộp của wpp.addText: AutoSize={0} (0 = giữ chiều cao đã đặt), WordWrap={1}, phông {2}, lề trong T{3} B{4} L{5} R{6}; "\n" -> {7} đoạn, {8} dòng' -f `
        $tf.AutoSize, $tf.WordWrap, $tf.TextRange.Font.Name, $tf.MarginTop, $tf.MarginBottom, $tf.MarginLeft, $tf.MarginRight, `
        $tf.TextRange.Paragraphs().Count, $tf.TextRange.Lines().Count)
    $probe.Delete()

    Write-Output '== 3. Chiều cao CẦN cho n dòng: hộp tự giãn về bao nhiêu (shape.Height) và dòng chữ (BoundHeight)'
    foreach ($size in 14, 18, 20, 24, 28, 40) {
        $cells = @()
        foreach ($n in 1, 2, 4, 8) {
            $text = (1..$n | ForEach-Object { "Dòng $_ có dấu" }) -join "`n"
            $shape = New-GrowBox $slide 40 30 ($W - 80) $text $size
            $cells += ('{0}: {1:N1}/{2:N1}' -f $n, $shape.Height, $shape.TextFrame.TextRange.BoundHeight)
            $shape.Delete()
        }
        Write-Output ('   {0,2}pt  dòng: cao hộp/cao chữ   {1}' -f $size, ($cells -join '   '))
    }

    Write-Output '== 4. Bề ngang ký tự: chuỗi dài nhất vừa MỘT dòng'
    $letters = ('abcdefghijklmnopqrstuvwxyz' * 8)
    $prose = ($Sample + ' ') * 4
    foreach ($width in ($W - 80), 713, (($W / 2) - 50)) {
        foreach ($size in 14, 20, 28, 40) {
            $a = Fit-Line $slide $width $size $letters
            $b = Fit-Line $slide $width $size $prose
            Write-Output ('   rộng {0,5:N1}  {1,2}pt  chữ liền {2,3} ký tự ({3:N3} x cỡ)   văn xuôi {4,3} ký tự ({5:N3} x cỡ)' -f `
                $width, $size, $a, ([int]$width / $a / $size), $b, ([int]$width / $b / $size))
        }
    }

    Write-Output '== 5. Slide đầy nhất theo lưới (đúng như skill hướng dẫn), soát theo luật của checkLayout'
    $bullets = (1..5 | ForEach-Object { $Bullet -f $_ }) -join "`n"
    $boxes = @(
        (New-Box $slide 40 30 ($W - 80) 80 $Title 28),
        (New-Box $slide 40 130 ($W - 80) ($H - 170) $bullets 20),
        (New-Box $slide 40 ($H - 36) ($W - 80) 24 $Source 14)
    )
    $names = 'tiêu đề', 'nội dung', 'nguồn'
    $planned = 80, ($H - 170), 24
    $chars = 0
    for ($i = 0; $i -lt 3; $i++) {
        $s = $boxes[$i]
        $chars += $s.TextFrame.TextRange.Text.Length
        Write-Output ('   {0,-9} hộp cao {1,5:N1}, chữ cao {2,5:N1}, {3} dòng, overflow = {4}' -f `
            $names[$i], $s.Height, $s.TextFrame.TextRange.BoundHeight, $s.TextFrame.TextRange.Lines().Count, (Test-Overflow $s))
    }
    Write-Output ('   tổng {0} ký tự (dense > 600)' -f $chars)

    Write-Output '== 6. Tiêu đề dài 3 dòng: có lộ ra không?'
    $long = New-Box $slide 40 30 ($W - 80) 80 ($Title + ', nhờ chương trình khuyến mãi tháng 9 và kênh đối tác mới mở tại ba tỉnh') 28
    $lines = $long.TextFrame.TextRange.Lines().Count
    Write-Output ('   wpp.addText (hộp cố định): {0} dòng, hộp cao {1:N1}, chữ cao {2:N1} -> overflow = {3}' -f `
        $lines, $long.Height, $long.TextFrame.TextRange.BoundHeight, (Test-Overflow $long))
    $long.Delete()
    # Hộp tự giãn (cách cũ của wpp.addText trước 13/10/2026): đè xuống nội dung mà overlap không đủ 10%.
    $grow = New-GrowBox $slide 40 30 ($W - 80) ($Title + ', nhờ chương trình khuyến mãi tháng 9 và kênh đối tác mới mở tại ba tỉnh') 28
    $overlapH = [math]::Max(0, $grow.Top + $grow.Height - 130)
    Write-Output ('   hộp tự giãn (cách cũ): giãn tới {0:N1}, đè lên nội dung {1:N1}pt — overlap không báo vì dưới 10%' -f $grow.Height, $overlapH)
    $grow.Delete()

    Write-Output '== 7. Bảng: cỡ chữ ô mặc định, chiều cao khi ép 7 hàng (1 tiêu đề + 6 dữ liệu) vào vùng nội dung'
    $table = $slide.Shapes.AddTable(7, 4, 40, 130, [int]($W - 80), [int]($H - 170))
    $cell = $table.Table.Cell(2, 1).Shape.TextFrame
    $cell.TextRange.Text = 'Doanh thu kênh trực tuyến'
    Write-Output ('   cỡ chữ ô {0}pt, phông {1}; bảng cao {2:N1} (vùng {3:N1}), hàng 2 cao {4:N1}' -f `
        $cell.TextRange.Font.Size, $cell.TextRange.Font.Name, $table.Height, ($H - 170), $table.Table.Rows(2).Height)
    $table.Delete()
    foreach ($rows in 10, 13) {
        $table = $slide.Shapes.AddTable($rows, 4, 40, 130, [int]($W - 80), [int]($H - 170))
        Write-Output ('   {0} hàng trống: bảng cao {1:N1}' -f $rows, $table.Height)
        $table.Delete()
    }
    $long = 'Doanh thu kênh trực tuyến của toàn bộ chi nhánh phía Bắc trong quý ba'
    $table = $slide.Shapes.AddTable(7, 4, 40, 130, [int]($W - 80), [int]($H - 170))
    $table.Table.Cell(2, 1).Shape.TextFrame.TextRange.Text = $long
    # Rows(n).Height và Lines() của ô chưa cập nhật ngay sau khi gán chữ - chỉ tin chiều cao cả bảng.
    Write-Output ('   7 hàng, một ô {0} ký tự (cột {1:N0}pt): bảng giãn tới {2:N1}, đáy {3:N1}' -f `
        $long.Length, $table.Table.Columns(1).Width, $table.Height, ($table.Top + $table.Height))
    $table.Delete()

    Write-Output '== 8. Ảnh: wpp.addImage chỉ truyền width (làm như WppAddImage: AddPicture -1/-1, khoá tỉ lệ, đặt Width)'
    Add-Type -AssemblyName System.Drawing
    foreach ($px in @(@(1600, 900), @(800, 600))) {
        $png = Join-Path $env:TEMP ('axiom-measure-{0}x{1}.png' -f $px[0], $px[1])
        $bmp = New-Object System.Drawing.Bitmap $px[0], $px[1]
        $bmp.Save($png, [System.Drawing.Imaging.ImageFormat]::Png); $bmp.Dispose()
        $raw = $slide.Shapes.AddPicture($png, 0, -1, 40, 130, 491, -1)
        $pic = $slide.Shapes.AddPicture($png, 0, -1, 40, 130, -1, -1)
        $pic.LockAspectRatio = -1
        $pic.Width = 491
        Write-Output ('   ảnh {0}x{1}px: AddPicture(width 491, height -1) -> {2:N1} x {3:N1} (méo); khoá tỉ lệ -> {4:N1} x {5:N1}' -f `
            $px[0], $px[1], $raw.Width, $raw.Height, $pic.Width, $pic.Height)
        $raw.Delete(); $pic.Delete(); Remove-Item $png
    }

    Write-Output '== 9. Layout của wpp.addSlide: số placeholder, có chữ không'
    foreach ($layout in 1, 2, 11, 12) {
        $s = $pres.Slides.Add($pres.Slides.Count + 1, $layout)
        $texts = @()
        foreach ($sh in $s.Shapes) { $texts += [int]$sh.TextFrame.HasText }
        Write-Output ('   layout {0,2}: {1} hộp, HasText = [{2}]' -f $layout, $s.Shapes.Count, ($texts -join ','))
    }
}
catch {
    Write-Output ('LOI: {0}  (dòng {1})' -f $_.Exception.Message, $_.InvocationInfo.ScriptLineNumber)
    exit 1
}
finally {
    $pres.Saved = -1
    # Quit có thể ném 0x80048010 khi còn bài không cửa sổ - PowerPoint vẫn tắt, bỏ qua.
    try { $app.Quit() } catch { }
}

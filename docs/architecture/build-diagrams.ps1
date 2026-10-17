param(
  [string]$Exe = "C:\Program Files\draw.io\draw.io.exe",
  [switch]$NoTrim
)
# Dung lai so do kien truc:
#   docs/architecture/src/*.mmd   ->  docs/architecture/img/*.drawio      (mo/sua bang draw.io)
#                                 ->  docs/architecture/img/*.drawio.svg  (nhung XML de con sua duoc)
# draw.io xuat moi nhan thanh <foreignObject> kem anh PNG du phong trong <switch>, lam tep phinh ~20 lan.
# Buoc Trim go cac anh PNG du phong do (giu foreignObject) cho nhe ma van net tren trinh duyet.
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
$src = Join-Path $root "src"
$img = Join-Path $root "img"
New-Item -ItemType Directory -Force -Path $img | Out-Null

function Invoke-Drawio([string[]]$argList, [string]$outFile, [int]$timeoutSec = 120) {
  if (Test-Path $outFile) { Remove-Item $outFile -Force }
  Start-Process -FilePath $Exe -ArgumentList $argList -NoNewWindow | Out-Null
  $deadline = (Get-Date).AddSeconds($timeoutSec)
  while (-not (Test-Path $outFile) -and (Get-Date) -lt $deadline) { Start-Sleep -Milliseconds 300 }
  if (Test-Path $outFile) { return $true } else { Write-Warning "FAIL $outFile"; return $false }
}

function Remove-FallbackImages([string]$path) {
  $raw = [System.IO.File]::ReadAllText($path)
  $trimmed = [regex]::Replace($raw, '<image\b[^>]*xlink:href="data:image/png;base64,[^"]*"[^>]*/>', '')
  if ($trimmed -ne $raw) {
    [System.IO.File]::WriteAllText($path, $trimmed, (New-Object System.Text.UTF8Encoding($false)))
    return $true
  }
  return $false
}

$fails = @()
Get-ChildItem $src -Filter *.mmd | Sort-Object Name | ForEach-Object {
  $name = $_.BaseName
  $drawio = Join-Path $img ($name + ".drawio")
  $svg = Join-Path $img ($name + ".drawio.svg")
  if (-not (Invoke-Drawio @("-x","-f","xml","-o",$drawio,$_.FullName) $drawio)) { $fails += $name; return }
  if (-not (Invoke-Drawio @("-x","-f","svg","-e","-b","10","-o",$svg,$drawio) $svg)) { $fails += $name; return }
  if (-not $NoTrim) { [void](Remove-FallbackImages $svg) }
  $kb = [math]::Round((Get-Item $svg).Length / 1KB)
  Write-Output ("done {0,-26} {1} KB" -f $name, $kb)
}
if ($fails.Count) { Write-Output ("FAILED: " + ($fails -join ", ")) } else { Write-Output "ALL OK" }

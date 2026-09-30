param(
    # Dong goi .oxt tu src\AxiomOffice.LibreOffice vao dist (mac dinh).
    [switch]$Package,
    # Cai .oxt vao profile LibreOffice cua nguoi dung (unopkg add --force). Phai tat LibreOffice truoc.
    [switch]$Install,
    # Go extension.
    [switch]$Uninstall,
    # In trang thai: duong dan unopkg, extension da cai, session/health cua bridge.
    [switch]$Status,
    # In 40 dong cuoi cua bridge.log.
    [switch]$Log,
    # Thu muc cai LibreOffice (mac dinh C:\Program Files\LibreOffice).
    [string]$SoPath = "C:\Program Files\LibreOffice"
)

# Extension bridge LibreOffice (LibreOffice_arch.md muc 10, giai doan L1): dong goi .oxt, cai bang unopkg,
# xem log. Khong bao gio tat LibreOffice cua nguoi dung - chi bao neu dang chay.

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$root = Split-Path -Parent $PSScriptRoot
$source = Join-Path $root "src\AxiomOffice.LibreOffice"
$dist = Join-Path $root "dist"
$unopkg = Join-Path $SoPath "program\unopkg.com"
$logFile = Join-Path $env:LOCALAPPDATA "AxiomOffice\bridge.log"

if (-not ($Package -or $Install -or $Uninstall -or $Status -or $Log)) {
    $Package = $true
}

function Get-BridgeVersion {
    $init = Join-Path $source "python\pythonpath\axiom\__init__.py"
    $match = Select-String -LiteralPath $init -Pattern 'VERSION\s*=\s*"([^"]+)"' | Select-Object -First 1
    if (-not $match) {
        throw "Khong doc duoc VERSION trong $init"
    }
    return $match.Matches[0].Groups[1].Value
}

function Get-SofficeProcesses {
    return @(Get-Process -Name soffice, soffice.bin -ErrorAction SilentlyContinue)
}

function Build-Oxt {
    $version = Get-BridgeVersion
    $oxt = Join-Path $dist "AxiomOffice-LibreOffice-$version.oxt"
    if (-not (Test-Path -LiteralPath $dist)) {
        New-Item -ItemType Directory -Path $dist -Force | Out-Null
    }
    if (Test-Path -LiteralPath $oxt) {
        Remove-Item -LiteralPath $oxt -Force
    }

    # Giu dung cau truc: description.xml + META-INF/manifest.xml + Jobs.xcu + python/ (xem pythonloader.py:
    # thu muc pythonpath canh file component duoc them vao sys.path).
    $files = @(Get-ChildItem -LiteralPath $source -Recurse -File -Force |
        Where-Object { $_.FullName -notmatch "__pycache__" -and $_.Extension -ne ".pyc" })
    $archive = [System.IO.Compression.ZipFile]::Open($oxt, [System.IO.Compression.ZipArchiveMode]::Create)
    try {
        $prefixLength = $source.Length + 1
        foreach ($file in $files) {
            $name = $file.FullName.Substring($prefixLength).Replace("\", "/")
            [void][System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile(
                $archive, $file.FullName, $name, [System.IO.Compression.CompressionLevel]::Optimal)
        }
    } finally {
        $archive.Dispose()
    }
    Write-Host "Packaged $oxt ($($files.Count) files)"
    $script:oxt = $oxt
}

function Assert-LibreOfficeClosed {
    $running = Get-SofficeProcesses
    if ($running.Count -gt 0) {
        $ids = ($running | ForEach-Object { $_.Id }) -join ", "
        throw "LibreOffice dang chay (pid $ids). Hay dong LibreOffice roi chay lai - script khong tu tat."
    }
}

function Install-Oxt([string]$oxt) {
    Assert-LibreOfficeClosed
    if (-not (Test-Path -LiteralPath $unopkg)) {
        throw "Khong thay unopkg.com trong $SoPath\program - dung -SoPath de chi duong cai LibreOffice."
    }
    & $unopkg add --force $oxt
    if ($LASTEXITCODE -ne 0) {
        throw "unopkg add that bai (exit $LASTEXITCODE)"
    }
    Write-Host "Installed $oxt"
}

function Uninstall-Oxt {
    Assert-LibreOfficeClosed
    & $unopkg remove org.axiomoffice.bridge
    if ($LASTEXITCODE -ne 0) {
        Write-Output "unopkg remove tra ve exit $LASTEXITCODE (extension co the chua duoc cai)."
    } else {
        Write-Host "Removed org.axiomoffice.bridge"
    }
}

function Show-Status {
    Write-Output "unopkg: $unopkg"
    if (Test-Path -LiteralPath $unopkg) {
        & $unopkg list
    } else {
        Write-Output "Khong thay unopkg.com - kiem tra -SoPath."
    }
    $sessions = Join-Path $env:LOCALAPPDATA "AxiomOffice\sessions"
    if (Test-Path -LiteralPath $sessions) {
        Write-Output ""
        Write-Output "Sessions ($sessions):"
        Get-ChildItem -LiteralPath $sessions -Filter "*libreoffice*" -ErrorAction SilentlyContinue | ForEach-Object { $_.Name }
        Get-ChildItem -LiteralPath $sessions -Filter "*.json" -ErrorAction SilentlyContinue | ForEach-Object {
            $text = Get-Content -LiteralPath $_.FullName -Raw
            if ($text -match '"family"\s*:\s*"libreoffice"') {
                Write-Output ("  {0}: {1}" -f $_.Name, ($text -replace "\s+", " "))
            }
        }
    }
    foreach ($port in 47851, 47852, 47853) {
        try {
            $health = Invoke-RestMethod -Uri ("http://127.0.0.1:{0}/health" -f $port) -TimeoutSec 2
            Write-Output ("Health {0}: ok={1} app={2} pid={3}" -f $port, $health.ok, $health.result.app, $health.result.pid)
        } catch {
            Write-Output ("Health {0}: khong tra loi" -f $port)
        }
    }
}

function Show-Log {
    if (-not (Test-Path -LiteralPath $logFile)) {
        Write-Output "Chua co $logFile"
        return
    }
    Get-Content -LiteralPath $logFile -Tail 40
}

if ($Uninstall) {
    Uninstall-Oxt
}
if ($Package -or $Install) {
    Build-Oxt
}
if ($Install) {
    Install-Oxt $script:oxt
}
if ($Status) {
    Show-Status
}
if ($Log) {
    Show-Log
}

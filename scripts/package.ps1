param(
    # Dung ban da build san trong bin\Release (khong build lai).
    [switch]$NoBuild,
    # Truyen cho build.ps1: tu tat app dang lock DLL.
    [switch]$Kill,
    # Thu muc dau ra (mac dinh <repo>\dist).
    [string]$OutDir
)

# Dong goi ban cai cho nguoi khac: add-in DLL + companion/MCP EXE + script cai/go + huong dan.
# Nguoi nhan chi can giai nen va nhap dup install.cmd (khong can Python, khong can quyen admin).

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$root = Split-Path -Parent $PSScriptRoot
$release = Join-Path $root "src\WpsAiBridge\bin\Release"
if (-not $OutDir) {
    $OutDir = Join-Path $root "dist"
}

if (-not $NoBuild) {
    & (Join-Path $PSScriptRoot "build.ps1") -Kill:$Kill
    if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

$dll = Join-Path $release "WpsAiBridge.dll"
$exe = Join-Path $release "WpsAiBridge.Host.exe"
foreach ($file in @($dll, $exe)) {
    if (-not (Test-Path -LiteralPath $file)) {
        throw "Thieu $file - chay scripts\build.ps1 truoc (hoac bo -NoBuild)"
    }
}

$version = (Get-Item -LiteralPath $dll).VersionInfo.FileVersion
$commit = ""
try {
    $commit = (& git -C $root rev-parse --short HEAD 2>$null)
    if ($commit -and (& git -C $root status --porcelain 2>$null)) {
        $commit += "-dirty"
    }
} catch {
}
$name = "WpsAiBridge-$version-$(Get-Date -Format yyyyMMdd)"
if ($commit) {
    $name += "-$commit"
}

New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
$stage = Join-Path $OutDir $name
$zip = "$stage.zip"
if (Test-Path -LiteralPath $stage) {
    Remove-Item -LiteralPath $stage -Recurse -Force
}
if (Test-Path -LiteralPath $zip) {
    Remove-Item -LiteralPath $zip -Force
}

# Giu nguyen cau truc thu muc: install.ps1 tim DLL o ..\src\WpsAiBridge\bin\Release tinh tu scripts\.
$layout = [ordered]@{
    "scripts\install.ps1"                            = Join-Path $root "scripts\install.ps1"
    "scripts\uninstall.ps1"                          = Join-Path $root "scripts\uninstall.ps1"
    "src\WpsAiBridge\bin\Release\WpsAiBridge.dll"      = $dll
    "src\WpsAiBridge\bin\Release\WpsAiBridge.Host.exe" = $exe
    "install.cmd"                                    = Join-Path $PSScriptRoot "dist\install.cmd"
    "uninstall.cmd"                                  = Join-Path $PSScriptRoot "dist\uninstall.cmd"
    "HUONG-DAN-CAI-DAT.txt"                          = Join-Path $PSScriptRoot "dist\HUONG-DAN-CAI-DAT.txt"
    "THIRD-PARTY-NOTICES.md"                         = Join-Path $root "src\WpsAiBridge.Host\Mcp\Templates\NOTICE.md"
}
foreach ($entry in $layout.GetEnumerator()) {
    $target = Join-Path $stage $entry.Key
    New-Item -ItemType Directory -Path (Split-Path -Parent $target) -Force | Out-Null
    Copy-Item -LiteralPath $entry.Value -Destination $target -Force
}

# Tu ghi tung entry voi "/" (CreateFromDirectory trong PowerShell 5.1 ghi "\", cong cu ngoai Windows doc sai).
$archive = [System.IO.Compression.ZipFile]::Open($zip, [System.IO.Compression.ZipArchiveMode]::Create)
try {
    foreach ($entry in $layout.GetEnumerator()) {
        $entryName = ($name + "/" + $entry.Key) -replace "\\", "/"
        [void][System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, (Join-Path $stage $entry.Key), $entryName, [System.IO.Compression.CompressionLevel]::Optimal)
    }
} finally {
    $archive.Dispose()
}

$hash = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash
$size = [math]::Round((Get-Item -LiteralPath $zip).Length / 1KB, 1)
Write-Output "Package: $zip ($size KB)"
Write-Output "SHA256:  $hash"
Write-Output "Noi dung:"
$archive = [System.IO.Compression.ZipFile]::OpenRead($zip)
try {
    foreach ($item in $archive.Entries) {
        Write-Output ("  {0,-70} {1,8}" -f $item.FullName, $item.Length)
    }
} finally {
    $archive.Dispose()
}

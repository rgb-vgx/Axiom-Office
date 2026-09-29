# Cai .NET 10 SDK cho RIENG nguoi dung hien tai (khong can quyen admin, khong dung Program Files).
# Chi can cho viec build Agent Core (AxiomOffice.Core.exe) - nguoi dung cuoi khong phai cai gi vi
# build.ps1 publish ban self-contained. Xem New_arch.md muc 5.2.
#
#   powershell -ExecutionPolicy Bypass -File scripts\install-dotnet-sdk.ps1

param(
    # Thu muc cai dat (mac dinh %LOCALAPPDATA%\Microsoft\dotnet).
    [string]$InstallDir,
    # Kenh SDK (mac dinh 10.0 = ban LTS).
    [string]$Channel = "10.0"
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "Microsoft\dotnet"
}

$installer = Join-Path $env:TEMP "dotnet-install.ps1"
if (-not (Test-Path -LiteralPath $installer)) {
    Write-Output "Downloading dotnet-install.ps1 from dot.net ..."
    Invoke-WebRequest -Uri "https://dot.net/v1/dotnet-install.ps1" -OutFile $installer -UseBasicParsing
}

Write-Output "Installing .NET SDK $Channel into $InstallDir ..."
& $installer -Channel $Channel -InstallDir $InstallDir -NoPath
if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) {
    throw "dotnet-install.ps1 failed with exit code $LASTEXITCODE"
}

$dotnet = Join-Path $InstallDir "dotnet.exe"
Write-Output "Installed SDKs:"
& $dotnet --list-sdks
Write-Output ""
Write-Output "scripts\build.ps1 finds this automatically. For running the framework-dependent build"
Write-Output "(dotnet build/test without publish) set DOTNET_ROOT=$InstallDir"

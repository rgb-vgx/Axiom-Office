param(
    # Xoa ca cau hinh (HKCU\Software\WpsAiBridge: token, cai dat AI) va %LOCALAPPDATA%\WpsAiBridge (log, session).
    [switch]$Purge
)

$ErrorActionPreference = "Stop"

$progId = "WpsAiBridge.Connect"
$classGuid = "{F4524DFD-C4F6-4027-8CA6-08B7F7DB4C44}"
$currentUser = [Microsoft.Win32.Registry]::CurrentUser

$currentUser.DeleteSubKeyTree("Software\Classes\$progId", $false)
$currentUser.DeleteSubKeyTree("Software\Classes\CLSID\$classGuid", $false)
# Class cua Ask AI task pane (dang ky boi install.ps1)
$currentUser.DeleteSubKeyTree("Software\Classes\WpsAiBridge.AskAiPane", $false)
$currentUser.DeleteSubKeyTree("Software\Classes\CLSID\{D99F8693-4316-45AF-8916-B70D87DEEF87}", $false)

foreach ($officeApp in @("Word", "Excel", "PowerPoint")) {
    $currentUser.DeleteSubKeyTree("Software\Microsoft\Office\$officeApp\Addins\$progId", $false)
}

foreach ($hive in @("WPS", "ET", "WPP")) {
    $wl = $currentUser.OpenSubKey("Software\Kingsoft\Office\$hive\AddinsWL", $true)
    if ($wl -ne $null) {
        $wl.DeleteValue($progId, $false)
        $wl.Close()
    }
    $cl = $currentUser.OpenSubKey("Software\Kingsoft\Office\$hive\AddinsCL", $true)
    if ($cl -ne $null) {
        $cl.DeleteValue($progId, $false)
        $cl.Close()
    }
}

if ($Purge) {
    $currentUser.DeleteSubKeyTree("Software\WpsAiBridge", $false)
    $dataDir = Join-Path $env:LOCALAPPDATA "WpsAiBridge"
    if (Test-Path -LiteralPath $dataDir) {
        Remove-Item -LiteralPath $dataDir -Recurse -Force -ErrorAction SilentlyContinue
    }
    Write-Output "Uninstalled WpsAiBridge and removed config (HKCU\Software\WpsAiBridge) and $dataDir."
} else {
    Write-Output "Uninstalled WpsAiBridge (config key HKCU\Software\WpsAiBridge kept; use -Purge to remove it)."
}
Write-Output "Close WPS first if it is running."

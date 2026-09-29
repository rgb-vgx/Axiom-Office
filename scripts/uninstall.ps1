param(
    # Xoa ca cau hinh (HKCU\Software\AxiomOffice: token, cai dat AI) va %LOCALAPPDATA%\AxiomOffice (log, session).
    [switch]$Purge
)

$ErrorActionPreference = "Stop"

$progId = "AxiomOffice.Connect"
$classGuid = "{BDB3732A-A479-4A24-AD64-D35952035BBA}"
$currentUser = [Microsoft.Win32.Registry]::CurrentUser
. (Join-Path $PSScriptRoot "legacy.ps1")
[void](Remove-LegacyRegistration)

$currentUser.DeleteSubKeyTree("Software\Classes\$progId", $false)
$currentUser.DeleteSubKeyTree("Software\Classes\CLSID\$classGuid", $false)
# Class cua Ask AI task pane (dang ky boi install.ps1)
$currentUser.DeleteSubKeyTree("Software\Classes\AxiomOffice.AskAiPane", $false)
$currentUser.DeleteSubKeyTree("Software\Classes\CLSID\{8001B0D7-F189-443A-B3CB-6EB98038C72E}", $false)

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
    $currentUser.DeleteSubKeyTree("Software\AxiomOffice", $false)
    $currentUser.DeleteSubKeyTree($LegacyConfigKey, $false)
    if (Test-Path -LiteralPath $LegacyDataDir) {
        Remove-Item -LiteralPath $LegacyDataDir -Recurse -Force -ErrorAction SilentlyContinue
    }
    $dataDir = Join-Path $env:LOCALAPPDATA "AxiomOffice"
    if (Test-Path -LiteralPath $dataDir) {
        Remove-Item -LiteralPath $dataDir -Recurse -Force -ErrorAction SilentlyContinue
    }
    Write-Output "Uninstalled Axiom Office and removed config (HKCU\Software\AxiomOffice) and $dataDir."
} else {
    Write-Output "Uninstalled Axiom Office (config key HKCU\Software\AxiomOffice kept; use -Purge to remove it)."
}
Write-Output "Close WPS first if it is running."

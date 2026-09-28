$ErrorActionPreference = "Stop"

$progId = "WpsAiBridge.Connect"
$classGuid = "{F4524DFD-C4F6-4027-8CA6-08B7F7DB4C44}"
$currentUser = [Microsoft.Win32.Registry]::CurrentUser

$currentUser.DeleteSubKeyTree("Software\Classes\$progId", $false)
$currentUser.DeleteSubKeyTree("Software\Classes\CLSID\$classGuid", $false)

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

Write-Output "Uninstalled WpsAiBridge (config key HKCU\Software\WpsAiBridge kept)."
Write-Output "Close WPS first if it is running."

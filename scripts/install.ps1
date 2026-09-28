$ErrorActionPreference = "Stop"

$progId = "WpsAiBridge.Connect"
$classGuid = "{F4524DFD-C4F6-4027-8CA6-08B7F7DB4C44}"
$dllVersion = "1.0.0.0"

$root = Split-Path -Parent $PSScriptRoot
$dllPath = Join-Path $root "src\WpsAiBridge\bin\Release\WpsAiBridge.dll"

if (-not (Test-Path -LiteralPath $dllPath)) {
    throw "DLL not found: $dllPath - run scripts\build.ps1 first"
}

$codeBase = "file:///" + ($dllPath -replace '\\', '/')
$currentUser = [Microsoft.Win32.Registry]::CurrentUser

function New-Key([string]$path) {
    return $currentUser.CreateSubKey($path)
}

$progKey = New-Key "Software\Classes\$progId"
$progKey.SetValue("", $progId, [Microsoft.Win32.RegistryValueKind]::String)
$progClsid = $progKey.CreateSubKey("CLSID")
$progClsid.SetValue("", $classGuid, [Microsoft.Win32.RegistryValueKind]::String)
$progKey.Close()

$clsidKey = New-Key "Software\Classes\CLSID\$classGuid"
$clsidKey.SetValue("", $progId, [Microsoft.Win32.RegistryValueKind]::String)
$inproc = $clsidKey.CreateSubKey("InprocServer32")
$inproc.SetValue("", "mscoree.dll", [Microsoft.Win32.RegistryValueKind]::String)
$inproc.SetValue("ThreadingModel", "Both", [Microsoft.Win32.RegistryValueKind]::String)
$inproc.SetValue("CodeBase", $codeBase, [Microsoft.Win32.RegistryValueKind]::String)
$versioned = $inproc.CreateSubKey($dllVersion)
$versioned.SetValue("Assembly", "WpsAiBridge, Version=$dllVersion, Culture=neutral, PublicKeyToken=null", [Microsoft.Win32.RegistryValueKind]::String)
$versioned.SetValue("Class", $progId, [Microsoft.Win32.RegistryValueKind]::String)
$versioned.SetValue("RuntimeVersion", "v4.0.30319", [Microsoft.Win32.RegistryValueKind]::String)
$versioned.SetValue("CodeBase", $codeBase, [Microsoft.Win32.RegistryValueKind]::String)
$versioned.Close()
$inproc.Close()
$clsidKey.Close()

foreach ($officeApp in @("Word", "Excel", "PowerPoint")) {
    $key = New-Key "Software\Microsoft\Office\$officeApp\Addins\$progId"
    $key.SetValue("FriendlyName", "WPS AI Bridge", [Microsoft.Win32.RegistryValueKind]::String)
    $key.SetValue("Description", "Local HTTP bridge that lets an AI agent inspect and edit open documents in WPS Office", [Microsoft.Win32.RegistryValueKind]::String)
    $key.SetValue("LoadBehavior", 3, [Microsoft.Win32.RegistryValueKind]::DWord)
    $key.SetValue("CommandLineSafe", 1, [Microsoft.Win32.RegistryValueKind]::DWord)
    $key.Close()
}

foreach ($hive in @("WPS", "ET", "WPP")) {
    $wl = New-Key "Software\Kingsoft\Office\$hive\AddinsWL"
    $wl.SetValue($progId, "", [Microsoft.Win32.RegistryValueKind]::String)
    $wl.Close()

    $cl = $currentUser.OpenSubKey("Software\Kingsoft\Office\$hive\AddinsCL", $true)
    if ($cl -ne $null) {
        $cl.DeleteValue($progId, $false)
        $cl.Close()
    }
}

$config = New-Key "Software\WpsAiBridge"
if ($config.GetValue("Port") -eq $null) {
    $config.SetValue("Port", 47821, [Microsoft.Win32.RegistryValueKind]::DWord)
}
if ($config.GetValue("Enabled") -eq $null) {
    $config.SetValue("Enabled", 1, [Microsoft.Win32.RegistryValueKind]::DWord)
}
$port = $config.GetValue("Port")
$config.Close()

Write-Output "Installed WpsAiBridge."
Write-Output "  DLL:      $dllPath"
Write-Output "  ProgID:   $progId"
Write-Output "  HTTP:     http://127.0.0.1:$port/"
Write-Output "  Config:   HKCU\Software\WpsAiBridge (Port, Token, Enabled)"
Write-Output ""
Write-Output "Next: open WPS Writer/Spreadsheets/Presentation."
Write-Output "If the add-in does not load, open Tools tab -> COM Add-ins and enable 'WPS AI Bridge'"
Write-Output "(WPS blocks uncertified COM add-ins by default; a one-time manual enable is required)."

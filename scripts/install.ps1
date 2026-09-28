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

function Register-ComClass([string]$progIdToRegister, [string]$classGuidToRegister, [string]$className) {
    $progKey = New-Key "Software\Classes\$progIdToRegister"
    $progKey.SetValue("", $progIdToRegister, [Microsoft.Win32.RegistryValueKind]::String)
    $progClsid = $progKey.CreateSubKey("CLSID")
    $progClsid.SetValue("", $classGuidToRegister, [Microsoft.Win32.RegistryValueKind]::String)
    $progKey.Close()

    $clsidKey = New-Key "Software\Classes\CLSID\$classGuidToRegister"
    $clsidKey.SetValue("", $progIdToRegister, [Microsoft.Win32.RegistryValueKind]::String)
    $inproc = $clsidKey.CreateSubKey("InprocServer32")
    $inproc.SetValue("", "mscoree.dll", [Microsoft.Win32.RegistryValueKind]::String)
    $inproc.SetValue("ThreadingModel", "Both", [Microsoft.Win32.RegistryValueKind]::String)
    $inproc.SetValue("CodeBase", $codeBase, [Microsoft.Win32.RegistryValueKind]::String)
    $versioned = $inproc.CreateSubKey($dllVersion)
    $versioned.SetValue("Assembly", "WpsAiBridge, Version=$dllVersion, Culture=neutral, PublicKeyToken=null", [Microsoft.Win32.RegistryValueKind]::String)
    $versioned.SetValue("Class", $className, [Microsoft.Win32.RegistryValueKind]::String)
    $versioned.SetValue("RuntimeVersion", "v4.0.30319", [Microsoft.Win32.RegistryValueKind]::String)
    $versioned.SetValue("CodeBase", $codeBase, [Microsoft.Win32.RegistryValueKind]::String)
    $versioned.Close()
    $inproc.Close()
    $clsidKey.Close()
}

Register-ComClass $progId $classGuid $progId
Register-ComClass "WpsAiBridge.AskAiPane" "{D99F8693-4316-45AF-8916-B70D87DEEF87}" "WpsAiBridge.Ai.AskAiPane"

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
if ($config.GetValue("PortOffice") -eq $null) {
    $config.SetValue("PortOffice", 47831, [Microsoft.Win32.RegistryValueKind]::DWord)
}
if ($config.GetValue("Enabled") -eq $null) {
    $config.SetValue("Enabled", 1, [Microsoft.Win32.RegistryValueKind]::DWord)
}
$token = $config.GetValue("Token")
$tokenGenerated = $false
if ([string]::IsNullOrEmpty($token)) {
    $bytes = New-Object byte[] 16
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $token = -join ($bytes | ForEach-Object { $_.ToString("x2") })
    $config.SetValue("Token", $token, [Microsoft.Win32.RegistryValueKind]::String)
    $tokenGenerated = $true
}
$port = $config.GetValue("Port")
$portOffice = $config.GetValue("PortOffice")
$config.Close()

Write-Output "Installed WpsAiBridge."
Write-Output "  DLL:      $dllPath"
Write-Output "  ProgID:   $progId"
Write-Output "  WPS:      Word $port / Spreadsheets $($port + 1) / Presentation $($port + 2)"
Write-Output "  Office:   Word $portOffice / Excel $($portOffice + 1) / PowerPoint $($portOffice + 2)"
Write-Output "  Config:   HKCU\Software\WpsAiBridge (Port, PortOffice, Token, Enabled)"
if ($tokenGenerated) {
    Write-Output "  Token:    auto-generated (32 hex) - requests to /cmd and /config must send X-Auth-Token"
    Write-Output "            (python tools/excel-mcp reads it from the registry automatically)"
} else {
    Write-Output "  Token:    existing value kept"
}
Write-Output ""
Write-Output "Next: open WPS or Microsoft Office (Word/Excel/PowerPoint)."
Write-Output "Companion: WpsAiBridge.Host.exe wps|et|wpp for WPS, word|excel|ppt for Microsoft Office."
Write-Output "If the add-in does not load, open Tools tab -> COM Add-ins and enable 'WPS AI Bridge'."

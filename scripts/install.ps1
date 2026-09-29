$ErrorActionPreference = "Stop"

$progId = "AxiomOffice.Connect"
$classGuid = "{BDB3732A-A479-4A24-AD64-D35952035BBA}"

$root = Split-Path -Parent $PSScriptRoot
$dllPath = Join-Path $root "src\AxiomOffice\bin\Release\AxiomOffice.dll"

if (-not (Test-Path -LiteralPath $dllPath)) {
    throw "DLL not found: $dllPath - run scripts\build.ps1 first"
}
$dllVersion = [System.Reflection.AssemblyName]::GetAssemblyName($dllPath).Version.ToString()

# File giai nen tu zip tai qua mang mang Zone.Identifier: .NET tu choi nap DLL do. Go danh dau truoc.
$binDir = Split-Path -Parent $dllPath
Get-ChildItem -LiteralPath $binDir -File | Unblock-File -ErrorAction SilentlyContinue
$hostExe = Join-Path $binDir "AxiomOffice.Host.exe"

$codeBase = "file:///" + ($dllPath -replace '\\', '/')
$currentUser = [Microsoft.Win32.Registry]::CurrentUser

# Du an truoc day ten WpsAiBridge: go dang ky cu (tranh nap add-in 2 lan) va chuyen cau hinh sang khoa moi.
. (Join-Path $PSScriptRoot "legacy.ps1")
$legacyRemoved = Remove-LegacyRegistration
$legacyConfig = Move-LegacyConfig "Software\AxiomOffice"

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
    $versioned.SetValue("Assembly", "AxiomOffice, Version=$dllVersion, Culture=neutral, PublicKeyToken=null", [Microsoft.Win32.RegistryValueKind]::String)
    $versioned.SetValue("Class", $className, [Microsoft.Win32.RegistryValueKind]::String)
    $versioned.SetValue("RuntimeVersion", "v4.0.30319", [Microsoft.Win32.RegistryValueKind]::String)
    $versioned.SetValue("CodeBase", $codeBase, [Microsoft.Win32.RegistryValueKind]::String)
    $versioned.Close()
    $inproc.Close()
    $clsidKey.Close()
}

Register-ComClass $progId $classGuid $progId
Register-ComClass "AxiomOffice.AskAiPane" "{8001B0D7-F189-443A-B3CB-6EB98038C72E}" "AxiomOffice.Ai.AskAiPane"

foreach ($officeApp in @("Word", "Excel", "PowerPoint")) {
    $key = New-Key "Software\Microsoft\Office\$officeApp\Addins\$progId"
    $key.SetValue("FriendlyName", "Axiom Office", [Microsoft.Win32.RegistryValueKind]::String)
    $key.SetValue("Description", "Axiom Office - AI agent that reads and edits the open document (Microsoft Office and WPS Office)", [Microsoft.Win32.RegistryValueKind]::String)
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

$config = New-Key "Software\AxiomOffice"
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

Write-Output "Installed Axiom Office."
if ($legacyRemoved -gt 0 -or $legacyConfig -ne "none") {
    Write-Output "  Migrated: removed $legacyRemoved old WPS AI Bridge (WpsAiBridge) registration entries; config: $legacyConfig"
    if (Test-Path -LiteralPath $LegacyDataDir) {
        Write-Output "            old log folder $LegacyDataDir is no longer used and can be deleted"
    }
}
Write-Output "  DLL:      $dllPath"
Write-Output "  ProgID:   $progId"
Write-Output "  WPS:      Word $port / Spreadsheets $($port + 1) / Presentation $($port + 2)"
Write-Output "  Office:   Word $portOffice / Excel $($portOffice + 1) / PowerPoint $($portOffice + 2)"
Write-Output "  Config:   HKCU\Software\AxiomOffice (Port, PortOffice, Token, Enabled)"
if ($tokenGenerated) {
    Write-Output "  Token:    auto-generated (32 hex) - requests to /cmd and /config must send X-Auth-Token"
    Write-Output "            (AxiomOffice.Host.exe mcp and tools/*-mcp read it from the registry automatically)"
} else {
    Write-Output "  Token:    existing value kept"
}
Write-Output ""
Write-Output "Next: open WPS or Microsoft Office (Word/Excel/PowerPoint)."
Write-Output "Companion: AxiomOffice.Host.exe wps|et|wpp for WPS, word|excel|ppt for Microsoft Office."
if (Test-Path -LiteralPath $hostExe) {
    $mcpJson = @{ command = $hostExe; args = @("mcp") } | ConvertTo-Json -Compress
    Write-Output "MCP server (AI agent): add to your MCP client config, e.g."
    Write-Output "  ""office"": $mcpJson"
}
Write-Output "If the add-in does not load, open Tools tab -> COM Add-ins and enable 'Axiom Office'."

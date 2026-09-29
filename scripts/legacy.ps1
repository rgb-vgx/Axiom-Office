# Don ban cai cu ten WpsAiBridge ("WPS AI Bridge") - du an da doi ten thanh Axiom Office.
# Dot-source tu install.ps1 / uninstall.ps1. Neu de lai ProgID/CLSID cu, Office/WPS se nap
# add-in hai lan (hai bridge tranh cung port), nen install.ps1 luon go dang ky cu truoc.

$LegacyConfigKey = "Software\WpsAiBridge"
$LegacyDataDir = Join-Path $env:LOCALAPPDATA "WpsAiBridge"
$LegacyProgIds = @("WpsAiBridge.Connect", "WpsAiBridge.AskAiPane", "WpsAiBridge.Probe")
$LegacyClsids = @(
    "{F4524DFD-C4F6-4027-8CA6-08B7F7DB4C44}",   # WpsAiBridge.Connect
    "{D99F8693-4316-45AF-8916-B70D87DEEF87}",   # WpsAiBridge.AskAiPane
    "{30F7166A-059E-484C-90C6-E1862DA76F50}"    # WpsAiBridge.Probe
)

function Remove-RegistryTreeIfExists([string]$path) {
    $hkcu = [Microsoft.Win32.Registry]::CurrentUser
    $key = $hkcu.OpenSubKey($path)
    if ($key -eq $null) {
        return 0
    }
    $key.Close()
    $hkcu.DeleteSubKeyTree($path, $false)
    return 1
}

# Go dang ky COM, Office Addins va whitelist WPS cua ten cu. Tra ve so muc da xoa.
function Remove-LegacyRegistration {
    $hkcu = [Microsoft.Win32.Registry]::CurrentUser
    $removed = 0
    foreach ($id in $LegacyProgIds) {
        $removed += Remove-RegistryTreeIfExists "Software\Classes\$id"
    }
    foreach ($clsid in $LegacyClsids) {
        $removed += Remove-RegistryTreeIfExists "Software\Classes\CLSID\$clsid"
    }
    foreach ($officeApp in @("Word", "Excel", "PowerPoint")) {
        $removed += Remove-RegistryTreeIfExists "Software\Microsoft\Office\$officeApp\Addins\WpsAiBridge.Connect"
    }
    foreach ($hive in @("WPS", "ET", "WPP")) {
        foreach ($list in @("AddinsWL", "AddinsCL")) {
            $key = $hkcu.OpenSubKey("Software\Kingsoft\Office\$hive\$list", $true)
            if ($key -ne $null) {
                if ($key.GetValueNames() -contains "WpsAiBridge.Connect") {
                    $key.DeleteValue("WpsAiBridge.Connect", $false)
                    $removed++
                }
                $key.Close()
            }
        }
    }
    return $removed
}

# Chuyen cau hinh (port, token, cai dat AI) tu khoa cu sang khoa moi. API key ma hoa DPAPI
# khong dung entropy nen chep nguyen gia tri van giai ma duoc. Khoa cu chi bi xoa sau khi
# doc lai khoa moi thay khop tung gia tri. Tra ve "migrated" | "kept-new" | "none".
function Move-LegacyConfig([string]$newKeyPath) {
    $hkcu = [Microsoft.Win32.Registry]::CurrentUser
    $options = [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames
    $old = $hkcu.OpenSubKey($LegacyConfigKey)
    if ($old -eq $null) {
        return "none"
    }
    $existing = $hkcu.OpenSubKey($newKeyPath)
    if ($existing -ne $null) {
        # Da co cau hinh moi (da chuyen tu lan cai truoc): khoa cu khong con dung.
        $existing.Close()
        $old.Close()
        $hkcu.DeleteSubKeyTree($LegacyConfigKey, $false)
        return "kept-new"
    }
    $new = $hkcu.CreateSubKey($newKeyPath)
    $names = $old.GetValueNames()
    foreach ($name in $names) {
        $new.SetValue($name, $old.GetValue($name, $null, $options), $old.GetValueKind($name))
    }
    $mismatch = @()
    foreach ($name in $names) {
        $a = $old.GetValue($name, $null, $options)
        $b = $new.GetValue($name, $null, $options)
        if ($a -is [byte[]]) {
            $same = ($b -is [byte[]]) -and (($a -join ",") -eq ($b -join ","))
        } else {
            $same = [string]$a -ceq [string]$b
        }
        if (-not $same) {
            $mismatch += $name
        }
    }
    $old.Close()
    $new.Close()
    if ($mismatch.Count -gt 0) {
        throw "Chuyen cau hinh tu HKCU\$LegacyConfigKey that bai (lech: $($mismatch -join ', ')); khoa cu duoc giu nguyen."
    }
    $hkcu.DeleteSubKeyTree($LegacyConfigKey, $false)
    return "migrated"
}

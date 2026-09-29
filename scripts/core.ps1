# Tien ich cho Agent Core (AxiomOffice.Core.exe) - dot-source tu build.ps1 / uninstall.ps1.
# Core ghi %LOCALAPPDATA%\AxiomOffice\core.json khi san sang (New_arch.md muc 7.1); tat Core bang
# API shutdown truoc khi ghi de/go file. Token doc tu HKCU va chi gui trong header, khong in ra.

function Get-AgentCoreInfo {
    $coreJson = Join-Path $env:LOCALAPPDATA "AxiomOffice\core.json"
    if (-not (Test-Path -LiteralPath $coreJson)) {
        return $null
    }
    try {
        return (Get-Content -LiteralPath $coreJson -Raw -Encoding UTF8 | ConvertFrom-Json)
    } catch {
        return $null
    }
}

# Tra ve $true neu da yeu cau Core tat (hoac Core khong chay).
function Stop-AgentCore {
    $info = Get-AgentCoreInfo
    if ($null -eq $info) {
        return $true
    }
    $headers = @{}
    $token = (Get-ItemProperty -Path "HKCU:\Software\AxiomOffice" -Name Token -ErrorAction SilentlyContinue).Token
    if ($token) {
        $headers["X-Auth-Token"] = $token
    }
    try {
        Invoke-RestMethod -Method Post -Uri ("http://127.0.0.1:{0}/v1/admin/shutdown" -f $info.port) -Headers $headers -TimeoutSec 3 | Out-Null
        Start-Sleep -Milliseconds 800
        Write-Output "Stopped Agent Core (pid $($info.pid))."
        return $true
    } catch {
        Write-Output "Agent Core (pid $($info.pid)) did not answer the shutdown request; it may already be stopped."
        return $false
    }
}

# Tim dotnet.exe: DOTNET_ROOT -> %LOCALAPPDATA%\Microsoft\dotnet (cai khong can admin) -> PATH.
function Find-Dotnet {
    $candidates = @($env:DOTNET_ROOT, (Join-Path $env:LOCALAPPDATA "Microsoft\dotnet"), "C:\Program Files\dotnet")
    foreach ($candidate in $candidates) {
        if ($candidate) {
            $exe = Join-Path $candidate "dotnet.exe"
            if (Test-Path -LiteralPath $exe) {
                return $exe
            }
        }
    }
    $command = Get-Command dotnet -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }
    return $null
}

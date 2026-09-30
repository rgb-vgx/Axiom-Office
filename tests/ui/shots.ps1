# Render wizard thiet lap (add-in net48) ra anh tung buoc - test thi giac, khong can Office/WPS.
#
#   powershell -ExecutionPolicy Bypass -File tests\ui\shots.ps1
#   powershell -ExecutionPolicy Bypass -File tests\ui\shots.ps1 -OutDir C:\tmp\wizard
#
# Bien dich harness CHUNG voi ma nguon add-in (nhu scripts\build.ps1) vao thu muc tam, nen khong dung toi
# bin\Release va khong anh huong app dang mo cua nguoi dung.
param(
    [string]$OutDir = ""
)
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)      # ...\wps-ai-bridge
$srcDir = Join-Path $root "src\AxiomOffice"
if (-not (Test-Path -LiteralPath $srcDir)) { throw "khong thay $srcDir" }
if (-not $OutDir) { $OutDir = Join-Path $PSScriptRoot "out" }

$stage = Join-Path $env:TEMP "axiom-ui-shots"
Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $stage, $OutDir | Out-Null

$fw = "C:\Windows\Microsoft.NET\Framework64\v4.0.30319"
$csc = "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\MSBuild\Current\Bin\Roslyn\csc.exe"
if (-not (Test-Path -LiteralPath $csc)) { $csc = "$fw\csc.exe" }

$sources = @(Get-ChildItem -Path $srcDir -Recurse -Filter *.cs |
    Where-Object { $_.FullName -notlike "*\bin\*" -and $_.FullName -notlike "*\obj\*" } |
    ForEach-Object { $_.FullName })
$sources += (Join-Path $PSScriptRoot "SetupWizardShots.cs")

$exe = Join-Path $stage "setup-wizard-shots.exe"
& $csc /nologo /platform:x64 /langversion:7.3 /codepage:65001 /target:exe "/out:$exe" `
    "/r:$fw\mscorlib.dll" "/r:$fw\System.dll" "/r:$fw\System.Core.dll" `
    "/r:$fw\System.Web.Extensions.dll" "/r:$fw\System.Windows.Forms.dll" `
    "/r:$fw\System.Drawing.dll" "/r:$fw\System.Security.dll" "/r:$fw\Microsoft.CSharp.dll" `
    $sources
if ($LASTEXITCODE -ne 0) { throw "Build harness failed" }

# Chay tu thu muc Release de moi duong dan tuong doi (neu co) tro dung cho add-in.
Push-Location (Join-Path $srcDir "bin\Release")
try {
    & $exe $OutDir
    if ($LASTEXITCODE -ne 0) { throw "Harness exit $LASTEXITCODE" }
} finally {
    Pop-Location
}

Write-Host "Anh wizard: $OutDir"

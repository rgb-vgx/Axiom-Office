$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$srcDir = Join-Path $root "src\WpsAiBridge"
$hostDir = Join-Path $root "src\WpsAiBridge.Host"
$out = Join-Path $srcDir "bin\Release"
$fw = "C:\Windows\Microsoft.NET\Framework64\v4.0.30319"
$csc = "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\MSBuild\Current\Bin\Roslyn\csc.exe"

if (-not (Test-Path -LiteralPath $csc)) {
    $csc = "$fw\csc.exe"
    if (-not (Test-Path -LiteralPath $csc)) {
        throw "No C# compiler found. Install Visual Studio Build Tools or .NET Framework."
    }
}

New-Item -ItemType Directory -Path $out -Force | Out-Null

$addinSources = @(Get-ChildItem -Path $srcDir -Recurse -Filter *.cs | ForEach-Object { $_.FullName })
$hostSources = @($addinSources + @(Get-ChildItem -Path $hostDir -Recurse -Filter *.cs | ForEach-Object { $_.FullName }))

$commonArgs = @(
    "/nologo", "/platform:x64", "/optimize+", "/langversion:7.3",
    "/r:$fw\mscorlib.dll",
    "/r:$fw\System.dll",
    "/r:$fw\System.Core.dll",
    "/r:$fw\System.Web.Extensions.dll",
    "/r:$fw\System.Windows.Forms.dll",
    "/r:$fw\System.Drawing.dll",
    "/r:$fw\Microsoft.CSharp.dll"
)

$dll = Join-Path $out "WpsAiBridge.dll"
& $csc @commonArgs /target:library "/out:$dll" $addinSources
if ($LASTEXITCODE -ne 0) { throw "Add-in build failed with exit code $LASTEXITCODE" }
Write-Output "Built add-in: $dll"

$exe = Join-Path $out "WpsAiBridge.Host.exe"
& $csc @commonArgs /target:exe "/out:$exe" $hostSources
if ($LASTEXITCODE -ne 0) { throw "Host build failed with exit code $LASTEXITCODE" }
Write-Output "Built host: $exe"

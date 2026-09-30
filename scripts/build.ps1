param(
    # Tu tat cac tien trinh dang lock AxiomOffice.dll/.Host.exe (Word/Excel/PowerPoint/WPS/companion) thay vi dung build.
    [switch]$Kill,
    # Agent Core nao: "go" (mac dinh, binary ~11MB khong can runtime - core-go/README.md) hoac "dotnet"
    # (ban .NET 10 self-contained ~48MB) khi can doi chieu.
    [ValidateSet("go", "dotnet")][string]$Core = "go"
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$srcDir = Join-Path $root "src\AxiomOffice"
$hostDir = Join-Path $root "src\AxiomOffice.Host"
$coreDir = Join-Path $root "src\AxiomOffice.Core"
$out = Join-Path $srcDir "bin\Release"
$stage = Join-Path $out ".stage"
$coreExe = Join-Path $out "AxiomOffice.Core.exe"
$skillsDir = Join-Path $root "skills"
$fw = "C:\Windows\Microsoft.NET\Framework64\v4.0.30319"
$csc = "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\MSBuild\Current\Bin\Roslyn\csc.exe"

. (Join-Path $PSScriptRoot "core.ps1")

if (-not (Test-Path -LiteralPath $csc)) {
    $csc = "$fw\csc.exe"
    if (-not (Test-Path -LiteralPath $csc)) {
        throw "No C# compiler found. Install Visual Studio Build Tools or .NET Framework."
    }
}

# Build khi host con giu DLL tung lam DLL bien mat (csc doi ten file dang bi lock roi ghi hong).
# Hoi Windows Restart Manager xem chinh xac tien trinh nao dang lock DLL/EXE dau ra
# (assembly .NET khong hien trong Process.Modules nen khong do theo module duoc).
if (-not ("FileLockers" -as [type])) {
Add-Type @"
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
public static class FileLockers {
    [StructLayout(LayoutKind.Sequential)]
    struct RM_UNIQUE_PROCESS { public int dwProcessId; public System.Runtime.InteropServices.ComTypes.FILETIME ProcessStartTime; }
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    struct RM_PROCESS_INFO {
        public RM_UNIQUE_PROCESS Process;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 256)] public string strAppName;
        [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 64)] public string strServiceShortName;
        public int ApplicationType; public uint AppStatus; public uint TSSessionId;
        [MarshalAs(UnmanagedType.Bool)] public bool bRestartable;
    }
    [DllImport("rstrtmgr.dll", CharSet = CharSet.Unicode)] static extern int RmStartSession(out uint handle, int flags, string key);
    [DllImport("rstrtmgr.dll")] static extern int RmEndSession(uint handle);
    [DllImport("rstrtmgr.dll", CharSet = CharSet.Unicode)] static extern int RmRegisterResources(uint handle, uint nFiles, string[] files, uint nApps, RM_UNIQUE_PROCESS[] apps, uint nServices, string[] services);
    [DllImport("rstrtmgr.dll")] static extern int RmGetList(uint handle, out uint needed, ref uint count, [In, Out] RM_PROCESS_INFO[] infos, ref uint reasons);
    public static int[] Find(string[] paths) {
        uint handle; var result = new List<int>();
        if (RmStartSession(out handle, 0, Guid.NewGuid().ToString()) != 0) return result.ToArray();
        try {
            if (RmRegisterResources(handle, (uint)paths.Length, paths, 0, null, 0, null) != 0) return result.ToArray();
            uint needed = 0, count = 0, reasons = 0;
            int rc = RmGetList(handle, out needed, ref count, null, ref reasons);
            if (rc == 234 && needed > 0) {
                var infos = new RM_PROCESS_INFO[needed]; count = needed;
                if (RmGetList(handle, out needed, ref count, infos, ref reasons) == 0)
                    for (int i = 0; i < count; i++) result.Add(infos[i].Process.dwProcessId);
            }
        } finally { RmEndSession(handle); }
        return result.ToArray();
    }
}
"@
}
# Agent Core dang chay giu AxiomOffice.Core.exe: tat em truoc khi ghi de (New_arch.md muc 10).
# Core la tien trinh cua chinh du an nen tat bang API shutdown, khong can -Kill (khac Word/Excel cua nguoi dung).
[void](Stop-AgentCore)

$outputs = @("AxiomOffice.dll", "AxiomOffice.Host.exe", "AxiomOffice.Core.exe") | ForEach-Object { Join-Path $out $_ } | Where-Object { Test-Path -LiteralPath $_ }
$lockers = @()
if ($outputs.Count -gt 0) {
    $lockers = @([FileLockers]::Find([string[]]$outputs) | Sort-Object -Unique | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue })
}
if ($lockers.Count -gt 0) {
    $list = ($lockers | ForEach-Object { "$($_.ProcessName) (pid $($_.Id))" }) -join ", "
    if (-not $Kill) {
        Write-Host "AxiomOffice.dll dang bi nap boi: $list" -ForegroundColor Red
        Write-Host "Dong cac app nay (luu tai lieu truoc) roi build lai, hoac chay: scripts\build.ps1 -Kill" -ForegroundColor Red
        exit 1
    }
    Write-Host "Tat: $list" -ForegroundColor Yellow
    $lockers | Stop-Process -Force
    $lockers | ForEach-Object { $_.WaitForExit(15000) | Out-Null }
}

New-Item -ItemType Directory -Path $out -Force | Out-Null
if (Test-Path -LiteralPath $stage) {
    Remove-Item -LiteralPath $stage -Recurse -Force
}
New-Item -ItemType Directory -Path $stage -Force | Out-Null

# Don ban cu ma csc da doi ten khi file dich bi lock (<guid>_AxiomOffice.dll); file con bi giu thi bo qua.
Get-ChildItem -LiteralPath $out -Filter "*_AxiomOffice.dll" -ErrorAction SilentlyContinue |
    ForEach-Object { Remove-Item -LiteralPath $_.FullName -Force -ErrorAction SilentlyContinue }

$addinSources = @(Get-ChildItem -Path $srcDir -Recurse -Filter *.cs | Where-Object { $_.FullName -notlike "*\bin\*" -and $_.FullName -notlike "*\obj\*" } | ForEach-Object { $_.FullName })
$hostSources = @($addinSources + @(Get-ChildItem -Path $hostDir -Recurse -Filter *.cs | ForEach-Object { $_.FullName }))

$commonArgs = @(
    "/nologo", "/platform:x64", "/optimize+", "/langversion:7.3", "/codepage:65001",
    "/r:$fw\mscorlib.dll",
    "/r:$fw\System.dll",
    "/r:$fw\System.Core.dll",
    "/r:$fw\System.Web.Extensions.dll",
    "/r:$fw\System.Windows.Forms.dll",
    "/r:$fw\System.Drawing.dll",
    "/r:$fw\System.Security.dll",
    "/r:$fw\Microsoft.CSharp.dll"
)

# Build vao .stage roi moi chep de: loi bien dich hay file dich bi lock deu giu nguyen DLL cu.
$stagedDll = Join-Path $stage "AxiomOffice.dll"
& $csc @commonArgs /target:library "/out:$stagedDll" $addinSources
if ($LASTEXITCODE -ne 0) { throw "Add-in build failed with exit code $LASTEXITCODE" }

# Companion + MCP server: them thu vien zip/XLinq (doc/ghi OOXML) va template docx/pptx nhung trong exe.
$templates = Join-Path $hostDir "Mcp\Templates"
$hostArgs = @(
    "/r:$fw\System.IO.Compression.dll",
    "/r:$fw\System.IO.Compression.FileSystem.dll",
    "/r:$fw\System.Xml.dll",
    "/r:$fw\System.Xml.Linq.dll",
    "/resource:$(Join-Path $templates 'default.docx'),AxiomOffice.Mcp.default.docx",
    "/resource:$(Join-Path $templates 'default.pptx'),AxiomOffice.Mcp.default.pptx"
)
$stagedExe = Join-Path $stage "AxiomOffice.Host.exe"
& $csc @commonArgs @hostArgs /target:exe "/out:$stagedExe" $hostSources
if ($LASTEXITCODE -ne 0) { throw "Host build failed with exit code $LASTEXITCODE" }

# Agent Core: version lay tu DLL add-in vua build de /health cua Core khop version bridge (New_arch.md muc 10).
$coreVersion = [System.Reflection.AssemblyName]::GetAssemblyName($stagedDll).Version.ToString(3)

# Mac dinh: ban Go (core-go/) - mot binary, khong can .NET runtime tren may nguoi dung.
if ($Core -eq "go") {
    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($null -eq $go) {
        Write-Host "Khong tim thay go: bo qua Agent Core. Cai Go 1.26+ (hoac chay build.ps1 -Core dotnet de dung ban .NET)." -ForegroundColor Yellow
    } else {
        # -C: chay go trong core-go/ du build.ps1 duoc goi tu dau.
        $goDir = Join-Path $root "core-go"
        & $go.Source build -C $goDir -trimpath -ldflags "-s -w -X main.version=$coreVersion" -o $coreExe ./cmd/axiom-core
        if ($LASTEXITCODE -ne 0) { throw "Agent Core (Go) build failed with exit code $LASTEXITCODE" }
        Write-Output "Built Agent Core (Go): $coreExe (version $coreVersion)"
    }
} elseif ($Core -eq "dotnet") {
$dotnet = Find-Dotnet
if ($null -eq $dotnet) {
    Write-Host "Khong tim thay dotnet: bo qua Agent Core (AxiomOffice.Core.exe). Chay scripts\install-dotnet-sdk.ps1 roi build lai." -ForegroundColor Yellow
} else {
    $coreStage = Join-Path $stage "core"
    # EnableCompressionInSingleFile: 103MB -> ~48MB (do that tren may nay).
    # IncludeNativeLibrariesForSelfExtract: SQLite can e_sqlite3.dll native - phai nhung vao exe, neu
    # khong thi ban publish chi co AxiomOffice.Core.exe se loi "SqliteConnection type initializer".
    & $dotnet publish $coreDir -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -p:EnableCompressionInSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true -p:Version=$coreVersion -o $coreStage --nologo
    if ($LASTEXITCODE -ne 0) { throw "Agent Core publish failed with exit code $LASTEXITCODE" }
    $builtCore = Join-Path $coreStage "AxiomOffice.Core.exe"
    if (-not (Test-Path -LiteralPath $builtCore)) { throw "Agent Core publish did not produce $builtCore" }
    Copy-Item -LiteralPath $builtCore -Destination $coreExe -Force
    Write-Output "Built Agent Core (.NET): $coreExe (version $coreVersion)"
}
}

foreach ($file in @($stagedDll, $stagedExe)) {
    $target = Join-Path $out (Split-Path -Leaf $file)
    try {
        Copy-Item -LiteralPath $file -Destination $target -Force
    }
    catch {
        throw "Khong ghi duoc $target (dang bi lock?). DLL cu van con nguyen. Chi tiet: $($_.Exception.Message)"
    }
}
Remove-Item -LiteralPath $stage -Recurse -Force

# Skill dung san (New_arch.md muc 8.4): chep canh exe de Core doc duoc luc chay.
if (Test-Path -LiteralPath $skillsDir) {
    Copy-Item -LiteralPath $skillsDir -Destination (Join-Path $out "skills") -Recurse -Force
    Write-Output "Copied skills: $(Join-Path $out 'skills')"
}

Write-Output "Built add-in: $(Join-Path $out 'AxiomOffice.dll')"
Write-Output "Built host: $(Join-Path $out 'AxiomOffice.Host.exe')"

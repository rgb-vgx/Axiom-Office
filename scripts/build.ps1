param(
    # Tu tat cac tien trinh dang lock WpsAiBridge.dll/.Host.exe (Word/Excel/PowerPoint/WPS/companion) thay vi dung build.
    [switch]$Kill
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$srcDir = Join-Path $root "src\WpsAiBridge"
$hostDir = Join-Path $root "src\WpsAiBridge.Host"
$out = Join-Path $srcDir "bin\Release"
$stage = Join-Path $out ".stage"
$fw = "C:\Windows\Microsoft.NET\Framework64\v4.0.30319"
$csc = "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\MSBuild\Current\Bin\Roslyn\csc.exe"

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
$outputs = @("WpsAiBridge.dll", "WpsAiBridge.Host.exe") | ForEach-Object { Join-Path $out $_ } | Where-Object { Test-Path -LiteralPath $_ }
$lockers = @()
if ($outputs.Count -gt 0) {
    $lockers = @([FileLockers]::Find([string[]]$outputs) | Sort-Object -Unique | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue })
}
if ($lockers.Count -gt 0) {
    $list = ($lockers | ForEach-Object { "$($_.ProcessName) (pid $($_.Id))" }) -join ", "
    if (-not $Kill) {
        Write-Host "WpsAiBridge.dll dang bi nap boi: $list" -ForegroundColor Red
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

# Don ban cu ma csc da doi ten khi file dich bi lock (<guid>_WpsAiBridge.dll); file con bi giu thi bo qua.
Get-ChildItem -LiteralPath $out -Filter "*_WpsAiBridge.dll" -ErrorAction SilentlyContinue |
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
$stagedDll = Join-Path $stage "WpsAiBridge.dll"
& $csc @commonArgs /target:library "/out:$stagedDll" $addinSources
if ($LASTEXITCODE -ne 0) { throw "Add-in build failed with exit code $LASTEXITCODE" }

# Companion + MCP server: them thu vien zip/XLinq (doc/ghi OOXML) va template docx/pptx nhung trong exe.
$templates = Join-Path $hostDir "Mcp\Templates"
$hostArgs = @(
    "/r:$fw\System.IO.Compression.dll",
    "/r:$fw\System.IO.Compression.FileSystem.dll",
    "/r:$fw\System.Xml.dll",
    "/r:$fw\System.Xml.Linq.dll",
    "/resource:$(Join-Path $templates 'default.docx'),WpsAiBridge.Mcp.default.docx",
    "/resource:$(Join-Path $templates 'default.pptx'),WpsAiBridge.Mcp.default.pptx"
)
$stagedExe = Join-Path $stage "WpsAiBridge.Host.exe"
& $csc @commonArgs @hostArgs /target:exe "/out:$stagedExe" $hostSources
if ($LASTEXITCODE -ne 0) { throw "Host build failed with exit code $LASTEXITCODE" }

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

Write-Output "Built add-in: $(Join-Path $out 'WpsAiBridge.dll')"
Write-Output "Built host: $(Join-Path $out 'WpsAiBridge.Host.exe')"

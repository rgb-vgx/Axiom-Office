$dll = 'C:\Users\ThuyetMT\Documents\work\wps-ai-bridge\src\AxiomOffice\bin\Release\AxiomOffice.dll'
$a = [Reflection.Assembly]::LoadFrom($dll)

# 1. McpForm: dựng được, đủ cột + nút
$t = $a.GetType('AxiomOffice.Ai.McpForm')
$f = [Activator]::CreateInstance($t, $true)
'form: ' + $f.Text
$lv = @($f.Controls | Where-Object { $_ -is [Windows.Forms.ListView] })[0]
'columns: ' + (($lv.Columns | ForEach-Object { $_.Text }) -join '|')
$panel = @($f.Controls | Where-Object { $_ -is [Windows.Forms.FlowLayoutPanel] })[0]
'buttons: ' + ((@($panel.Controls) | ForEach-Object { $_.Text }) -join '|')
$f.Dispose()

# 2. McpFile: round-trip trên file thật (backup + restore byte nguyên)
$mf = $a.GetType('AxiomOffice.Ai.McpFile')
$path = $mf.GetProperty('ConfigPath').GetValue($null, $null)
'path: ' + $path
$raw = [IO.File]::ReadAllBytes($path)
$load = $mf.GetMethod('Load')
$save = $mf.GetMethod('Save')
function Invoke-Out($method, $target, $fixedArgs) {
  $err = ''
  $args = New-Object object[] ($fixedArgs.Count + 1)
  for ($i = 0; $i -lt $fixedArgs.Count; $i++) { $args[$i] = $fixedArgs[$i] }
  $args[$fixedArgs.Count] = $err
  $result = $method.Invoke($target, $args)
  return @{ Result = $result; Error = [string]$args[$fixedArgs.Count] }
}
$r = Invoke-Out $load $null @()
'load: ' + $r.Result.Count + ' server, err=' + $r.Error
$servers = $r.Result
$newEntry = New-Object 'System.Collections.Generic.Dictionary[string,object]'
$newEntry['command'] = 'node'
$newEntry['args'] = @('x.js')
$newEntry['trusted'] = $true
$servers['__test_tmp__'] = $newEntry
$r = Invoke-Out $save $null @($servers)
'save: ' + $r.Result + ' err=' + $r.Error
$r = Invoke-Out $load $null @()
'roundtrip trusted=' + $r.Result['__test_tmp__']['trusted'] + ' args0=' + $r.Result['__test_tmp__']['args'][0]
$r.Result.Remove('__test_tmp__')
Invoke-Out $save $null @($r.Result) | Out-Null
[IO.File]::WriteAllBytes($path, $raw)
'restored: ' + (([IO.File]::ReadAllBytes($path).Length) -eq $raw.Length)

# 3. McpServers() qua Core đang chạy
$cc = $a.GetType('AxiomOffice.Ai.CoreClient')
$inst = $cc.GetProperty('Instance').GetValue($null, $null)
$m = $cc.GetMethod('McpServers')
$r = Invoke-Out $m $inst @()
$list = $r.Result
'names: ' + (($list | ForEach-Object { $_['name'] }) -join ',')
foreach ($s in $list) { '  ' + $s['name'] + ' trusted=' + $s['trusted'] + ' tools=' + @($s['tools']).Count + ' err=' + $s['error'] }
'ALL OK'

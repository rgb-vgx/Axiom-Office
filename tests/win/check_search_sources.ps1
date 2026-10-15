# Kiem tra nguon tim kiem web (menu "Tim web"): Items/SelectedIndex/Activate phai ghi dung
# HKCU\Software\AxiomOffice\TavilyEnabled + mcp.json (co "role": "web-search") va khoi phuc nguyen trang.
# Chay khong can Office: nap DLL qua reflection.  Cach goi:
#   powershell -STA -File tests\win\check_search_sources.ps1 [duong-dan-DLL]
# (DLL mac dinh: src\AxiomOffice\bin\Release\AxiomOffice.dll; DLL khong bi lock thi khong can dong Word.)
$dll = $args[0]
if (-not $dll) { $dll = 'C:\Users\ThuyetMT\Documents\work\wps-ai-bridge\src\AxiomOffice\bin\Release\AxiomOffice.dll' }
$a = [Reflection.Assembly]::LoadFrom($dll)
$bf = [Reflection.BindingFlags]'Static, Public, NonPublic, Instance'
$ss = $a.GetType('AxiomOffice.Ai.SearchSources')
$itemType = $a.GetType('AxiomOffice.Ai.SearchSources+Item')
if (-not $ss -or -not $itemType) { 'FAIL: khong thay AxiomOffice.Ai.SearchSources trong DLL'; exit 1 }

$mcpPath = Join-Path $env:LOCALAPPDATA 'AxiomOffice\mcp.json'
$backup = [IO.File]::ReadAllBytes($mcpPath)
$reg = 'HKCU:\Software\AxiomOffice'
$hadFlag = $null -ne (Get-ItemProperty -Path $reg -Name TavilyEnabled -ErrorAction SilentlyContinue)
$savedFlag = if ($hadFlag) { (Get-ItemProperty -Path $reg).TavilyEnabled } else { $null }
$fail = 0
function Check($label, $actual, $expected) {
  if ("$actual" -ceq "$expected") { "PASS $label" }
  else { $script:fail++; "FAIL $label (got [$actual] want [$expected])" }
}
function ItemIds { @($ss.GetMethod('Items', $bf).Invoke($null, @()) | ForEach-Object { $itemType.GetField('Id', $bf).GetValue($_) }) }
function SelectedIndex { $ss.GetMethod('SelectedIndex', $bf).Invoke($null, @()) }
function Invoke-Activate($index) {
  $callArgs = @($index, $false, '')
  $ok = $ss.GetMethod('Activate', $bf).Invoke($null, $callArgs)
  return @{ Ok = [bool]$ok; NeedsKey = [bool]$callArgs[1]; Error = [string]$callArgs[2] }
}
function ChromeSearchEntry {
  ([IO.File]::ReadAllText($mcpPath, [Text.Encoding]::UTF8) | ConvertFrom-Json).mcpServers.'chrome-search'
}
function TavilyFlag { (Get-ItemProperty -Path $reg -Name TavilyEnabled -ErrorAction SilentlyContinue).TavilyEnabled }

try {
  $ids = ItemIds
  Check 'items-count' $ids.Count 3
  Check 'item-off' $ids[0] 'off'
  Check 'item-tavily' $ids[1] 'tavily'
  Check 'item-mcp' $ids[2] 'mcp:chrome-search'
  Check 'selected-default-tavily' (SelectedIndex) 1

  # Chon chrome-search: Tavily tat, server bat.
  $r = Invoke-Activate 2
  Check 'activate-mcp-ok' $r.Ok 'True'
  Check 'activate-mcp-tavily-flag' (TavilyFlag) 0
  Check 'activate-mcp-server-enabled' ($null -eq (ChromeSearchEntry).disabled) 'True'
  Check 'activate-mcp-selected' (SelectedIndex) 2
  Check 'activate-mcp-trusted-kept' (ChromeSearchEntry).trusted 'True'

  # Tat het: Tavily tat, server tat.
  $r = Invoke-Activate 0
  Check 'activate-off-ok' $r.Ok 'True'
  Check 'activate-off-tavily-flag' (TavilyFlag) 0
  Check 'activate-off-server-disabled' (ChromeSearchEntry).disabled 'True'
  Check 'activate-off-selected' (SelectedIndex) 0

  # Chon Tavily: Tavily bat, server tat.
  $r = Invoke-Activate 1
  Check 'activate-tavily-ok' $r.Ok 'True'
  Check 'activate-tavily-flag' (TavilyFlag) 1
  Check 'activate-tavily-server-disabled' (ChromeSearchEntry).disabled 'True'
  Check 'activate-tavily-selected' (SelectedIndex) 1

  # Index ngoai danh sach: tu choi, khong ghi gi.
  $r = Invoke-Activate 9
  Check 'activate-bad-index' $r.Ok 'False'
} finally {
  [IO.File]::WriteAllBytes($mcpPath, $backup)
  if ($hadFlag) { Set-ItemProperty -Path $reg -Name TavilyEnabled -Value $savedFlag -Type DWord }
  else { Remove-ItemProperty -Path $reg -Name TavilyEnabled -ErrorAction SilentlyContinue }
}
if ($fail -eq 0) { 'SEARCH-SOURCES ALL OK' } else { 'SEARCH-SOURCES FAILURES: ' + $fail; exit 1 }

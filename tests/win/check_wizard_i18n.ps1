# Kiem tra wizard "Thiet lap" doi ngon ngu (khong can Office):
# nap DLL bien dich tam, doi sang English, dung form, doi lai giua chung de thu rebuild giu du lieu.
$dll = $args[0]
if (-not $dll) { $dll = 'C:\Users\ThuyetMT\Documents\work\wps-ai-bridge\src\AxiomOffice\bin\Release\AxiomOffice.dll' }
$a = [Reflection.Assembly]::LoadFrom($dll)
$bf = [Reflection.BindingFlags]'Static, Public, NonPublic, Instance'
$ui = $a.GetType('AxiomOffice.Ui')
$fail = 0
function Check($label, $actual, $expected) {
  if ($actual -ceq $expected) { 'PASS ' + $label }
  else { $script:fail++; 'FAIL ' + $label + ' (got [' + $actual + '] want [' + $expected + '])' }
}

$reg = 'HKCU:\Software\AxiomOffice'
$hadKey = (Get-ItemProperty -Path $reg -Name UiLanguage -ErrorAction SilentlyContinue) -ne $null
$saved = if ($hadKey) { (Get-ItemProperty -Path $reg).UiLanguage } else { $null }
try {
  $ui.GetMethod('Set').Invoke($null, @('vi')) | Out-Null
  $wt = $a.GetType('AxiomOffice.Ai.SetupWizardForm')
  $ctor = $wt.GetConstructors() | Select-Object -First 1
  $f = $ctor.Invoke(@($null))
  $get = { param($n) $f.GetType().GetField($n, $bf).GetValue($f) }
  # Tieng Viet ban dau
  Check 'vi-title' (& $get '_titleLabel').Text 'Chào mừng bạn đến với Axiom Office'
  # Go chu de thu rebuild
  (& $get '_endpoint').Text = 'http://may-chu-rieng:8080/v1'
  (& $get '_model').Text = 'model-rieng'
  (& $get '_tavilyKey').Text = 'tv-test'
  $ui.GetMethod('Set').Invoke($null, @('en')) | Out-Null
  Check 'lang' $ui.GetProperty('Code').GetValue($null, $null) 'en'
  Check 'en-form' $f.Text 'Axiom Office setup'
  Check 'en-title' (& $get '_titleLabel').Text 'Welcome to Axiom Office'
  Check 'en-step' (& $get '_stepLabel').Text 'step 1/5'
  Check 'en-next' (& $get '_next').Text 'Continue →'
  Check 'en-later' (& $get '_later').Text 'Later'
  Check 'keep-endpoint' (& $get '_endpoint').Text 'http://may-chu-rieng:8080/v1'
  Check 'keep-model' (& $get '_model').Text 'model-rieng'
  Check 'keep-tavily' (& $get '_tavilyKey').Text 'tv-test'
  Check 'keep-provider' (& $get '_providerId') 'company'
  $chips = & $get '_providerChips'
  Check 'en-chip0' $chips[0].Text '● Company server'
  Check 'en-subtitle' ((& $get '_subtitleLabel').Text.StartsWith('It takes about a minute')) 'True'
  $f.Dispose()
} finally {
  if ($hadKey) { $ui.GetMethod('Set').Invoke($null, @($saved)) | Out-Null }
  else { Remove-ItemProperty -Path $reg -Name UiLanguage -ErrorAction SilentlyContinue }
}
if ($fail -eq 0) { 'WIZARD-I18N ALL OK' } else { 'WIZARD-I18N FAILURES: ' + $fail; exit 1 }

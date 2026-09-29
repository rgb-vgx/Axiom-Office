@echo off
setlocal
echo Axiom Office - cai dat (khong can quyen admin)
echo Hay dong Word, Excel, PowerPoint va WPS truoc khi cai.
echo.
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-ChildItem -LiteralPath '%~dp0.' -Recurse -File | Unblock-File -ErrorAction SilentlyContinue; & '%~dp0scripts\install.ps1'; exit $LASTEXITCODE"
if errorlevel 1 (
    echo.
    echo CAI DAT THAT BAI - xem thong bao o tren.
) else (
    echo.
    echo Xong. Mo Word/Excel/PowerPoint hoac WPS: tren ribbon co tab "Axiom Office".
)
echo.
pause

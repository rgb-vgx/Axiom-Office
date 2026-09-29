@echo off
setlocal
echo WPS AI Bridge - go cai dat
echo Hay dong Word, Excel, PowerPoint va WPS truoc khi go.
echo (Them tham so -Purge de xoa ca cau hinh AI, token va log: uninstall.cmd -Purge)
echo.
powershell -NoProfile -ExecutionPolicy Bypass -Command "& '%~dp0scripts\uninstall.ps1' %*; exit $LASTEXITCODE"
echo.
pause

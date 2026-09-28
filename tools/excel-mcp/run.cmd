@echo off
cd /d "%~dp0"
".venv\Scripts\python.exe" -m excel_mcp.server %*

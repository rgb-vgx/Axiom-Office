@echo off
cd /d "%~dp0"
".venv\Scripts\python.exe" -m word_mcp.server %*

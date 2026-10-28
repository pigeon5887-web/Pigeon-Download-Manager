@echo off
chcp 65001 > nul
pushd %~dp0
"%~dp0jre\bin\java.exe" -Xmx512M -Dfile.encoding=utf-8 -jar "%~dp0netdisk-fast-download.jar" %*

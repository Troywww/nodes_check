param(
  [string]$Config = ".\configs\config.example.yaml"
)

$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
go build -o .\nodes-check.exe .\cmd\server
.\nodes-check.exe -config $Config

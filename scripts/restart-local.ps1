$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$frontendDir = Join-Path $root "frontend"
$backendDir = Join-Path $root "backend"
$logDir = Join-Path $root "logs"

New-Item -ItemType Directory -Force -Path $logDir | Out-Null

function Stop-PortProcess {
  param(
    [Parameter(Mandatory = $true)]
    [int]$Port
  )

  $connections = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue
  $processIds = $connections | Select-Object -ExpandProperty OwningProcess -Unique

  foreach ($processId in $processIds) {
    if (-not $processId -or $processId -eq 0) {
      continue
    }

    $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
    if ($process) {
      Write-Host "Stopping port $Port process: $($process.ProcessName) ($processId)"
      Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
    }
  }
}

Write-Host "Restarting local project..."
Stop-PortProcess -Port 5173
Stop-PortProcess -Port 8080

$backendLog = Join-Path $logDir "backend-local.log"
$frontendLog = Join-Path $logDir "frontend-local.log"
$backendCommand = "go run ./cmd/api *> '$backendLog'"
$frontendCommand = "npm.cmd run dev *> '$frontendLog'"

Write-Host "Starting backend: http://127.0.0.1:8080"
Start-Process -FilePath "powershell.exe" -WorkingDirectory $backendDir -WindowStyle Hidden -ArgumentList @(
  "-NoExit",
  "-ExecutionPolicy",
  "Bypass",
  "-Command",
  $backendCommand
)

Start-Sleep -Seconds 2

Write-Host "Starting frontend: http://127.0.0.1:5173"
Start-Process -FilePath "powershell.exe" -WorkingDirectory $frontendDir -WindowStyle Hidden -ArgumentList @(
  "-NoExit",
  "-ExecutionPolicy",
  "Bypass",
  "-Command",
  $frontendCommand
)

Write-Host ""
Write-Host "Local project started."
Write-Host "Frontend: http://127.0.0.1:5173/"
Write-Host "Backend:  http://127.0.0.1:8080/"
Write-Host "Frontend log: $frontendLog"
Write-Host "Backend log:  $backendLog"

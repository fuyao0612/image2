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
$backendHealthUrl = "http://127.0.0.1:8080/healthz"

function Wait-HttpOk {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Url,
    [int]$TimeoutSeconds = 60
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    try {
      $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 2
      if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
        return $true
      }
    } catch {
      Start-Sleep -Milliseconds 500
    }
  }

  return $false
}

Write-Host "Starting backend: http://127.0.0.1:8080"
Start-Process -FilePath "powershell.exe" -WorkingDirectory $backendDir -WindowStyle Hidden -ArgumentList @(
  "-NoExit",
  "-ExecutionPolicy",
  "Bypass",
  "-Command",
  $backendCommand
)

Write-Host "Waiting for backend health check: $backendHealthUrl"
if (-not (Wait-HttpOk -Url $backendHealthUrl -TimeoutSeconds 60)) {
  Write-Host ""
  Write-Host "Backend did not become ready within 60 seconds."
  Write-Host "Backend log tail:"
  if (Test-Path $backendLog) {
    Get-Content $backendLog -Tail 80
  } else {
    Write-Host "Backend log file was not created: $backendLog"
  }
  exit 1
}

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

param(
  [string]$HostName = "jh612.top",
  [string]$UserName = "ubuntu",
  [string]$KeyPath = $env:IMAGE2_SSH_KEY,
  [string]$RemoteDir = "/home/ubuntu/image2",
  [switch]$Deploy,
  [switch]$SkipTests
)

$ErrorActionPreference = "Stop"

function Step([string]$Name) {
  Write-Host ""
  Write-Host "==> $Name" -ForegroundColor Cyan
}

function Require-Command([string]$Name) {
  if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
    throw "Missing command: $Name"
  }
}

function Remove-PathIfExists([string[]]$Paths) {
  foreach ($Path in $Paths) {
    Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction SilentlyContinue
  }
}

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$BackendDir = Join-Path $Root "backend"
$FrontendDir = Join-Path $Root "frontend"
$FrontendDist = Join-Path $FrontendDir "dist"
$BackendArtifact = Join-Path $BackendDir "api"
$FrontendArchive = Join-Path $Root "frontend-dist.tgz"
$RemoteBackendDir = "$RemoteDir/backend"
$RemoteFrontendArchive = "$RemoteDir/frontend-dist.tgz"

Step "Check local tools"
Require-Command "go"
Require-Command "npm.cmd"
Require-Command "tar"
if ($Deploy) {
  Require-Command "ssh"
  Require-Command "scp"
  if ([string]::IsNullOrWhiteSpace($KeyPath)) {
    throw "Set IMAGE2_SSH_KEY or pass -KeyPath before deploying."
  }
  if (-not (Test-Path -LiteralPath $KeyPath)) {
    throw "SSH private key not found: $KeyPath"
  }
}

Step "Clean old artifacts"
Remove-PathIfExists @(
  $BackendArtifact,
  (Join-Path $BackendDir "api.exe"),
  (Join-Path $BackendDir "api.exe~"),
  $FrontendArchive,
  $FrontendDist
)

if (-not $SkipTests) {
  Step "Run backend tests"
  Push-Location $BackendDir
  try {
    go test ./...
  } finally {
    Pop-Location
  }
}

Step "Build frontend"
Push-Location $FrontendDir
try {
  npm.cmd run build
} finally {
  Pop-Location
}

Step "Build Linux backend binary"
Push-Location $BackendDir
try {
  $env:CGO_ENABLED = "0"
  $env:GOOS = "linux"
  $env:GOARCH = "amd64"
  go build -o $BackendArtifact ./cmd/api
} finally {
  Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
  Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
  Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
  Pop-Location
}

Step "Package frontend"
Push-Location $Root
try {
  tar -czf $FrontendArchive -C "frontend\dist" .
} finally {
  Pop-Location
}

$BackendInfo = Get-Item -LiteralPath $BackendArtifact
$FrontendInfo = Get-Item -LiteralPath $FrontendArchive
Write-Host "Backend: $($BackendInfo.FullName) / $([math]::Round($BackendInfo.Length / 1MB, 2)) MB"
Write-Host "Frontend: $($FrontendInfo.FullName) / $([math]::Round($FrontendInfo.Length / 1KB, 2)) KB"

if (-not $Deploy) {
  Write-Host ""
  Write-Host "Artifacts are ready. Re-run with -Deploy to upload and replace container files." -ForegroundColor Green
  exit 0
}

Step "Upload artifacts"
$SshTarget = "$UserName@$HostName"
$SshArgs = @("-i", $KeyPath, $SshTarget)
$ScpArgs = @("-i", $KeyPath)

& ssh @SshArgs "mkdir -p '$RemoteBackendDir'"
& scp @ScpArgs $BackendArtifact "$SshTarget`:$RemoteBackendDir/api"
& scp @ScpArgs $FrontendArchive "$SshTarget`:$RemoteFrontendArchive"

Step "Replace container files"
$RemoteScript = @"
set -e
cd '$RemoteDir'
mkdir -p frontend-dist
rm -rf frontend-dist/*
tar -xzf frontend-dist.tgz -C frontend-dist

sudo docker cp backend/api deploy_backend_1:/app/api
sudo docker exec deploy_backend_1 sh -c 'chmod +x /app/api'
sudo docker restart deploy_backend_1

for i in `$(seq 1 40); do
  status=`$(sudo docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' deploy_backend_1 2>/dev/null || true)
  echo "backend=`$status"
  [ "`$status" = "healthy" ] && break
  [ "`$status" = "running" ] && break
  sleep 2
done

status=`$(sudo docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' deploy_backend_1 2>/dev/null || true)
if [ "`$status" != "healthy" ] && [ "`$status" != "running" ]; then
  sudo docker logs --tail=200 deploy_backend_1 || true
  exit 30
fi

sudo docker exec deploy_web_1 sh -c 'rm -rf /usr/share/nginx/html/*'
sudo docker cp frontend-dist/. deploy_web_1:/usr/share/nginx/html/
sudo docker exec deploy_web_1 nginx -s reload

sudo docker ps --format '{{.Names}} {{.Status}}'
curl -fsS http://127.0.0.1/healthz
printf '\n'
curl -fsSI http://127.0.0.1/app | sed -n '1,10p'
"@

& ssh @SshArgs $RemoteScript

Write-Host ""
Write-Host "Deploy completed: https://jh612.top/app" -ForegroundColor Green

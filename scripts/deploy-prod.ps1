param(
  [string]$HostName = "111.230.17.35",
  [string]$UserName = "ubuntu",
  [string]$RemoteDir = "/opt/image2",
  [string]$PublicUrl = "https://jh612.top",
  [switch]$KeepRegistrationOpen,
  [switch]$CloseRegistration,
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

function Set-EnvLine([string]$Path, [string]$Key, [string]$Value) {
  if (-not (Test-Path $Path)) {
    New-Item -ItemType File -Force -Path $Path | Out-Null
  }
  $lines = @(Get-Content $Path -ErrorAction SilentlyContinue)
  $lineValue = "$Key=$Value"
  $found = $false
  $next = foreach ($line in $lines) {
    if ($line -match "^$([regex]::Escape($Key))=") {
      $found = $true
      $lineValue
    } else {
      $line
    }
  }
  if (-not $found) {
    $next += $lineValue
  }
  Set-Content -Path $Path -Value $next -Encoding UTF8
}

function Get-EnvLine([string]$Path, [string]$Key) {
  if (-not (Test-Path $Path)) {
    return ""
  }
  foreach ($line in (Get-Content $Path -ErrorAction SilentlyContinue)) {
    $trimmed = $line.Trim()
    if ($trimmed -match "^$([regex]::Escape($Key))=") {
      return $trimmed.Substring($Key.Length + 1).Trim().Trim('"').Trim("'")
    }
  }
  return ""
}

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Artifacts = Join-Path $Root ".artifacts"
$PackageDir = Join-Path $Artifacts "deploy-package"
$ToolsDir = Join-Path $Artifacts "deploy-tools"
$PackagePath = Join-Path $Artifacts "deploy-package.tgz"
$RemotePackage = "/home/$UserName/deploy-package.tgz"
$RegistrationEnabled = if ($CloseRegistration) { "false" } else { "true" }

Step "Check local tools"
Require-Command "go"
Require-Command "npm.cmd"
Require-Command "node"
Require-Command "tar"

Step "Prepare SSH helper"
New-Item -ItemType Directory -Force -Path $ToolsDir | Out-Null
if (-not (Test-Path (Join-Path $ToolsDir "node_modules/ssh2"))) {
  Push-Location $ToolsDir
  try {
    if (-not (Test-Path "package.json")) {
      Set-Content -Path "package.json" -Value '{"private":true,"dependencies":{"ssh2":"^1.17.0"}}' -Encoding ASCII
    }
    npm.cmd install --omit=dev
  } finally {
    Pop-Location
  }
}

if (-not $env:DEPLOY_PASSWORD) {
  $secure = Read-Host "SSH password" -AsSecureString
  $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
  try {
    $env:DEPLOY_PASSWORD = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
  } finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
  }
}

if (-not $SkipTests) {
  Step "Run backend tests"
  Push-Location (Join-Path $Root "backend")
  try {
    go test ./...
  } finally {
    Pop-Location
  }
}

Step "Build frontend"
Push-Location (Join-Path $Root "frontend")
try {
  $env:VITE_API_BASE_URL = ""
  $env:VITE_REGISTRATION_ENABLED = $RegistrationEnabled
  npm.cmd run build
} finally {
  Pop-Location
}

Step "Build Linux backend binary"
Remove-Item -Recurse -Force $PackageDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "backend") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "frontend") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $PackageDir "deploy") | Out-Null

Push-Location (Join-Path $Root "backend")
try {
  $env:CGO_ENABLED = "0"
  $env:GOOS = "linux"
  $env:GOARCH = "amd64"
  go build -o (Join-Path $PackageDir "backend/api") ./cmd/api
} finally {
  Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
  Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
  Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
  Pop-Location
}

Step "Package deploy bundle"
Copy-Item -Recurse -Force (Join-Path $Root "frontend/dist") (Join-Path $PackageDir "frontend/dist")
Copy-Item -Force (Join-Path $Root "deploy/nginx.conf") (Join-Path $PackageDir "deploy/nginx.conf")
Copy-Item -Force (Join-Path $Root "deploy/backend.runtime.Dockerfile") (Join-Path $PackageDir "deploy/backend.runtime.Dockerfile")
Copy-Item -Force (Join-Path $Root "deploy/frontend.runtime.Dockerfile") (Join-Path $PackageDir "deploy/frontend.runtime.Dockerfile")
Copy-Item -Force (Join-Path $Root "deploy/docker-compose.runtime.yml") (Join-Path $PackageDir "deploy/docker-compose.runtime.yml")

$LocalEnv = Join-Path $Root "deploy/.env.production"
if (Test-Path $LocalEnv) {
  Set-EnvLine $LocalEnv "REGISTRATION_ENABLED" $RegistrationEnabled
  Set-EnvLine $LocalEnv "VITE_REGISTRATION_ENABLED" $RegistrationEnabled
  Set-EnvLine $LocalEnv "EMAIL_CODE_TTL" "5m"
  Set-EnvLine $LocalEnv "EMAIL_CODE_COOLDOWN" "1m"
  Set-EnvLine $LocalEnv "EMAIL_CODE_SEND_WINDOW" "1h"
  Set-EnvLine $LocalEnv "EMAIL_CODE_MAX_PER_WINDOW" "5"
  Set-EnvLine $LocalEnv "EMAIL_CODE_MAX_ATTEMPTS" "5"
  Set-EnvLine $LocalEnv "EMAIL_DELIVERY" "resend"
  Set-EnvLine $LocalEnv "RESEND_API_URL" "https://api.resend.com/emails"
  Set-EnvLine $LocalEnv "SMTP_FROM" "verify@jh612.top"
  Set-EnvLine $LocalEnv "SMTP_FROM_NAME" "jh612"
  Set-EnvLine $LocalEnv "BACKEND_PUBLIC_URL" $PublicUrl
  Set-EnvLine $LocalEnv "CORS_ALLOW_ORIGINS" "$PublicUrl,$PublicUrl`:80"
}
$LocalResendApiKey = Get-EnvLine $LocalEnv "RESEND_API_KEY"

Remove-Item -Force $PackagePath -ErrorAction SilentlyContinue
Push-Location $PackageDir
try {
  tar -czf $PackagePath backend frontend deploy
} finally {
  Pop-Location
}
$packageInfo = Get-Item $PackagePath
Write-Host "Package: $($packageInfo.FullName) / $([math]::Round($packageInfo.Length / 1MB, 2)) MB"

Step "Upload and deploy"
$RunnerPath = Join-Path $ToolsDir "deploy-prod-runner.cjs"
$Runner = @'
const fs = require('fs');
const path = require('path');
const { Client } = require('./node_modules/ssh2');

const cfg = {
  host: process.env.DEPLOY_HOST,
  username: process.env.DEPLOY_USER,
  password: process.env.DEPLOY_PASSWORD,
  packagePath: process.env.DEPLOY_PACKAGE,
  remotePackage: process.env.DEPLOY_REMOTE_PACKAGE,
  remoteDir: process.env.DEPLOY_REMOTE_DIR,
  publicUrl: process.env.DEPLOY_PUBLIC_URL,
  registrationEnabled: process.env.DEPLOY_REGISTRATION_ENABLED,
  resendApiKey: process.env.DEPLOY_RESEND_API_KEY || '',
};

function shQuote(value) {
  return `'${String(value).replace(/'/g, `'\"'\"'`)}'`;
}

function encodeScript(script) {
  return Buffer.from(script, 'utf8').toString('base64');
}

const conn = new Client();

function run(script) {
  return new Promise((resolve, reject) => {
    const b64 = encodeScript(script);
    const inner = `eval "$(printf %s ${shQuote(b64)} | base64 -d)"`;
    const command = `sudo -S -p "" bash -lc ${shQuote(inner)}`;
    conn.exec(command, (err, stream) => {
      if (err) return reject(err);
      let stdout = '';
      let stderr = '';
      stream.on('data', (chunk) => { stdout += chunk.toString(); });
      stream.stderr.on('data', (chunk) => { stderr += chunk.toString(); });
      stream.on('close', (code) => {
        if (stdout.trim()) process.stdout.write(stdout);
        if (stderr.trim()) process.stderr.write(stderr);
        if (code !== 0) reject(new Error(`remote command failed: ${code}`));
        else resolve();
      });
      stream.write(cfg.password + '\n');
      stream.end();
    });
  });
}

function upload() {
  return new Promise((resolve, reject) => {
    conn.sftp((err, sftp) => {
      if (err) return reject(err);
      sftp.fastPut(cfg.packagePath, cfg.remotePackage, (putErr) => {
        if (putErr) reject(putErr);
        else resolve();
      });
    });
  });
}

async function main() {
  if (!cfg.password) throw new Error('DEPLOY_PASSWORD is required');
  const sizeMb = (fs.statSync(cfg.packagePath).size / 1024 / 1024).toFixed(2);
  console.log(`Upload ${path.basename(cfg.packagePath)} (${sizeMb} MB) to ${cfg.host}...`);
  await upload();

  const corsOrigins = `${cfg.publicUrl},${cfg.publicUrl}:80`;
  const remoteScript = `
set -euo pipefail
mkdir -p ${shQuote(cfg.remoteDir)}
tar -xzf ${shQuote(cfg.remotePackage)} -C ${shQuote(cfg.remoteDir)}
cd ${shQuote(cfg.remoteDir)}

if [ ! -f deploy/.env.production ]; then
  echo "Missing deploy/.env.production on server."
  exit 20
fi

set_env() {
  key="$1"
  value="$2"
  if grep -q "^$key=" deploy/.env.production; then
    sed -i "s|^$key=.*|$key=$value|" deploy/.env.production
  else
    printf "\\n%s=%s\\n" "$key" "$value" >> deploy/.env.production
  fi
}

set_env REGISTRATION_ENABLED ${shQuote(cfg.registrationEnabled)}
set_env VITE_REGISTRATION_ENABLED ${shQuote(cfg.registrationEnabled)}
set_env EMAIL_CODE_TTL 5m
set_env EMAIL_CODE_COOLDOWN 1m
set_env EMAIL_CODE_SEND_WINDOW 1h
set_env EMAIL_CODE_MAX_PER_WINDOW 5
set_env EMAIL_CODE_MAX_ATTEMPTS 5
set_env EMAIL_DELIVERY resend
set_env RESEND_API_URL https://api.resend.com/emails
set_env SMTP_FROM verify@jh612.top
set_env SMTP_FROM_NAME jh612
if [ -n ${shQuote(cfg.resendApiKey)} ]; then
  set_env RESEND_API_KEY ${shQuote(cfg.resendApiKey)}
fi
set_env BACKEND_PUBLIC_URL ${shQuote(cfg.publicUrl)}
set_env CORS_ALLOW_ORIGINS ${shQuote(corsOrigins)}
set_env VITE_API_BASE_URL ""
set_env APP_ENV prod
set_env IMAGE_STORAGE_DRIVER local
set_env IMAGE_STORAGE_LOCAL_DIR /app/storage/uploads
set_env IMAGE_STORAGE_PUBLIC_PATH /uploads

get_env() {
  key="$1"
  grep -m1 "^$key=" deploy/.env.production | sed "s|^$key=||"
}

if [ "${cfg.registrationEnabled}" = "true" ] && [ -z "$(get_env RESEND_API_KEY)" ]; then
  echo "Missing RESEND_API_KEY in deploy/.env.production. Create a new Resend key and set it before opening registration."
  exit 21
fi

compose_v1() {
  docker-compose -p deploy -f deploy/docker-compose.runtime.yml --env-file deploy/.env.production "$@"
}

compose_v2() {
  docker compose --env-file deploy/.env.production -p deploy -f deploy/docker-compose.runtime.yml "$@"
}

if docker compose version >/dev/null 2>&1; then
  compose() { compose_v2 "$@"; }
else
  compose() { compose_v1 "$@"; }
fi

docker rm -f deploy_backend_1 deploy_web_1 >/dev/null 2>&1 || true
compose build backend web
compose up -d --no-deps backend

for i in $(seq 1 45); do
  status=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' deploy_backend_1 2>/dev/null || true)
  echo "backend=$status"
  [ "$status" = "healthy" ] && break
  sleep 2
done

status=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' deploy_backend_1 2>/dev/null || true)
if [ "$status" != "healthy" ]; then
  docker logs --tail=200 deploy_backend_1 || true
  exit 30
fi

compose up -d --no-deps web
compose ps
curl -fsS http://127.0.0.1/healthz
printf "\\n"
curl -fsSI http://127.0.0.1/app | sed -n '1,10p'
`;

  await run(remoteScript);
}

conn.on('ready', () => {
  main()
    .then(() => conn.end())
    .catch((err) => {
      console.error(err.stack || err.message);
      conn.end();
      process.exitCode = 1;
    });
});

conn.on('error', (err) => {
  console.error(err.stack || err.message);
  process.exit(1);
});

conn.connect({
  host: cfg.host,
  port: 22,
  username: cfg.username,
  password: cfg.password,
  readyTimeout: 20000,
});
'@
Set-Content -Path $RunnerPath -Value $Runner -Encoding UTF8

$env:DEPLOY_HOST = $HostName
$env:DEPLOY_USER = $UserName
$env:DEPLOY_PACKAGE = $PackagePath
$env:DEPLOY_REMOTE_PACKAGE = $RemotePackage
$env:DEPLOY_REMOTE_DIR = $RemoteDir
$env:DEPLOY_PUBLIC_URL = $PublicUrl
$env:DEPLOY_REGISTRATION_ENABLED = $RegistrationEnabled
$env:DEPLOY_RESEND_API_KEY = $LocalResendApiKey

node $RunnerPath

Step "Public check"
try {
  $health = Invoke-WebRequest -UseBasicParsing -Uri "$PublicUrl/healthz" -TimeoutSec 15
  Write-Host $health.Content
  $app = Invoke-WebRequest -UseBasicParsing -Uri "$PublicUrl/app" -Method Head -TimeoutSec 15
  Write-Host "Frontend status: $($app.StatusCode)"
} catch {
  Write-Warning "Public check failed: $($_.Exception.Message)"
}

Write-Host ""
Write-Host "Deploy completed: $PublicUrl/app" -ForegroundColor Green

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $projectRoot

# --- Load .env (if present) so this script honors the same configuration ---
# --- compose.yaml uses, instead of hardcoding values that can drift from it. ---
function Read-DotEnv([string]$path) {
    $values = @{}
    if (Test-Path $path) {
        foreach ($line in Get-Content $path) {
            $trimmed = $line.Trim()
            if ($trimmed -eq '' -or $trimmed.StartsWith('#')) { continue }
            $parts = $trimmed.Split('=', 2)
            if ($parts.Length -eq 2) {
                $values[$parts[0].Trim()] = $parts[1].Trim()
            }
        }
    }
    return $values
}

function Get-Setting([hashtable]$dotEnv, [string]$key, [string]$fallback) {
    if ($dotEnv.ContainsKey($key) -and $dotEnv[$key] -ne '') { return $dotEnv[$key] }
    return $fallback
}

$dotEnv = Read-DotEnv (Join-Path $projectRoot '.env')

$postgresUser = Get-Setting $dotEnv 'POSTGRES_USER' 'petconnect'
$postgresPassword = Get-Setting $dotEnv 'POSTGRES_PASSWORD' 'petconnect_dev'
$postgresDb = Get-Setting $dotEnv 'POSTGRES_DB' 'petconnect'
$postgresPort = Get-Setting $dotEnv 'POSTGRES_PORT' '5433'
$redisPort = Get-Setting $dotEnv 'REDIS_PORT' '6380'
$minioUser = Get-Setting $dotEnv 'MINIO_ROOT_USER' 'petconnect'
$minioPassword = Get-Setting $dotEnv 'MINIO_ROOT_PASSWORD' 'petconnect_dev_secret'
$minioPort = Get-Setting $dotEnv 'MINIO_PORT' '9000'
$apiPort = Get-Setting $dotEnv 'API_PORT' '8080'
$publicBaseUrl = Get-Setting $dotEnv 'PUBLIC_BASE_URL' "http://localhost:$apiPort"
$allowedOrigins = Get-Setting $dotEnv 'ALLOWED_ORIGINS' 'http://localhost:*;http://127.0.0.1:*'

# --- Start dependencies ---

Write-Host 'Starting PetConnect dependencies (postgres, redis, minio)...'
docker compose up -d postgres redis minio

# --- Wait for each dependency's own healthcheck, instead of assuming ---
# --- `docker compose up -d` returning means the service is ready. ---
function Wait-ServiceHealthy([string]$service, [int]$timeoutSeconds = 90) {
    $elapsed = 0
    $intervalSeconds = 2
    Write-Host -NoNewline "Waiting for $service to become healthy"
    while ($elapsed -lt $timeoutSeconds) {
        $raw = docker compose ps $service --format json 2>$null
        if ($raw) {
            $lines = $raw -split "`n" | Where-Object { $_.Trim() -ne '' }
            foreach ($line in $lines) {
                try {
                    $status = ($line | ConvertFrom-Json).Health
                    if ($status -eq 'healthy') {
                        Write-Host ' healthy.'
                        return
                    }
                } catch {
                    # Older Docker Compose can print one JSON array instead of
                    # JSON-lines; fall back to parsing the whole payload once.
                    try {
                        $status = ($raw | ConvertFrom-Json)[0].Health
                        if ($status -eq 'healthy') {
                            Write-Host ' healthy.'
                            return
                        }
                    } catch {}
                }
            }
        }
        Write-Host -NoNewline '.'
        Start-Sleep -Seconds $intervalSeconds
        $elapsed += $intervalSeconds
    }
    Write-Host ''
    throw "Timed out waiting for '$service' to become healthy after ${timeoutSeconds}s. Check 'docker compose logs $service'."
}

Wait-ServiceHealthy 'postgres'
Wait-ServiceHealthy 'redis'
Wait-ServiceHealthy 'minio'

# --- Configure the Go process (mirrors the dependency config above so the ---
# --- API always talks to the same instances this script just started). ---

$env:APP_ENV = 'local'
$env:HTTP_ADDR = ":$apiPort"
$env:DATABASE_URL = "postgres://${postgresUser}:${postgresPassword}@127.0.0.1:${postgresPort}/${postgresDb}?sslmode=disable"
$env:REDIS_ADDR = "127.0.0.1:$redisPort"
$env:S3_ENDPOINT = "127.0.0.1:$minioPort"
$env:S3_ACCESS_KEY = $minioUser
$env:S3_SECRET_KEY = $minioPassword
$env:S3_BUCKET = 'petconnect-media'
$env:PUBLIC_BASE_URL = $publicBaseUrl
$env:ALLOWED_ORIGINS = $allowedOrigins
$env:DEMO_ASSET_DIR = Join-Path $projectRoot 'assets\images'
$env:GOTELEMETRY = 'off'
$env:GOTOOLCHAIN = 'local'

Set-Location (Join-Path $projectRoot 'server')

Write-Host 'Seeding local demo data (safe to re-run; no-ops if already seeded)...'
go run ./cmd/seed

Write-Host "Starting PetConnect API at http://localhost:$apiPort..."
go run ./cmd/api

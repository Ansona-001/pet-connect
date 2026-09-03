$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $projectRoot

Write-Host 'Starting PetConnect dependencies...'
docker compose up -d postgres redis minio

$env:DATABASE_URL = 'postgres://petconnect:petconnect_dev@127.0.0.1:5433/petconnect?sslmode=disable'
$env:REDIS_ADDR = '127.0.0.1:6380'
$env:S3_ENDPOINT = '127.0.0.1:9000'
$env:S3_ACCESS_KEY = 'petconnect'
$env:S3_SECRET_KEY = 'petconnect_dev_secret'
$env:S3_BUCKET = 'petconnect-media'
$env:DEMO_ASSET_DIR = Join-Path $projectRoot 'assets\images'
$env:PUBLIC_BASE_URL = 'http://localhost:8080'
$env:GOTELEMETRY = 'off'
$env:GOTOOLCHAIN = 'local'

Set-Location (Join-Path $projectRoot 'server')
Write-Host 'Starting PetConnect API at http://localhost:8080...'
go run ./cmd/api

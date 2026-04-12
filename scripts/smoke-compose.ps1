param(
    [string]$ComposeFile = (Join-Path $PSScriptRoot "..\deployments\compose\docker-compose.yml"),
    [string]$ApiBaseUrl = "http://127.0.0.1:8080",
    [string]$ApiKey = "dev-api-key",
    [string]$TriggerAt = "2026-04-11T07:00:00+08:00"
)

$ErrorActionPreference = "Stop"

function Wait-HttpJson {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [int]$TimeoutSeconds = 90
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            return Invoke-RestMethod -Method Get -Uri $Uri
        }
        catch {
            Start-Sleep -Seconds 2
        }
    }

    throw "Timed out waiting for $Uri"
}

$resolvedComposeFile = (Resolve-Path $ComposeFile).Path
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Write-Host "Using compose file: $resolvedComposeFile"
Write-Host "Using repo root: $repoRoot"

Push-Location $repoRoot
try {
    docker compose -f $resolvedComposeFile down --volumes --remove-orphans
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose down --volumes --remove-orphans failed with exit code $LASTEXITCODE"
    }

    npm --prefix web install
    if ($LASTEXITCODE -ne 0) {
        throw "npm --prefix web install failed with exit code $LASTEXITCODE"
    }

    npm --prefix web run build
    if ($LASTEXITCODE -ne 0) {
        throw "npm --prefix web run build failed with exit code $LASTEXITCODE"
    }

    docker compose -f $resolvedComposeFile build rss-api
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose build rss-api failed with exit code $LASTEXITCODE"
    }

    docker compose -f $resolvedComposeFile up -d
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose up -d failed with exit code $LASTEXITCODE"
    }

    $null = Wait-HttpJson -Uri "$ApiBaseUrl/healthz"
    $adminStatus = Wait-HttpJson -Uri "$ApiBaseUrl/api/v1/admin/status"
    if (-not $adminStatus.runtime) {
        throw "admin status missing runtime"
    }

    $indexHtml = Invoke-WebRequest -Uri "$ApiBaseUrl/dashboard"
    if ($indexHtml.Content -notmatch "FluxDigest") {
        throw "spa index missing FluxDigest"
    }

    $triggerBody = @{ trigger_at = $TriggerAt } | ConvertTo-Json -Compress
    $trigger = Invoke-RestMethod `
        -Method Post `
        -Uri "$ApiBaseUrl/api/v1/jobs/daily-digest" `
        -Headers @{ "X-API-Key" = $ApiKey } `
        -ContentType "application/json" `
        -Body $triggerBody

    if ($trigger.status -ne "accepted") {
        throw "Expected accepted on clean baseline, got: $($trigger | ConvertTo-Json -Compress)"
    }
    if ($trigger.digest_date -ne "2026-04-11") {
        throw "Unexpected digest_date from job trigger: $($trigger | ConvertTo-Json -Compress)"
    }

    $articles = $null
    $deadline = (Get-Date).AddSeconds(60)
    while ((Get-Date) -lt $deadline) {
        try {
            $articles = Invoke-RestMethod -Method Get -Uri "$ApiBaseUrl/api/v1/articles"
            if ($articles.items.Count -gt 0 -and
                -not [string]::IsNullOrWhiteSpace($articles.items[0].title_translated) -and
                -not [string]::IsNullOrWhiteSpace($articles.items[0].core_summary)) {
                break
            }
        }
        catch {
        }

        Start-Sleep -Seconds 2
    }

    if (-not $articles -or $articles.items.Count -eq 0) {
        throw "Articles endpoint did not return processed items."
    }
    if ([string]::IsNullOrWhiteSpace($articles.items[0].title_translated)) {
        throw "Processed article missing title_translated: $($articles | ConvertTo-Json -Depth 6 -Compress)"
    }
    if ([string]::IsNullOrWhiteSpace($articles.items[0].core_summary)) {
        throw "Processed article missing core_summary: $($articles | ConvertTo-Json -Depth 6 -Compress)"
    }

    $latest = $null
    $deadline = (Get-Date).AddSeconds(60)
    while ((Get-Date) -lt $deadline) {
        try {
            $latest = Invoke-RestMethod -Method Get -Uri "$ApiBaseUrl/api/v1/digests/latest"
            if (-not [string]::IsNullOrWhiteSpace($latest.title) -and $latest.digest_date -eq "2026-04-11") {
                break
            }
        }
        catch {
        }

        Start-Sleep -Seconds 2
    }

    if (-not $latest) {
        throw "Latest digest endpoint did not return a payload."
    }
    if ([string]::IsNullOrWhiteSpace($latest.title)) {
        throw "Latest digest title is empty: $($latest | ConvertTo-Json -Depth 5 -Compress)"
    }
    if ($latest.digest_date -ne "2026-04-11") {
        throw "Latest digest date mismatch: $($latest | ConvertTo-Json -Depth 5 -Compress)"
    }

    Write-Host "Smoke OK"
    Write-Host ("  job_status       : {0}" -f $trigger.status)
    Write-Host ("  article_title_cn : {0}" -f $articles.items[0].title_translated)
    Write-Host ("  article_summary  : {0}" -f $articles.items[0].core_summary)
    Write-Host ("  digest_date      : {0}" -f $latest.digest_date)
    Write-Host ("  digest_title     : {0}" -f $latest.title)
}
finally {
    docker compose -f $resolvedComposeFile down --volumes --remove-orphans | Out-Host
    Pop-Location
}

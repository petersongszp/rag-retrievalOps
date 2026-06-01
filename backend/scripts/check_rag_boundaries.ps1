# RAG 边界依赖检查脚本 (PowerShell 版)
# 用于检查 RAG 模块与业务模块之间的违规导入
# 用法: powershell -File scripts/check_rag_boundaries.ps1

Set-Location (Join-Path $PSScriptRoot "..")

Write-Host "=== Checking RAG -> Business imports ===" -ForegroundColor Cyan
$violations = 0

$ragDirs = @("internal/milvus", "internal/rag", "api/handler/kb", "internal/model/kb_", "internal/service/kb")
$bizPkgs = @("internal/agents", "internal/service/interview", "internal/service/resume", "internal/service/prediction", "internal/payment", "internal/model/interview_", "internal/model/resume", "internal/model/prediction", "internal/model/payment_", "internal/model/subscription")

foreach ($ragDir in $ragDirs) {
    foreach ($bizPkg in $bizPkgs) {
        $searchPattern = "`"interview-agents/$bizPkg"
        if (Test-Path $ragDir) {
            $matches = Get-ChildItem -Path $ragDir -Recurse -File -ErrorAction SilentlyContinue |
                Select-String -Pattern $searchPattern -ErrorAction SilentlyContinue
            if ($matches) {
                foreach ($match in $matches) {
                    Write-Host "VIOLATION: RAG package $ragDir imports business package $bizPkg" -ForegroundColor Red
                    Write-Host "  $($match.Filename):$($match.LineNumber): $($match.Line.Trim())" -ForegroundColor Yellow
                    $violations++
                }
            }
        }
    }
}

Write-Host ""
Write-Host "=== Checking Business -> RAG internal imports ===" -ForegroundColor Cyan
$bizDirs = @("internal/agents", "internal/service/interview", "internal/service/resume", "internal/service/prediction", "internal/payment")
$ragPkgs = @("internal/milvus", "internal/rag", "api/handler/kb")

foreach ($bizDir in $bizDirs) {
    foreach ($ragPkg in $ragPkgs) {
        $searchPattern = "`"interview-agents/$ragPkg"
        if (Test-Path $bizDir) {
            $matches = Get-ChildItem -Path $bizDir -Recurse -File -ErrorAction SilentlyContinue |
                Select-String -Pattern $searchPattern -ErrorAction SilentlyContinue
            if ($matches) {
                foreach ($match in $matches) {
                    Write-Host "VIOLATION: Business package $bizDir imports RAG package $ragPkg" -ForegroundColor Red
                    Write-Host "  $($match.Filename):$($match.LineNumber): $($match.Line.Trim())" -ForegroundColor Yellow
                    $violations++
                }
            }
        }
    }
}

Write-Host ""
if ($violations -gt 0) {
    Write-Host "Found $violations boundary violations!" -ForegroundColor Red
    exit 1
}
Write-Host "No boundary violations found." -ForegroundColor Green

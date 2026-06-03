param(
  [string]$BaseUrl = $env:RAG_BASE_URL,
  [string]$ApiKey = $env:RAG_API_KEY,
  [string]$KBID = $env:KB_ID,
  [string]$Query = $env:QUERY
)

if (-not $BaseUrl) { $BaseUrl = "http://localhost:8081" }
if (-not $KBID) { $KBID = "1" }
if (-not $Query) { $Query = "知识库里关于 Go 并发的内容是什么？" }

Write-Host "== Phase 4 Retrieve Smoke =="
Write-Host "BASE_URL=$BaseUrl"
Write-Host "KB_ID=$KBID"

if ($ApiKey) {
  Write-Host ""
  Write-Host "== Test: API Key retrieve =="
  $body = @{
    query = $Query
    kb_ids = @([int]$KBID)
    top_k = 5
  } | ConvertTo-Json

  Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/retrieve" -Headers @{
    Authorization = "Bearer $ApiKey"
  } -ContentType "application/json" -Body $body | ConvertTo-Json -Depth 8
}
else {
  Write-Host ""
  Write-Host "RAG_API_KEY 未设置，跳过 API Key smoke。"
}

Write-Host ""
Write-Host "== Test: legacy app_id retrieve =="
$legacyBody = @{
  app_id = "interview-agent"
  query = $Query
  kb_ids = @([int]$KBID)
  top_k = 3
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri "$BaseUrl/v1/retrieve" -ContentType "application/json" -Body $legacyBody | ConvertTo-Json -Depth 8

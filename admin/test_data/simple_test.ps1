# 简单的测试脚本

$apiBase = "http://localhost:8899/api"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "=== 创建测试数据 ==="

# 1. 创建知识库
Write-Host "1. 创建知识库..."
$createKbUrl = "$apiBase/admin/kb/bases"
$createKbBody = @{
    name = "测试知识库"
    description = "用于测试的知识库"
} | ConvertTo-Json

try {
    $kbResponse = Invoke-RestMethod -Uri $createKbUrl -Method Post -Body $createKbBody -ContentType "application/json"
    Write-Host "成功创建知识库!" -ForegroundColor Green
    Write-Host ($kbResponse | ConvertTo-Json)
    $kbId = if ($kbResponse.id) { $kbResponse.id } else { 1 }
} catch {
    Write-Host "创建知识库失败: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "尝试获取现有知识库..."
    $kbResponse = Invoke-RestMethod -Uri $createKbUrl -Method Get
    $kbId = if ($kbResponse.items -and $kbResponse.items.Count -gt 0) { $kbResponse.items[0].id } else { 1 }
}

Write-Host "使用知识库 ID: $kbId"

# 2. 上传文档
Write-Host "2. 上传文档..."
$uploadUrl = "$apiBase/admin/kb/documents/upload"

$filesToUpload = @(
    @{ name = "go_introduction.md"; path = Join-Path $scriptDir "go_introduction.md" }
)

foreach ($fileInfo in $filesToUpload) {
    if (Test-Path $fileInfo.path) {
        try {
            $formData = @{
                kb_id = $kbId
                file = Get-Item -Path $fileInfo.path
            }
            $uploadResult = Invoke-RestMethod -Uri $uploadUrl -Method Post -Form $formData
            Write-Host "成功上传 $($fileInfo.name)!" -ForegroundColor Green
            Write-Host ($uploadResult | ConvertTo-Json)
        } catch {
            Write-Host "上传失败 $($fileInfo.name): $($_.Exception.Message)" -ForegroundColor Red
        }
    }
}

# 3. 查看文档列表
Write-Host "3. 查看文档..."
$docsUrl = "$apiBase/admin/kb/documents?kb_id=$kbId"
try {
    $docsResponse = Invoke-RestMethod -Uri $docsUrl -Method Get
    Write-Host "文档列表:" -ForegroundColor Yellow
    Write-Host ($docsResponse | ConvertTo-Json)
} catch {
    Write-Host "获取文档列表失败" -ForegroundColor Red
}

# 4. 查看任务列表
Write-Host "4. 查看任务..."
$jobsUrl = "$apiBase/admin/kb/jobs"
try {
    $jobsResponse = Invoke-RestMethod -Uri $jobsUrl -Method Get
    Write-Host "任务列表:" -ForegroundColor Yellow
    Write-Host ($jobsResponse | ConvertTo-Json)
} catch {
    Write-Host "获取任务列表失败" -ForegroundColor Red
}

Write-Host "完成!" -ForegroundColor Green

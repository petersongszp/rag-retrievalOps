# 创建测试知识库和文档数据的 PowerShell 脚本

$ErrorActionPreference = "Stop"

# API 基础地址
$apiBase = "http://localhost:8899/api"

Write-Host "=== 创建测试知识库和文档 ===" -ForegroundColor Cyan
Write-Host ""

# 函数：创建知识库
function Create-KnowledgeBase {
    param(
        [string]$name,
        [string]$description = ""
    )
    
    $url = "$apiBase/admin/kb/bases"
    $payload = @{
        name = $name
        description = $description
    } | ConvertTo-Json
    
    try {
        Write-Host "正在创建知识库: $name"
        $response = Invoke-RestMethod -Uri $url -Method Post -Body $payload -ContentType "application/json"
        Write-Host "成功创建知识库!" -ForegroundColor Green
        Write-Host ($response | ConvertTo-Json -Depth 10)
        return $response
    }
    catch {
        Write-Host "创建知识库失败: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.Exception.Response) {
            $responseStream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($responseStream)
            $errorBody = $reader.ReadToEnd()
            Write-Host "错误详情: $errorBody" -ForegroundColor Red
        }
        return $null
    }
}

# 函数：列出所有知识库
function List-KnowledgeBases {
    $url = "$apiBase/admin/kb/bases"
    
    try {
        $response = Invoke-RestMethod -Uri $url -Method Get
        Write-Host "知识库列表:" -ForegroundColor Yellow
        Write-Host ($response | ConvertTo-Json -Depth 10)
        return $response
    }
    catch {
        Write-Host "获取知识库列表失败: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

# 函数：上传文档
function Upload-Document {
    param(
        [string]$kbId,
        [string]$filePath
    )
    
    $url = "$apiBase/admin/kb/documents/upload"
    $fileName = Split-Path $filePath -Leaf
    
    try {
        Write-Host "正在上传文档: $fileName"
        $form = @{
            kb_id = $kbId
            file = Get-Item -Path $filePath
        }
        
        $response = Invoke-RestMethod -Uri $url -Method Post -Form $form
        Write-Host "成功上传文档!" -ForegroundColor Green
        Write-Host ($response | ConvertTo-Json -Depth 10)
        return $response
    }
    catch {
        Write-Host "上传文档失败: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.Exception.Response) {
            $responseStream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($responseStream)
            $errorBody = $reader.ReadToEnd()
            Write-Host "错误详情: $errorBody" -ForegroundColor Red
        }
        return $null
    }
}

# 函数：列出文档
function List-Documents {
    param([string]$kbId)
    
    $url = "$apiBase/admin/kb/documents?kb_id=$kbId"
    
    try {
        $response = Invoke-RestMethod -Uri $url -Method Get
        Write-Host "文档列表:" -ForegroundColor Yellow
        Write-Host ($response | ConvertTo-Json -Depth 10)
        return $response
    }
    catch {
        Write-Host "获取文档列表失败: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

# 函数：列出任务
function List-Jobs {
    $url = "$apiBase/admin/kb/jobs"
    
    try {
        $response = Invoke-RestMethod -Uri $url -Method Get
        Write-Host "任务列表:" -ForegroundColor Yellow
        Write-Host ($response | ConvertTo-Json -Depth 10)
        return $response
    }
    catch {
        Write-Host "获取任务列表失败: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

# 主程序
try {
    # Step 1: 创建知识库
    Write-Host "Step 1: 创建知识库" -ForegroundColor Cyan
    $kb = Create-KnowledgeBase `
        -name "测试知识库 - 编程语言" `
        -description "包含 Go、React、Python 等编程语言相关文档"
    
    # 如果创建失败，尝试列出已有的知识库
    if (-not $kb) {
        Write-Host ""
        Write-Host "尝试列出已有的知识库..." -ForegroundColor Yellow
        $kbList = List-KnowledgeBases
        if ($kbList -and $kbList.items -and $kbList.items.Count -gt 0) {
            $kb = $kbList.items[0]
            Write-Host "使用现有知识库: $($kb.name) (ID: $($kb.id))" -ForegroundColor Cyan
        }
    }
    
    if (-not $kb) {
        Write-Host "无法创建或获取知识库，脚本终止" -ForegroundColor Red
        exit 1
    }
    
    $kbId = if ($kb.id) { $kb.id } elseif ($kb.ID) { $kb.ID } else { 1 }
    Write-Host ""
    Write-Host "使用知识库 ID: $kbId" -ForegroundColor Cyan
    
    # Step 2: 上传测试文档
    Write-Host ""
    Write-Host "Step 2: 上传测试文档" -ForegroundColor Cyan
    
    $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    $testFiles = @(
        Join-Path $scriptDir "go_introduction.md",
        Join-Path $scriptDir "react_tutorial.md"
    )
    
    foreach ($testFile in $testFiles) {
        if (Test-Path $testFile) {
            Upload-Document -kbId $kbId -filePath $testFile
            Write-Host ""
        }
        else {
            Write-Host "警告: 文件 $testFile 不存在" -ForegroundColor Yellow
            Write-Host ""
        }
    }
    
    # Step 3: 列出文档和任务
    Write-Host "Step 3: 检查数据" -ForegroundColor Cyan
    List-Documents -kbId $kbId
    Write-Host ""
    List-Jobs
    
    Write-Host ""
    Write-Host "=== 脚本执行完成 ===" -ForegroundColor Cyan
}
catch {
    Write-Host "脚本执行异常: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host $_.ScriptStackTrace -ForegroundColor DarkGray
    exit 1
}

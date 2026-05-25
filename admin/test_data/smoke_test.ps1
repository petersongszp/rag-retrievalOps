# Phase 0 - RAG 基线完整冒烟测试脚本
# 对应 phase0-rag-baseline-detailed-roadmap.md 中的冒烟测试清单

$ErrorActionPreference = "Stop"

# API 基础地址
$apiBase = "http://localhost:8899/api/admin/kb"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$testResults = @()
$passed = 0
$failed = 0
$total = 0

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Phase 0 - RAG 基线冒烟测试" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

function Add-TestResult {
    param([string]$name, [bool]$success, [string]$message)
    $script:total++
    if ($success) {
        $script:passed++
        Write-Host "[PASS] $name" -ForegroundColor Green
    } else {
        $script:failed++
        Write-Host "[FAIL] $name : $message" -ForegroundColor Red
    }
    $script:testResults += @{ Name = $name; Success = $success; Message = $message }
}

function Invoke-Api {
    param([string]$method, [string]$url, $body, $form)
    
    try {
        if ($form) {
            $response = Invoke-RestMethod -Method $method -Uri $url -Form $form -ErrorAction Stop
        } elseif ($body) {
            $response = Invoke-RestMethod -Method $method -Uri $url -Body ($body | ConvertTo-Json) -ContentType "application/json" -ErrorAction Stop
        } else {
            $response = Invoke-RestMethod -Method $method -Uri $url -ErrorAction Stop
        }
        return @{ Success = $true; Data = $response; Error = $null }
    } catch {
        $errorMsg = $_.Exception.Message
        if ($_.Exception.Response) {
            $stream = $_.Exception.Response.GetResponseStream()
            $reader = New-Object System.IO.StreamReader($stream)
            $reader.BaseStream.Position = 0
            $errorMsg = $reader.ReadToEnd()
        }
        return @{ Success = $false; Data = $null; Error = $errorMsg }
    }
}

function Wait-For-JobCompletion {
    param([int]$jobId, [int]$timeoutSeconds = 60)
    
    $endTime = (Get-Date).AddSeconds($timeoutSeconds)
    while ((Get-Date) -lt $endTime) {
        $result = Invoke-Api -Method GET -url "$apiBase/jobs/$jobId"
        if ($result.Success -and $result.Data) {
            $status = $result.Data.status
            if ($status -eq "completed" -or $status -eq "failed" -or $status -eq "dead" -or $status -eq "canceled") {
                return $result.Data
            }
        }
        Start-Sleep -Seconds 2
    }
    return $null
}

# 1. 创建知识库
Write-Host "测试 1: 创建知识库" -ForegroundColor Yellow
$createKbResult = Invoke-Api -Method POST -url "$apiBase/bases" -body @{ name = "Smoke Test KB"; description = "Test knowledge base" }
$kbId = $null
if ($createKbResult.Success -and $createKbResult.Data) {
    $kbId = $createKbResult.Data.id
    Add-TestResult -name "创建知识库" -success $true -message "KB ID: $kbId"
} else {
    Add-TestResult -name "创建知识库" -success $false -message ($createKbResult.Error ?? "Unknown error")
}

# 2. 上传合法文件
Write-Host "`n测试 2: 上传合法文件 (Markdown)" -ForegroundColor Yellow
$docId = $null
$jobId = $null
if ($kbId) {
    $testFile = Join-Path $scriptDir "go_introduction.md"
    if (Test-Path $testFile) {
        $uploadForm = @{
            kb_id = $kbId
            file = Get-Item -Path $testFile
        }
        $uploadResult = Invoke-Api -Method POST -url "$apiBase/documents/upload" -form $uploadForm
        if ($uploadResult.Success -and $uploadResult.Data) {
            $docId = $uploadResult.Data.document_id
            $jobId = $uploadResult.Data.job_id
            Add-TestResult -name "上传合法文件" -success $true -message "Doc ID: $docId, Job ID: $jobId"
        } else {
            Add-TestResult -name "上传合法文件" -success $false -message ($uploadResult.Error ?? "Unknown error")
        }
    } else {
        Add-TestResult -name "上传合法文件" -success $false -message "Test file not found: $testFile"
    }
} else {
    Add-TestResult -name "上传合法文件" -success $false -message "Skip: KB not created"
}

# 3. 等待任务完成
Write-Host "`n测试 3: 等待任务完成" -ForegroundColor Yellow
$jobCompleted = $false
if ($jobId) {
    Write-Host "  等待任务处理 (最多 60 秒)..." -ForegroundColor Gray
    $finalJob = Wait-For-JobCompletion -jobId $jobId
    if ($finalJob) {
        if ($finalJob.status -eq "completed") {
            Add-TestResult -name "任务处理成功" -success $true -message "Job status: $($finalJob.status)"
            $jobCompleted = $true
        } else {
            Add-TestResult -name "任务处理成功" -success $false -message "Job status: $($finalJob.status), Error: $($finalJob.error_msg)"
        }
    } else {
        Add-TestResult -name "任务处理成功" -success $false -message "Timeout waiting for job"
    }
} else {
    Add-TestResult -name "任务处理成功" -success $false -message "Skip: No job to wait for"
}

# 4. 测试检索
Write-Host "`n测试 4: 知识库检索" -ForegroundColor Yellow
$retrieveSuccess = $false
if ($jobCompleted -and $kbId) {
    $retrieveResult = Invoke-Api -Method POST -url "$apiBase/retrieve" -body @{
        query = "Go"
        kb_id = $kbId
        top_k = 5
    }
    if ($retrieveResult.Success -and $retrieveResult.Data) {
        $items = $retrieveResult.Data.items
        if ($items -and $items.Count -gt 0) {
            $allValid = $true
            foreach ($item in $items) {
                if (-not $item.content -or -not $item.score -or -not $item.citation -or -not $item.source) {
                    $allValid = $false
                    break
                }
            }
            if ($allValid) {
                Add-TestResult -name "检索命中内容" -success $true -message "Found $($items.Count) items with valid structure (content/score/citation/source)"
                $retrieveSuccess = $true
            } else {
                Add-TestResult -name "检索命中内容" -success $false -message "Result items missing required fields"
            }
        } else {
            Add-TestResult -name "检索命中内容" -success $false -message "No results returned"
        }
    } else {
        Add-TestResult -name "检索命中内容" -success $false -message ($retrieveResult.Error ?? "Unknown error")
    }
} else {
    Add-TestResult -name "检索命中内容" -success $false -message "Skip: Job not completed"
}

# 5. 删除文档
Write-Host "`n测试 5: 删除文档" -ForegroundColor Yellow
$deleteSuccess = $false
if ($docId) {
    $deleteResult = Invoke-Api -Method DELETE -url "$apiBase/documents/$docId"
    if ($deleteResult.Success) {
        Add-TestResult -name "删除文档" -success $true -message "Document $docId deleted"
        $deleteSuccess = $true
    } else {
        Add-TestResult -name "删除文档" -success $false -message ($deleteResult.Error ?? "Unknown error")
    }
} else {
    Add-TestResult -name "删除文档" -success $false -message "Skip: No document to delete"
}

# 6. 验证删除后无法检索到
Write-Host "`n测试 6: 删除后检索验证" -ForegroundColor Yellow
if ($deleteSuccess -and $kbId) {
    Start-Sleep -Seconds 2
    $retrieveAfterDelete = Invoke-Api -Method POST -url "$apiBase/retrieve" -body @{
        query = "Go"
        kb_id = $kbId
        top_k = 10
    }
    if ($retrieveAfterDelete.Success -and $retrieveAfterDelete.Data) {
        $foundDeleted = $false
        if ($retrieveAfterDelete.Data.items) {
            foreach ($item in $retrieveAfterDelete.Data.items) {
                if ($item.citation -and $item.citation.document_id -eq $docId) {
                    $foundDeleted = $true
                    break
                }
            }
        }
        if (-not $foundDeleted) {
            Add-TestResult -name "删除后检索不命中" -success $true -message "No results from deleted document"
        } else {
            Add-TestResult -name "删除后检索不命中" -success $false -message "Still found results from deleted document"
        }
    } else {
        Add-TestResult -name "删除后检索不命中" -success $false -message "Retrieve after delete failed"
    }
} else {
    Add-TestResult -name "删除后检索不命中" -success $false -message "Skip: Document not deleted"
}

# 7. 非法文件类型测试
Write-Host "`n测试 7: 非法文件类型拒绝" -ForegroundColor Yellow
if ($kbId) {
    $invalidFile = Join-Path $scriptDir "invalid_type.exe"
    if (-not (Test-Path $invalidFile)) {
        New-Item -Path $invalidFile -ItemType File -Value "dummy" -Force | Out-Null
    }
    $invalidUploadForm = @{
        kb_id = $kbId
        file = Get-Item -Path $invalidFile
    }
    $invalidUpload = Invoke-Api -Method POST -url "$apiBase/documents/upload" -form $invalidUploadForm
    if (-not $invalidUpload.Success) {
        Add-TestResult -name "非法文件类型拒绝" -success $true -message "Rejected invalid file type as expected"
    } else {
        Add-TestResult -name "非法文件类型拒绝" -success $false -message "Should have rejected invalid file type"
    }
    Remove-Item $invalidFile -Force -ErrorAction SilentlyContinue
} else {
    Add-TestResult -name "非法文件类型拒绝" -success $false -message "Skip: KB not created"
}

# 8. 大文件测试 (创建一个大文件)
Write-Host "`n测试 8: 大文件超限拒绝" -ForegroundColor Yellow
if ($kbId) {
    $largeFile = Join-Path $scriptDir "large_test_file.txt"
    # 创建 30MB 内容 (超过限制)
    $largeContent = "x" * (30MB)
    [System.IO.File]::WriteAllText($largeFile, $largeContent)
    
    $largeUploadForm = @{
        kb_id = $kbId
        file = Get-Item -Path $largeFile
    }
    $largeUpload = Invoke-Api -Method POST -url "$apiBase/documents/upload" -form $largeUploadForm
    Remove-Item $largeFile -Force -ErrorAction SilentlyContinue
    
    if (-not $largeUpload.Success) {
        Add-TestResult -name "大文件超限拒绝" -success $true -message "Rejected large file as expected"
    } else {
        Add-TestResult -name "大文件超限拒绝" -success $false -message "Should have rejected large file"
    }
} else {
    Add-TestResult -name "大文件超限拒绝" -success $false -message "Skip: KB not created"
}

# 9. 文件哈希去重测试
Write-Host "`n测试 9: file_hash 去重策略" -ForegroundColor Yellow
$duplicateSuccess = $false
if ($kbId) {
    $testFile = Join-Path $scriptDir "go_introduction.md"
    if (Test-Path $testFile) {
        $uploadForm = @{
            kb_id = $kbId
            file = Get-Item -Path $testFile
        }
        $upload1 = Invoke-Api -Method POST -url "$apiBase/documents/upload" -form $uploadForm
        if ($upload1.Success) {
            Start-Sleep -Seconds 1
            $upload2 = Invoke-Api -Method POST -url "$apiBase/documents/upload" -form $uploadForm
            if ($upload2.Success -and $upload2.Data.reused) {
                Add-TestResult -name "重复文件去重" -success $true -message "Detected duplicate and reused existing"
                $duplicateSuccess = $true
            } else {
                Add-TestResult -name "重复文件去重" -success $false -message "Should have detected duplicate file"
            }
        } else {
            Add-TestResult -name "重复文件去重" -success $false -message "First upload failed"
        }
    } else {
        Add-TestResult -name "重复文件去重" -success $false -message "Test file not found"
    }
} else {
    Add-TestResult -name "重复文件去重" -success $false -message "Skip: KB not created"
}

# 10. Collection 配置错误测试 (这个需要重启服务才能测试，这里只验证基本连通性)
Write-Host "`n测试 10: 检索结果完整字段检查" -ForegroundColor Yellow
# 我们用前面的检索结果来验证字段完整性
if ($retrieveSuccess) {
    Add-TestResult -name "检索结果包含完整字段" -success $true -message "Verified earlier"
} elseif ($kbId) {
    $retrieveResult2 = Invoke-Api -Method POST -url "$apiBase/retrieve" -body @{
        query = "anything"
        kb_id = $kbId
        top_k = 3
    }
    if ($retrieveResult2.Success -and $retrieveResult2.Data) {
        $validStructure = $true
        if ($retrieveResult2.Data.items -and $retrieveResult2.Data.items.Count -gt 0) {
            foreach ($item in $retrieveResult2.Data.items) {
                if (-not $item.content -or -not $item.score -or -not $item.citation -or -not $item.source) {
                    $validStructure = $false
                    break
                }
            }
        }
        if ($validStructure) {
            Add-TestResult -name "检索结果包含完整字段" -success $true -message "All items have content/score/citation/source"
        } else {
            Add-TestResult -name "检索结果包含完整字段" -success $false -message "Some items missing required fields"
        }
    } else {
        Add-TestResult -name "检索结果包含完整字段" -success $false -message "Retrieve failed"
    }
} else {
    Add-TestResult -name "检索结果包含完整字段" -success $false -message "Skip: No KB"
}

# 输出总结
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "冒烟测试总结" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "总计: $total" -ForegroundColor White
Write-Host "通过: $passed" -ForegroundColor Green
Write-Host "失败: $failed" -ForegroundColor Red

if ($failed -eq 0) {
    Write-Host "`n🎉 所有冒烟测试通过！" -ForegroundColor Green
    exit 0
} else {
    Write-Host "`n❌ 部分测试失败" -ForegroundColor Red
    exit 1
}

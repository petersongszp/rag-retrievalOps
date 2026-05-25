# Phase 0 - RAG 基线回滚预案

## 概述

本文档描述了在 Phase 0 实施过程中遇到问题时的回滚策略和操作步骤。

## 回滚策略

### 1. 代码层面：通过 Feature Flag 临时关闭 RAG 功能

#### 快速关闭整个 RAG 模块

**配置文件位置**: `backend/config.yaml`

```yaml
rag:
  enabled: false  # 设置为 false 即可关闭 RAG 功能
```

**影响**:
- 所有 `/api/kb/*` 和 `/api/admin/kb/*` 路由不再注册
- Milvus 不会初始化
- 知识摄入消费器仍然运行，但会跳过处理

**操作步骤**:
1. 编辑 `config.yaml`，设置 `rag.enabled: false`
2. 重启后端服务
3. 验证 RAG 相关 API 返回 404 或被禁用

---

#### 仅关闭知识摄入（保留检索）

**API 调用**:

```bash
# 暂停知识摄入
curl -X POST http://localhost:8899/api/admin/kb/ingest/pause

# 查看状态
curl http://localhost:8899/api/admin/kb/ingest/status

# 恢复知识摄入
curl -X POST http://localhost:8899/api/admin/kb/ingest/resume
```

**影响**:
- 新上传的文档不会被处理，但仍然可以上传
- 正在处理的任务会继续完成
- 检索功能仍然可用

---

### 2. 运行层面：暂停知识摄入消费

#### 场景：消费器处理异常

**症状**:
- 大量 ingest job 失败
- Milvus 连接问题导致写入失败
- Embedding API 异常

**回滚步骤**:

1. **立即暂停新的摄入任务**
   ```bash
   curl -X POST http://localhost:8899/api/admin/kb/ingest/pause
   ```

2. **检查失败任务**
   ```bash
   curl http://localhost:8899/api/admin/kb/jobs?status=failed
   ```

3. **排查问题根源**
   - 检查 Milvus 连接状态
   - 检查 Embedding API 健康状况
   - 查看服务日志

4. **根据情况恢复**
   - 问题解决后恢复: `curl -X POST http://localhost:8899/api/admin/kb/ingest/resume`
   - 或使用 Feature Flag 完全关闭 RAG

---

### 3. 数据层面：软删除保护

#### 已实现的保护机制

- **文档删除是软删除**: 数据库中 `deleted` 字段标记为 1，数据保留
- **Milvus 向量会被删除，但数据库记录保留**
- **可以通过数据库恢复被软删除的文档**

#### 误删除恢复步骤

1. **查询被软删除的文档**
   ```sql
   SELECT * FROM kb_document WHERE deleted = 1 AND id = <doc_id>;
   ```

2. **恢复文档**
   ```sql
   UPDATE kb_document SET deleted = 0 WHERE id = <doc_id>;
   ```

3. **重新触发索引**（如果 Milvus 中的向量也被删除）
   - 可能需要重新上传文档或手动重新索引

---

## 紧急回滚流程

### 步骤 1: 评估影响范围

- 检查是否影响核心面试功能
- 确定是 RAG 特有问题还是系统级问题
- 查看错误率和用户影响

### 步骤 2: 选择回滚策略

| 严重程度 | 推荐策略 |
|---------|---------|
| 低 (仅检索质量问题) | 暂停摄入，排查问题 |
| 中 (摄入失败率高) | 暂停摄入 + 准备关闭 RAG |
| 高 (服务崩溃) | 立即通过配置关闭 RAG |

### 步骤 3: 执行回滚

1. 先尝试最轻量级的方案（暂停摄入）
2. 如果不行，再使用 Feature Flag 关闭
3. 最后考虑代码回滚（如果需要）

### 步骤 4: 验证回滚

- 确认服务稳定运行
- 确认核心面试功能正常
- 确认 RAG 功能按预期关闭/暂停

### 步骤 5: 记录和复盘

- 记录问题现象和处理时间
- 记录回滚操作步骤
- 事后进行复盘和改进

---

## 具体场景回滚

### 场景 A: Milvus 服务不可用

**症状**:
- 初始化失败，服务无法启动
- 检索超时或失败
- 文档摄入失败

**回滚步骤**:
1. 设置 `rag.enabled: false`
2. 重启服务
3. 确认服务启动成功
4. 排查 Milvus 问题
5. Milvus 恢复后再重新开启 RAG

---

### 场景 B: 消费器异常占用资源

**症状**:
- CPU/内存使用率异常高
- 消费器日志报错频繁
- 大量任务堆积

**回滚步骤**:
1. 调用暂停 API: `POST /api/admin/kb/ingest/pause`
2. 观察资源使用情况
3. 排查问题原因
4. 修复后恢复消费

---

### 场景 C: 错误数据被索引

**症状**:
- 检索结果包含错误内容
- 需要批量清除已索引数据

**回滚步骤**:
1. 暂停摄入
2. 通过管理界面删除相关文档（软删除）
3. 等待 Milvus 中的向量被清理
4. 重新上传正确的数据

---

## 冒烟测试验证

回滚后，请运行冒烟测试确认功能正常：

```powershell
cd admin/test_data
.\smoke_test.ps1
```

---

## 预防措施

1. **配置检查**: 启动时验证 RAG 相关配置，失败则快速失败
2. **监控告警**: 设置关键指标告警（Milvus 连接、任务失败率）
3. **灰度发布**: Phase 0 初期可以只对部分用户或场景开放
4. **定期备份**: 定期备份知识库相关数据库表

---

## 联系和支持

如遇到问题，参考:
- `phase0-rag-baseline-detailed-roadmap.md` - 原始设计文档
- 代码中的日志和错误信息
- ADMIN_TEST_GUIDE.md - 管理后台测试指南

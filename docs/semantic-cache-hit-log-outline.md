# 语义缓存命中检索日志说明 - 文档大纲

## 1. 背景
- 为什么需要在检索日志里展示语义缓存命中状态
- 当前排查“是否走缓存”的痛点

## 2. 功能目标
- 在检索日志列表中逐条展示是否命中缓存
- 在检索日志详情中展示缓存相关关键字段
- 支持结合 `request_id` 回溯单次请求

## 3. 页面范围
- 检索日志页：`/trace-logs/retrieval`
- 检索调试链路关联页：`/retrieval-lab/debug?request_id=...`
- 语义缓存页：`/semantic-cache`

## 4. 前端展示方案
- 列表新增“Cache”列
- 命中显示 `Hit`
- 未命中显示 `Miss`
- 字段缺失显示 `Contract gap`

## 5. 详情抽屉展示字段
- `semantic_cache_enabled`
- `semantic_cache_hit`
- `semantic_cache_lookup_ms`
- `semantic_cache_similarity`
- `semantic_cache_reason`
- `semantic_cache_entry_id`

## 6. 字段含义说明
- `semantic_cache_hit`：本次检索是否直接命中语义缓存
- `semantic_cache_lookup_ms`：缓存查找耗时
- `semantic_cache_similarity`：命中条目与当前请求的相似度
- `semantic_cache_reason`：命中或未命中的原因
- `semantic_cache_entry_id`：缓存条目标识

## 7. 后端契约来源
- 检索日志模型：`backend/internal/model/kb_retrieve_log.go`
- 检索日志接口：`GET /api/admin/kb/retrieve/audit`
- 单条详情接口：`GET /api/admin/kb/retrieve/audit/{request_id}`

## 8. 使用方式
- 进入检索日志页观察列表中的 `Cache` 列
- 点击某条日志查看详细缓存字段
- 结合 `request_id` 与调试视图做链路核对

## 9. 排查建议
- `Hit` 但结果异常时重点看 `semantic_cache_reason`
- `Miss` 且预期应命中时检查相似度和阈值配置
- 字段为空时优先检查后端日志契约是否完整返回

## 10. 验证用例
- 发送一条可重复命中的查询，确认列表出现 `Hit`
- 发送一条全新查询，确认列表出现 `Miss`
- 打开详情抽屉，确认缓存字段完整展示
- 用 `request_id` 跳转调试页，确认链路可追踪

## 11. 后续可扩展项
- 增加按缓存命中状态筛选
- 增加缓存命中率统计入口
- 在调试视图中补充结构化缓存阶段卡片

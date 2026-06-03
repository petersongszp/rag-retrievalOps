# Phase 0 验收记录

## 验收日期
2026-06-02

## 阶段总览

| 阶段 | 状态 | 提交 | 说明 |
|------|------|------|------|
| L0 | ✅ 完成 | 6277650 | 认证与路由基线盘点 |
| L1 | ✅ 完成 | 658b7c0 | Admin 后门环境门禁 |
| L2 | ✅ 完成 | 4466344 | Bootstrap 测试管理员方案 |
| L3 | ✅ 完成 | 92f4d7c | 统一身份上下文契约冻结 |
| L4 | ✅ 完成 | 92f4d7c | 第一版 API 契约冻结 |
| L5 | ✅ 完成 | 92f4d7c | 旧 app_id 白名单迁移边界 |
| L6 | ✅ 完成 | 66e95d4 | SDK 示例与接入说明更新 |
| L7 | ✅ 完成 | 66e95d4 | 测试回归门禁 |
| L8 | ✅ 完成 | 当前 | 验收收口 |

## 通过标准验证

| # | 标准 | 状态 |
|---|------|------|
| 1 | Admin 注入只能在非生产环境生效 | ✅ |
| 2 | prod 环境配置不合规时启动失败 | ✅ |
| 3 | 测试管理员有确定创建入口 | ✅ |
| 4 | `/v1/auth/*`、`/v1/api-keys`、`/v1/retrieve` 契约已冻结 | ✅ |
| 5 | 旧 `app_id` 白名单兼容路径边界清楚 | ✅ |
| 6 | SDK 示例区分 JWT/API Key/Legacy 三种方式 | ✅ |
| 7 | 自动化测试覆盖核心场景 | ✅ |
| 8 | Phase 1 可直接基于统一身份上下文接入 | ✅ |

## 产出物清单

| 文件 | 用途 |
|------|------|
| `docs/zhuhu/phase0-l0-baseline-report.md` | 路由基线盘点报告 |
| `docs/zhuhu/phase0-l5-legacy-migration-boundary.md` | 旧白名单迁移边界 |
| `docs/zhuhu/phase0-acceptance-record.md` | 本文档 |
| `internal/auth/context.go` | 统一身份上下文 |
| `internal/auth/contract.go` | API 契约结构体 |
| `internal/auth/bootstrap.go` | Bootstrap 管理员 |
| `internal/auth/auth_test.go` | 自动化测试 |
| `internal/config/config.go` | AuthConfig 配置 |
| `cmd/rag-server/main.go` | Admin 门禁 + Bootstrap 调用 |
| `api/handler/rag/retrieve.go` | Legacy 标记 |
| `pkg/ragsdk/README.md` | SDK 接入文档 |
| `scripts/test-retrieve.sh` | 测试脚本 |

## 风险与回滚

### 回滚方式

每个阶段都可以独立回滚：

```bash
# 回滚 L1（Admin 门禁）
git revert 658b7c0

# 回滚 L2（Bootstrap）
git revert 4466344

# 回滚 L3/L4/L5（统一上下文 + API契约 + 白名单）
git revert 92f4d7c

# 回滚 L6/L7（SDK + 测试）
git revert 66e95d4
```

### 兼容性

- 现有 `/api/admin/*` 路由在 dev 环境继续工作
- 现有 `/v1/retrieve` 的 legacy app_id 继续工作
- 现有 SDK 调用方式不受影响

## Phase 1 入口条件

Phase 1 可以直接启动，不需要额外准备：

1. `auth/context.go` 已定义统一身份上下文
2. `auth/contract.go` 已定义 API 契约
3. `config.go` 已定义 AuthConfig
4. `main.go` 已有 Admin 门禁和 Bootstrap 调用点
5. 自动化测试框架已建立

## 下一步

Phase 1：基础认证（注册/登录/JWT）
- 创建 `rag_tenant` 和 `rag_user` 表
- 实现注册/登录 API
- 实现 JWT Token 生成和验证
- 将现有 Admin 注入替换为真实 JWT

# Phase 4 灰度与回滚清单

## 灰度顺序

1. 本地完成 owner 登录、API Key 创建、租户用量页和接入文档页检查。
2. 测试环境开启 `/login`、`/register`。
3. 测试环境开启 `/api-keys`、`/tenant/settings`、`/tenant/usage`、`/docs/integration`。
4. 测试环境执行：
   - `backend/scripts/test-retrieve.sh`
   - `backend/scripts/smoke/phase4-agent-retrieve.ps1`
5. staging 对内部租户开放。
6. 小范围真实 Agent 项目试用。

## 回滚顺序

1. 隐藏 `/docs/integration` 入口。
2. 隐藏 API Key 轮换按钮。
3. 保留 API Key 列表，只关闭创建入口。
4. 保留真实 JWT 鉴权，不回退到默认 admin 后门。
5. 用量页异常时显示契约缺口提示，不阻断其他页面。

## 不可回滚的安全底线

- 不恢复默认 admin 后门。
- 不在前端保存 API Key 明文。
- 不绕过 JWT 访问 Admin API。
- 不允许错误 API Key 降级回 legacy。
- 不允许 `/v1/retrieve` 信任前端传入 `tenant_id`。

## 观察指标

- 登录失败率
- refresh 失败率
- API Key 创建失败率
- API Key 轮换失败率
- `/v1/retrieve` 的 401 / 403 / 429 比例
- revoked Key 是否仍可调用
- 检索日志字段是否包含 `tenant_id/app_id/api_key_id/auth_type/source_api`

## 验收材料建议

- Admin 登录页截图
- API Key 一次性明文展示截图
- API Key 轮换成功截图
- 租户设置页截图
- 用量页截图
- 接入文档页截图
- smoke 脚本输出
- 检索日志详情截图

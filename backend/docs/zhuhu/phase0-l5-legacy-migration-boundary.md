# L5 旧 app_id 白名单迁移边界

## 当前状态
- `/v1/retrieve` 使用静态 `allowedAppIDs` 白名单
- 3 个硬编码的 app_id：`interview-agent`, `mianshiba-web`, `mianshiba-admin`

## 迁移边界
1. Phase 0：标记 `auth_type=legacy_app_id`，日志区分
2. Phase 2：引入 API Key，旧白名单继续工作
3. Phase 3：旧白名单降级为兼容路径，新用户必须用 API Key
4. Phase 4：旧白名单下线（可选）

## 兼容策略
- 旧请求继续工作，日志标记 `is_legacy=true`
- 新请求优先使用 API Key
- 日志能区分 `legacy_app_id` 和 `api_key`

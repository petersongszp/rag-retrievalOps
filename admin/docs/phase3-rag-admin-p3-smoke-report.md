# Phase 3 P3 Smoke Report

## Scope

- Phase 3 admin/backend implementation through `L0-L7`
- Automated smoke verification for:
  - retrieval debug route and strategy center route
  - strategy control APIs
  - retrieval debug structured contract rendering
  - frontend production build

## Completed Commits

- `2c53a6b` 冻结P3策略契约与调试路由口径
- `3a6d350` 实现高级检索调试结构化接口
- `4273753` 实现策略开关版本与回滚接口
- `3f9b2cc` 实现策略影响分析与操作日志接口
- `7e7d8eb` 实现前端调试视图与策略中心骨架
- `7dd2cee` 实现检索调试视图页面主体
- `fd17180` 打通调试视图入口并增强日志摘要
- `ddd0e45` 实现策略中心控制台页面

## Verification Commands

### Backend

```powershell
cd backend
go test ./internal/rag/phase3admin
go test ./internal/service/kb ./api/router
go test ./api/handler/kb -run 'TestStrategyImpactAndGatesEndpoints|TestStrategyOperationsEndpointReturnsLatestChanges|TestStrategyHandlersLifecycle|TestListStrategyVersionsRejectsInvalidFlagKey|TestBuildRetrievalDebugTraceResponse|TestBuildRetrievalDebugTraceResponseFallbackMarksContractGaps|TestBuildRetrieveDebugTraceIncludesParentFillDiff|TestClassifyRewriteGainBucket'
```

Result:

- Passed.

### Frontend

```powershell
cd admin
npm run build
```

Result:

- Passed.
- Existing repository-wide Prettier/CRLF warnings remain, but they do not block compilation.

## Smoke Checklist Status

| Item | Status | Notes |
| --- | --- | --- |
| `/retrieval-lab` route available | PASS | existing route preserved |
| `/retrieval-lab/debug?request_id=xxx` route available | PASS | built and linked from lab/logs/report |
| `/trace-logs/retrieval` can enter debug view | PASS | list action + drawer jump available |
| evaluation failure samples can enter debug view | PASS | debug-first link path added, trace logs fallback preserved |
| `/strategy-center` route available | PASS | built and navigable from sidebar |
| strategy flags / impact / gates / operations consume API successfully | PASS | frontend build + backend handler tests passed |
| strategy edit / rollback UI present | PASS | strategy center modal actions implemented |
| backend structured retrieval debug response available | PASS | handler tests passed |
| frontend local degradation on contract gaps | PASS | debug page and strategy center both surface contract gaps |

## Known Environment Limits

- Full `go test ./api/handler/kb` still depends on local MySQL at `127.0.0.1:3307`; this environment-specific suite was not used as a release gate.
- Browser-level manual click testing was not recorded in this report; route wiring was verified by source integration plus `next build`.

## Phase 4 Handoff Notes

- Strategy operation logs already exist and can be expanded into full audit trails.
- Strategy impact APIs already return `contract_gaps`, so later platform work can extend metrics without forcing fake zero values.
- Retrieval debug view already handles partial contracts stage by stage, which is suitable for future audit/quality report expansion.
- Current backend contract still lacks some rich debug fields requested by the roadmap, notably:
  - query rewrite `term_hits`
  - query rewrite `rewrite_strategy`
  - query rewrite `rewrite_gain_bucket`
  - persisted `topk_policy_version` in retrieval log detail drawer
- These gaps are surfaced as UI contract gaps instead of silently fabricated values.

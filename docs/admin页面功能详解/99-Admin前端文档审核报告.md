# 99-Admin 前端文档审核报告

## 一、审核结论

本轮 Admin 前端使用说明文档已完成。

- 已按左侧菜单顺序完成总规划与模块文档。
- 已覆盖当前 Admin 前端源码中的主要页面、入口、按钮、表格、弹窗、抽屉、详情页和可展开内容。
- 已统一写入：`docs/admin前端使用说明/`
- 已修正一次生成过程中的中文文件名乱码问题，最终文件名为中文规范命名。
- 文档语言整体面向小白，优先解释“这个参数有什么用、怎么看、什么时候要关注”。

## 二、审核范围

源码依据主要包括：

- `admin/src/components/admin/admin-shell.tsx`
- `admin/src/components/admin/knowledge-base-provider.tsx`
- `admin/src/components/admin/dashboard-page.tsx`
- `admin/src/components/admin/knowledge-bases-page.tsx`
- `admin/src/components/admin/knowledge-base-detail-page.tsx`
- `admin/src/components/admin/create-knowledge-base-modal.tsx`
- `admin/src/components/admin/retrieval-lab-page.tsx`
- `admin/src/components/admin/retrieval-debug-page.tsx`
- `admin/src/components/admin/retrieval-logs-page.tsx`
- `admin/src/components/admin/ingest-logs-page.tsx`
- `admin/src/components/admin/evaluation-datasets-page.tsx`
- `admin/src/components/admin/evaluation-runs-page.tsx`
- `admin/src/components/admin/evaluation-report-page.tsx`
- `admin/src/components/admin/quality-monitor-page.tsx`
- `admin/src/components/admin/strategy-center-page.tsx`
- `admin/src/components/admin/cost-ops-cost-page.tsx`
- `admin/src/components/admin/vector-db-page.tsx`
- `admin/src/components/admin/audit-page.tsx`
- `admin/src/components/admin/alerts-page.tsx`
- `admin/src/components/admin/weekly-reports-page.tsx`
- `admin/src/components/admin/vector-ops-page.tsx`
- `admin/src/types/kb.ts`
- `admin/src/config/api.ts`

## 三、文档清单

| 编号 | 文档 | 覆盖页面/模块 | 审核状态 |
|---|---|---|---|
| 00 | `00-写作总规划.md` | 写作顺序、分工、审核标准 | 通过 |
| 00 | `00-Admin前端总览与通用操作.md` | Admin 外壳、左侧菜单、顶部知识库选择、面包屑、刷新 | 通过 |
| 01 | `01-概览使用说明.md` | Dashboard 概览、统计卡片、近期失败/趋势等 | 通过 |
| 02 | `02-知识库使用说明.md` | 知识库列表、创建知识库弹窗、知识库详情、文档上传、入库任务 | 通过 |
| 03 | `03-检索实验室使用说明.md` | 检索实验室、调试表单、结果区、保存样本、调试详情页 | 通过 |
| 04 | `04-链路日志使用说明.md` | 检索日志、入库日志、筛选、详情抽屉 | 通过 |
| 05 | `05-评测使用说明.md` | 评测集、评测运行、评测报告、profiles、gate thresholds、样本展开 | 通过 |
| 06 | `06-质量监控使用说明.md` | 最近成功评测摘要、Delta 指标、Gate、贡献摘要 | 通过 |
| 07 | `07-策略中心使用说明.md` | Feature Flags、Impact、Gate、版本、操作日志、修改/回滚弹窗 | 通过 |
| 08 | `08-成本运营使用说明.md` | 成本看板、Vector DB 页面 | 通过 |
| 09 | `09-审计中心使用说明.md` | 审计列表、详情抽屉 | 通过 |
| 10 | `10-告警中心使用说明.md` | 告警列表、确认、解决 | 通过 |
| 11 | `11-报告周报使用说明.md` | 周报列表、生成周报、详情抽屉 | 通过 |
| 12 | `12-向量运维使用说明.md` | Vector Ops、active collection、registry、operations、health、activate、rollback | 通过 |

## 四、重点审核结果

### 1. 左侧导航顺序

文档顺序已按 `admin-shell.tsx` 中的左侧导航排列：

1. 概览
2. 知识库
3. 检索实验室
4. 链路日志
5. 评测
6. 质量监控
7. 策略中心
8. 成本运营
9. 审计中心
10. 告警中心
11. 报告/周报

另外，源码中存在 `/vector-ops` 路由和页面组件，但当前不在左侧菜单里，因此单独作为 `12-向量运维使用说明.md` 补充说明。

### 2. 弹窗/抽屉/展开内容

已重点核对以下交互内容：

- 创建知识库弹窗
- 知识库详情页上传文档、入库任务操作
- 检索实验室保存评测样本弹窗
- 检索调试详情页中的 trace、route、fusion、fallback、evidence、citation 等区域
- 检索日志和入库日志详情抽屉
- 评测集创建、样本导入/导出、样本展开详情
- 评测运行创建弹窗、运行详情抽屉
- 评测报告中的失败样本展开、指标、Gate、贡献分析
- 策略中心修改策略弹窗、回滚弹窗
- Vector DB 健康详情弹窗
- 审计详情抽屉
- 周报详情抽屉
- 向量运维 Health 弹窗、Activate、Rollback 操作

### 3. 参数解释

各文档均已覆盖主要参数含义，包括但不限于：

- 知识库 ID、名称、描述、文档数量、入库状态
- 检索参数、topK、query、trace、route、fusion、fallback
- 评测集、样本、profiles、gate thresholds、运行状态、报告指标
- 策略状态 enabled/disabled/shadow/canary/rolling_back/error、rollout、risk、impact、gate
- 成本时间范围、Token、query 成本、高成本查询
- Vector DB collection、health、active、operation
- 审计 operation、actor、resource、metadata
- 告警 severity、status、category、acknowledge、resolve
- 周报 window、risks、actions、quality summary

## 五、如实标注的源码限制

文档中已明确标注这些“当前前端没有实现/只展示部分能力”的地方，避免误导新人：

1. `vector-ops` 页面存在，但当前不在左侧菜单中，需要直接访问路由。
2. 成本看板源码未展示 `COST_BY_STRATEGY` 对应的策略维度汇总，文档中已说明。
3. 审计中心当前没有筛选控件和分页控件，主要是列表 + 详情抽屉。
4. 告警中心当前没有筛选控件、分页控件和独立详情页。
5. 周报页面 API 中有详情/导出能力，但当前页面没有单独详情接口调用和导出按钮。
6. 质量监控页面当前不是完整趋势大屏，主要展示最近成功评测、Delta、Gate、贡献摘要。
7. 策略中心 Phase2 Baseline 回滚的具体冻结顺序由后端控制，前端只展示“按后端冻结顺序做全量回滚”。
8. 评测 profiles 和 gate thresholds 前端主要做 JSON 格式校验，业务合法性由后端判断。

## 六、最终建议

这套文档已经可以作为 Admin 前端新人入门说明使用。

如果后续 Admin 页面继续迭代，建议同步维护：

- 新增菜单时，新建对应编号文档；
- 新增弹窗/抽屉/展开项时，补充“参数说明”和“常见流程”；
- 若接口返回字段变化，优先更新字段解释；
- 若页面只是骨架或占位，继续保持“如实说明当前限制”的写法。

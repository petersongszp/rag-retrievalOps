# RAG 管理台前端功能优化实现大纲

## 1. 文档定位

本文档承接 `admin/docs/rag-admin-platform-ui-optimization-plan.md`，用于把“前端体验与视觉平台化优化方案”拆成可执行的功能实现大纲。

本次优化的目标不是新增一批后端能力，而是在现有 `admin` 前端基础上，把页面从“功能验证后台”升级为“可交付给真实用户使用的平台型管理台”。

三个用途：

1. 作为前端优化开发的任务拆解基线，后续可按 `L0 -> L8` 逐步实现。
2. 作为产品、前端、后端协作时的文案、页面、状态和验收口径。
3. 作为后续代码改造、测试验收、截图检查和回归检查的依据。

---

## 2. 当前基线

### 2.1 已有稳定能力

1. `admin` 已完成多页面管理台骨架，包含 `AdminShell`、侧边导航、顶部栏、面包屑和知识库选择器。
2. 已有登录注册、权限状态、租户上下文和账号菜单。
3. 已有核心业务页面：
   - `/dashboard`
   - `/knowledge-bases`
   - `/knowledge-bases/[kbId]`
   - `/retrieval-lab`
   - `/retrieval-lab/debug`
   - `/trace-logs/retrieval`
   - `/trace-logs/ingest`
   - `/evaluation/datasets`
   - `/evaluation/runs`
   - `/evaluation/reports/[runId]`
   - `/quality-monitor`
   - `/strategy-center`
   - `/cost-ops/cost`
   - `/cost-ops/vector-db`
   - `/audit`
   - `/alerts`
   - `/reports/weekly`
   - `/api-keys`
   - `/tenant/settings`
   - `/tenant/usage`
   - `/docs/integration`
4. 已复用 `Ant Design 5` 和 `Tailwind CSS`，不需要重新引入设计系统框架。
5. 已有 `admin/src/config/api.ts`、`admin/src/types/kb.ts` 和 `apiClient`，本次前端优化优先复用现有接口与类型。

### 2.2 主要问题

1. 页面仍有内部研发阶段痕迹：`Phase`、`P3`、`P4`、`Phase2 Baseline`、`Feature Flags`、`debug_available` 等。
2. 部分页面文案偏测试语气：`检索实验室`、`运行检索测试`、`检查契约字段`、`debug trace`。
3. 专业指标缺少中文解释：`Recall@K`、`MRR`、`nDCG`、`P95 Latency`、`Gate`、`Baseline`、`Candidate`。
4. 视觉表现仍偏 Ant Design 默认后台，缺少统一的平台品牌、页面结构、指标卡、空状态和错误状态。
5. 首页更像指标集合，缺少工作台应有的状态总览、待处理事项和推荐动作。
6. 高级页面可用但门槛偏高，普通用户很难判断“质量是否变好”“策略能不能开”“成本为什么升高”。

---

## 3. 范围边界

### 3.1 必须完成

1. 全局品牌和导航改造：从 `RAG Admin` 过渡到 `智能知识库管理平台`。
2. 用户可见文案产品化：移除主路径上的研发阶段词和测试语气。
3. 工作台重构：让 `/dashboard` 能回答系统状态、风险事项和下一步动作。
4. 核心页面视觉统一：页面头、指标卡、筛选区、表格、状态标签、空状态、错误状态。
5. 专业术语汉化：对质量、检索、策略、成本、审计相关术语给出中文名和解释。
6. 检索调优、链路分析、质量评测、策略管理、成本治理等页面完成产品化包装。
7. 建立后续维护规则：新增页面不得继续暴露测试文案和研发阶段文案。

### 3.2 明确不做

1. 不改动后端 API 语义。
2. 不用假数据伪装能力已完整上线。
3. 不把管理台做成营销首页。
4. 不引入新的大型 UI 框架。
5. 不移除高级字段和专业能力，只降低默认展示门槛。
6. 不在本阶段重写全部页面逻辑，优先做低风险的结构、文案、样式和组件抽象。

---

## 4. 总体验收 Gate

本轮前端优化完成后，需要满足以下 Gate：

1. 侧边导航、面包屑、页面标题和主要按钮中不出现 `Phase`、`P3`、`P4`、`debug`、`test`、`Feature Flags`、`Baseline` 等内部口径。
2. `/dashboard` 首屏能清楚展示：
   - 当前知识库上下文
   - 系统健康状态
   - 核心指标
   - 待处理事项
   - 快捷操作
3. `/retrieval-lab` 用户看到的是“检索调优 / 检索验证”，不是“测试工具”。
4. `/retrieval-lab/debug` 用户看到的是“链路分析”，不是裸露的 debug 字段集合。
5. `/quality-monitor` 和 `/evaluation/reports/[runId]` 能给出清晰质量结论，而不仅是算法指标。
6. `/strategy-center` 的策略开关、灰度、回退、风险说明使用中文业务表达。
7. 成本、审计、告警、周报页面统一为治理能力，不再像临时功能入口。
8. 所有空状态和错误状态至少包含一个下一步动作。
9. `npm run test --prefix admin` 通过；如果仅做样式文案且部分测试需同步更新，必须更新对应断言。
10. 人工检查桌面宽度和窄屏宽度，页面无明显文字重叠、按钮溢出和表格不可操作问题。

---

## 5. 路线总览

推荐按以下顺序实现：

| 路线 | 目标 | 主要文件 |
|---|---|---|
| L0 | 全局品牌、导航和文案基线 | `admin-shell.tsx`、`auth-shell.tsx`、`layout.tsx` |
| L1 | 视觉基础与公共组件 | `globals.css`、`tailwind.config.js`、新增 `components/admin/ui/*` |
| L2 | 工作台重构 | `dashboard-page.tsx` |
| L3 | 知识库管理产品化 | `knowledge-bases-page.tsx`、`knowledge-base-detail-page.tsx`、`create-knowledge-base-modal.tsx` |
| L4 | 检索调优与链路分析 | `retrieval-lab-page.tsx`、`retrieval-debug-page.tsx` |
| L5 | 链路追踪和日志排障 | `retrieval-logs-page.tsx`、`ingest-logs-page.tsx` |
| L6 | 质量评测降门槛 | `quality-monitor-page.tsx`、`evaluation-*.tsx` |
| L7 | 策略、成本、审计、告警治理化 | `strategy-center-page.tsx`、`cost-ops-cost-page.tsx`、`audit-page.tsx`、`alerts-page.tsx`、`weekly-reports-page.tsx` |
| L8 | 验收、测试和维护规则固化 | `__tests__`、文档、截图检查清单 |

---

## 6. 详细路线拆解

### L0 全局品牌、导航和文案基线

**目标：** 先把全局框架从内部后台口径改为正式平台口径。

**任务：**

1. 修改 `admin/src/components/admin/admin-shell.tsx`：
   - `RAG Admin` 改为 `智能知识库管理平台`。
   - `知识库与评测管理台` 改为 `知识资产、检索质量与运营治理`。
   - `概览` 改为 `工作台`。
   - `检索实验室` 改为 `检索调优`。
   - `调试详情` 改为 `链路分析`。
   - `链路日志` 改为 `链路追踪`。
   - `评测` 改为 `质量评测`。
   - `评测集` 改为 `评测样本`。
   - `评测运行` 改为 `评测任务`。
   - `策略中心` 改为 `策略管理`。
   - `成本运营` 改为 `成本与运维`。
   - `成本看板` 改为 `成本分析`。
   - `Vector DB` 改为 `向量库运维`。
   - `API Keys` 改为 `接入密钥`。
   - `租户` 改为 `组织`，如果后端仍使用 `tenant`，仅前端展示改中文。
2. 移除侧边栏底部 `Phase 4` 文案，替换为：
   - 当前平台状态
   - 帮助入口
   - 接入指南入口
3. 修改 `admin/src/components/auth/auth-shell.tsx`：
   - 移除 `Phase 4 Admin Access`。
   - 登录页介绍改为“使用账号进入智能知识库管理平台”。
   - 删除“开发期默认入口”相关表达。
4. 修改 `admin/src/app/layout.tsx`：
   - `metadata.title` 改为 `智能知识库管理平台`。
   - `metadata.description` 改为 `知识库、检索质量与运营治理平台`。

**验收：**

1. 全局导航和登录页不出现 `Phase`、`P3`、`P4`。
2. 面包屑使用新中文命名。
3. 页面跳转路径保持不变，不影响现有路由。

---

### L1 视觉基础与公共组件

**目标：** 先沉淀轻量设计系统，避免每个页面各改各的。

**任务：**

1. 调整 `admin/tailwind.config.js`：
   - 主色建议统一为 `#2563EB`。
   - 中性色统一为 `slate` 系列。
   - 保留 `success / warning / error / info` 语义色。
2. 调整 `admin/src/styles/globals.css`：
   - 统一 body 背景为 `#f8fafc`。
   - 增加页面容器、页面头、指标卡、状态点、说明文本等通用 class。
   - 保持卡片圆角在 `6px - 8px`，减少大圆角。
3. 新增公共 UI 组件目录：`admin/src/components/admin/ui/`。
4. 建议首批抽象组件：
   - `PageHeader`：统一页面标题、副标题、右侧操作。
   - `MetricCard`：统一指标卡。
   - `StatusBadge`：统一状态标签。
   - `ActionEmpty`：带下一步操作的空状态。
   - `InlineHelp`：术语解释 tooltip。
   - `RiskConfirmModal`：危险操作确认弹窗。
5. 页面先逐步接入公共组件，不要求一次性替换所有页面。

**验收：**

1. 新组件不改变业务接口和数据结构。
2. 首页、检索调优、策略管理至少接入 `PageHeader` 或等价结构。
3. 空状态不再只有“暂无数据”，至少有说明和动作。

---

### L2 工作台重构

**目标：** 把 `/dashboard` 从指标堆叠改成平台工作台。

**任务：**

1. 修改 `admin/src/components/admin/dashboard-page.tsx` 页面结构：
   - 顶部：工作台标题、当前知识库、刷新时间、刷新按钮。
   - 第一屏：系统状态条。
   - 第二块：核心资产指标。
   - 第三块：待处理事项。
   - 第四块：快捷操作。
   - 第五块：趋势图和风险分布。
2. 保留现有接口：
   - `KB_ADMIN_API.DASHBOARD_STATS`
   - `KB_ADMIN_API.METRICS_OVERVIEW`
3. 指标重命名：
   - `知识库` -> `知识库数量`
   - `文档` -> `文档总数`
   - `处理中任务` -> `处理中入库任务`
   - `失败任务` -> `失败入库任务`
   - `检索 P95` -> `P95 响应耗时`
   - `每千次问答成本` -> `每千次问答成本`
4. 新增“待处理事项”计算逻辑：
   - `failed_job_count > 0`：提示处理失败入库任务。
   - `latestEmptyRate` 高于阈值：提示检查空结果率。
   - `latestP95` 高于阈值：提示检查响应耗时。
   - `error_type_topn.length > 0`：提示查看错误类型。
5. 快捷操作：
   - 新建知识库
   - 上传文档
   - 开始检索验证
   - 查看质量报告
   - 创建接入密钥

**验收：**

1. 工作台首屏能看出系统是否正常。
2. 指标解释为中文业务表达。
3. 点击快捷操作能跳转到现有页面。
4. 空数据时显示入门引导，而不是指标全为 0 的冷冰冰页面。

---

### L3 知识库管理产品化

**目标：** 让知识库页面像资产管理工具，而不是上传文件测试页。

**任务：**

1. 修改 `create-knowledge-base-modal.tsx`：
   - 示例从“Go 面试指南”扩展为更正式的业务示例。
   - 增加说明：知识库用于组织文档、生成向量并支持检索问答。
2. 修改 `knowledge-bases-page.tsx`：
   - 页面标题改为 `知识库管理`。
   - 空状态增加“创建第一个知识库”按钮。
   - 状态字段统一展示为：可用、处理中、异常、停用。
3. 修改 `knowledge-base-detail-page.tsx`：
   - 文档区说明上传后的处理流程。
   - 入库任务阶段汉化：解析中、切分中、向量化中、写入中、完成、失败。
   - 失败任务增加下一步动作：重试、查看原因、复制错误编号。
4. 上传相关文案：
   - `上传` 改为 `上传文档`。
   - `处理任务` 改为 `入库任务`。
   - `打开检索实验室` 改为 `进入检索调优`。

**验收：**

1. 新用户能理解知识库、文档、入库任务之间的关系。
2. 文档失败时能看到原因和下一步。
3. 页面不出现“测试文档”“测试数据”等可见文案。

---

### L4 检索调优与链路分析

**目标：** 保留检索验证和调试能力，但把用户心智从“测试工具”改成“效果调优工具”。

**任务：**

1. 修改 `retrieval-lab-page.tsx`：
   - 页面标题改为 `检索调优`。
   - 副标题改为“输入问题，查看相关结果、引用来源和本次检索链路编号。”
   - `运行检索测试` 改为 `开始检索验证`。
   - `查看调试视图` 改为 `查看链路分析`。
   - `保存为评测样本` 改为 `加入质量样本`。
   - `Contract Gap` 或缺失字段相关区域改为 `返回信息完整性检查`，默认折叠。
2. 修改检索结果展示：
   - `score` 展示为 `相关度`。
   - `citation` 展示为 `引用来源`。
   - `source.route` 展示为 `召回路线`。
   - `retriever_version` 展示为 `检索版本`。
3. 修改 `retrieval-debug-page.tsx`：
   - 页面标题改为 `检索链路分析`。
   - `debug_available` 展示为 `链路明细状态`。
   - `request_id` 展示为 `请求编号`。
   - 阶段命名汉化：查询改写、召回路线、结果融合、去重、重排、过滤、引用检查、最终结果。
   - 没有 `request_id` 时提示“请输入请求编号，或从检索调优、链路追踪、评测报告进入。”
4. 保留原始字段：
   - 原始接口字段可放入“高级信息”折叠区。
   - 不在主视觉区展示裸字段名。

**验收：**

1. 用户主路径不出现 `debug trace`、`contract`、`test`。
2. 高级用户仍可通过请求编号进入完整链路分析。
3. 检索结果能看懂相关度、引用来源和召回路线。

---

### L5 链路追踪和日志排障

**目标：** 把日志页从开发日志列表改成正式排障工具。

**任务：**

1. 修改 `retrieval-logs-page.tsx`：
   - 页面标题改为 `检索链路追踪`。
   - 搜索提示强调“用请求编号定位一次检索”。
   - 表头汉化：请求编号、问题、知识库、返回数量、耗时、状态、创建时间。
   - `P3 summary fields are not fully available yet` 改为 `高级链路信息暂未完整返回`。
   - `P3 Summary` 改为 `高级链路摘要`。
2. 修改 `ingest-logs-page.tsx`：
   - 页面标题改为 `入库链路追踪`。
   - 阶段字段汉化为文档处理流程。
   - 错误信息提供“复制错误编号”和“查看任务详情”。
3. 链路详情展示层级：
   - 基础信息
   - 阶段耗时
   - 结果摘要
   - 错误原因
   - 高级字段

**验收：**

1. 用户能用请求编号或任务编号定位问题。
2. 日志页不再像原始接口字段表。
3. 字段缺失时明确提示“当前版本暂未返回”，不展示英文内部说明。

---

### L6 质量评测降门槛

**目标：** 让质量评测结果能被非算法用户理解。

**任务：**

1. 修改 `quality-monitor-page.tsx`：
   - 页面标题改为 `质量看板`。
   - `Run ID` -> `评测任务编号`
   - `Generated At` -> `生成时间`
   - `Baseline` -> `对照版本`
   - `Candidate` -> `候选版本`
   - `Gate` -> `发布门禁`
   - `passed / failed` -> `通过 / 未通过`
   - `Delta` 指标增加中文解释。
2. 修改 `evaluation-datasets-page.tsx`：
   - `Dataset` -> `评测样本`
   - 导入说明改为“用于持续验证检索质量的标准问题集合”。
3. 修改 `evaluation-runs-page.tsx`：
   - `Run` -> `评测任务`
   - 创建任务时说明对照版本和候选版本。
4. 修改 `evaluation-report-page.tsx`：
   - 报告顶部增加结论区：建议上线 / 建议继续观察 / 不建议上线。
   - 指标展示中文名：
     - 召回率@K
     - 首个正确结果排名
     - 排序质量
     - 引用准确率
     - P95 响应耗时
   - 失败样本增加“可能原因”和“建议动作”。

**验收：**

1. 评测报告能直接说明质量是变好还是变差。
2. 非算法用户能理解每个指标大概含义。
3. 原始指标值保留，方便研发排查。

---

### L7 策略、成本、审计、告警治理化

**目标：** 把高级管理页面统一包装为正式治理能力。

**任务：**

1. 修改 `strategy-center-page.tsx`：
   - `策略中心` -> `策略管理`。
   - `Feature Flags` -> `策略开关`。
   - `Canary` -> `小流量试用`。
   - `Error 策略数` -> `异常策略`。
   - `Impact 分析` -> `策略影响分析`。
   - `回滚到 Phase2 Baseline` -> `回退到稳定检索策略`。
   - 高风险策略启用前增加风险确认说明。
2. 修改 `cost-ops-cost-page.tsx`：
   - 页面标题改为 `成本分析`。
   - `High Cost Query` -> `高成本请求`。
   - `Request ID` -> `请求编号`。
   - `Tokens` -> `模型用量`，括号中保留 `tokens`。
   - 高成本请求提供进入链路分析入口。
3. 修改 `vector-ops-page.tsx` 或 `vector-db-page.tsx`：
   - `Vector DB` -> `向量库运维`。
   - `Collection` -> `向量集合`。
   - 健康、容量、索引、active 状态使用统一状态标签。
4. 修改 `audit-page.tsx`：
   - 字段名统一为：操作人、操作对象、操作内容、发生时间、来源 IP、关联请求。
   - 原始 JSON 或 before/after 放到高级详情。
5. 修改 `alerts-page.tsx`：
   - 告警按质量、成本、容量、安全分组。
   - 每条告警提供确认、解决、查看详情、跳转关联页面。
6. 修改 `weekly-reports-page.tsx`：
   - 页面定位改为 `周期运营复盘`。
   - 内容区统一为质量、稳定性、成本、策略变化、告警复盘、建议动作。

**验收：**

1. 高级页面看起来是长期运营工具，不是临时排查入口。
2. 风险操作必须有二次确认和影响范围说明。
3. 所有页面专业字段都有中文展示名。

---

### L8 测试、验收和维护规则固化

**目标：** 防止优化后回归到测试型页面。

**任务：**

1. 更新现有测试断言：
   - `dashboard-page.test.tsx`
   - `retrieval-debug-page.test.tsx`
   - `retrieval-logs-page.test.tsx`
   - `strategy-center-page.test.tsx`
   - `cost-ops-cost-page.test.tsx`
   - `knowledge-base-detail-page.test.tsx`
2. 增加轻量文案检查：
   - 主路径页面不得出现 `Phase 3`、`Phase 4`、`P3`、`P4`。
   - 主按钮不得出现 `测试`、`debug`、`Feature Flags`。
3. 增加人工验收清单：
   - 登录页
   - 工作台
   - 知识库管理
   - 检索调优
   - 链路分析
   - 质量看板
   - 策略管理
   - 成本分析
   - 操作审计
   - 风险告警
4. 执行验证命令：
   - `npm run test --prefix admin`
   - `npm run build --prefix admin`

**验收：**

1. 测试通过或已记录明确阻塞原因。
2. 文案检查无主路径违规词。
3. 关键页面人工验收通过。

---

## 7. 推荐执行节奏

### 第一批：低风险快速见效

建议先做：

1. L0 全局品牌、导航和文案基线。
2. L1 视觉基础中的颜色、背景、页面头规范。
3. L4 检索调优主文案替换。
4. L7 策略管理里的 `Feature Flags / Phase2 Baseline` 替换。

原因：

1. 主要是文案和局部样式，风险低。
2. 能快速消除“测试后台”的第一印象。
3. 对后端无依赖。

### 第二批：平台感核心

建议随后做：

1. L2 工作台重构。
2. L3 知识库管理产品化。
3. L5 链路追踪和日志排障。

原因：

1. 工作台决定第一眼平台感。
2. 知识库和检索是用户主路径。
3. 链路追踪能承接排障和质量分析。

### 第三批：高级能力降门槛

建议最后做：

1. L6 质量评测降门槛。
2. L7 成本、审计、告警、周报治理化。
3. L8 测试和维护规则固化。

原因：

1. 高级页面字段多，测试断言可能更多。
2. 需要更细的术语统一。
3. 适合在前两批风格稳定后统一收口。

---

## 8. 首批文件修改清单

第一批建议直接进入开发的文件：

1. `admin/src/components/admin/admin-shell.tsx`
2. `admin/src/components/auth/auth-shell.tsx`
3. `admin/src/app/layout.tsx`
4. `admin/src/styles/globals.css`
5. `admin/tailwind.config.js`
6. `admin/src/components/admin/dashboard-page.tsx`
7. `admin/src/components/admin/retrieval-lab-page.tsx`
8. `admin/src/components/admin/retrieval-debug-page.tsx`
9. `admin/src/components/admin/strategy-center-page.tsx`
10. `admin/src/components/admin/quality-monitor-page.tsx`

可新增的公共组件：

1. `admin/src/components/admin/ui/page-header.tsx`
2. `admin/src/components/admin/ui/metric-card.tsx`
3. `admin/src/components/admin/ui/action-empty.tsx`
4. `admin/src/components/admin/ui/status-badge.tsx`
5. `admin/src/components/admin/ui/inline-help.tsx`
6. `admin/src/components/admin/ui/risk-confirm-modal.tsx`

---

## 9. 回退与降级策略

1. 文案替换可直接回退到单文件修改，不影响接口。
2. 公共组件引入必须逐页替换，避免一次性影响所有页面。
3. 工作台重构保留原有 API 调用和指标计算逻辑，若新布局出现问题，可恢复原卡片区域。
4. 高级页面的原始字段不删除，只移动到高级信息区，便于排查。
5. 视觉主题调整如果影响 Ant Design 默认组件可读性，优先回退颜色变量，不回退业务文案。

---

## 10. 维护规则

1. 新增用户可见页面时，必须使用中文任务命名。
2. 新增专业指标时，必须同时提供中文名和一句解释。
3. 主路径不展示研发阶段词：`Phase`、`P0-P4`、`debug trace`、`mock`、`contract freeze`。
4. 空状态必须说明原因和下一步动作。
5. 危险操作必须有影响范围、二次确认和回退说明。
6. 字段缺失必须显示“暂未返回”或“当前版本暂不支持”，不得显示为 0 或静默隐藏。
7. 页面结构优先复用公共组件，避免后续页面重新长成临时测试界面。


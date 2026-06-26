# zhihangAI 对接 rag-retrievalOps 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `zhihangAI` 增加可切换的外部 RAG provider，使 Agent 能通过 `rag-retrievalOps` 完成知识检索，并保持现有对话、Skill、Tool、Workflow 主链路稳定。

**Architecture:** 保持 `zhihangAI` 作为 Agent 应用层不变，只在 Agent 数据模型、请求结构、检索适配层和 RAG 上下文构建处增加最小侵入改造。`rag-retrievalOps` 继续作为独立 RAG 中台，通过现有 `/v1/retrieve` 接口对外提供检索能力。

**Tech Stack:** Go、Gin、GORM、JSONB、SSE、HTTP API、Vue、TypeScript

---

## 文件结构与职责

### zhihangAI 后端

- 修改：`C:\code\MSZLU-AI\model\agents.go`
  - 为 `Agent` 增加 `ragConfig` 字段与配置结构。
- 修改：`C:\code\MSZLU-AI\app\internal\agents\req.go`
  - 让创建 / 更新 Agent 接口支持 `ragConfig`。
- 修改：`C:\code\MSZLU-AI\app\shared\knowledge.go`
  - 扩展统一检索结果结构，支持 score、sourceName、metadata。
- 修改：`C:\code\MSZLU-AI\app\internal\agents\service.go`
  - 保存 `ragConfig`。
  - 根据 provider 选择本地检索或远程 RetrievalOps。
  - 保持现有 SSE 聊天主链路不变。
- 创建：`C:\code\MSZLU-AI\app\internal\integrations\retrievalops\client.go`
  - 负责调用 `rag-retrievalOps` 的 `/v1/retrieve`。
- 创建：`C:\code\MSZLU-AI\app\internal\integrations\retrievalops\types.go`
  - 维护请求与响应结构，避免把外部协议散落在业务代码里。
- 创建：`C:\code\MSZLU-AI\app\internal\integrations\retrievalops\client_test.go`
  - 测试远程检索 client 的成功与失败路径。

### zhihangAI 前端

- 修改：`C:\code\MSZLU-AI\frontend\src\types\agent.ts`
  - 增加 `ragConfig` 类型。
- 修改：`C:\code\MSZLU-AI\frontend\src\api\*.ts`
  - 让 Agent 创建 / 更新请求包含 `ragConfig`。
- 修改：`C:\code\MSZLU-AI\frontend\src\views\AgentManagement.vue`
  - 增加 provider、externalKbIds、topK、strategyProfile 配置。
- 如页面拆分存在其他表单组件，则按实际组件路径同步修改。

### 文档

- 修改：`c:\code\rag-retrievalOps\docs\2026-06-26-zhihangAI-rag-integration-design.md`
  - 记录实现完成后的实际落地情况和偏差。
- 创建：`c:\code\rag-retrievalOps\docs\superpowers\plans\2026-06-26-zhihangAI-rag-integration-plan.md`
  - 当前实施计划文档。

---

### Task 1: 定义 Agent 的 RAG 配置模型

**Files:**
- Modify: `C:\code\MSZLU-AI\model\agents.go`
- Modify: `C:\code\MSZLU-AI\app\internal\agents\req.go`

- [ ] **Step 1: 为 `Agent` 定义 `RAGConfig` 结构**

```go
type RAGConfig struct {
	Provider        string   `json:"provider"`
	ExternalKBIDs   []uint64 `json:"externalKbIds"`
	TopK            int      `json:"topK"`
	StrategyProfile string   `json:"strategyProfile"`
	APIKeyRef       string   `json:"apiKeyRef"`
}
```

- [ ] **Step 2: 在 `Agent` 中增加 `ragConfig` 字段**

```go
RAGConfig JSON `json:"ragConfig" gorm:"column:rag_config;type:jsonb"`
```

- [ ] **Step 3: 给 `DefaultAgent` 增加默认值**

```go
RAGConfig: JSON{
	"provider":        "local",
	"externalKbIds":   []uint64{},
	"topK":            4,
	"strategyProfile": "default",
	"apiKeyRef":       "",
},
```

- [ ] **Step 4: 扩展创建 / 更新 Agent 请求结构**

```go
type CreateAgentReq struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      model.AgentStatus `json:"status"`
	Mode        model.AgentMode   `json:"mode"`
	DeepConfig  model.JSON        `json:"deepConfig"`
	RAGConfig   model.JSON        `json:"ragConfig"`
}

type UpdateAgentReq struct {
	ID              uuid.UUID         `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Status          model.AgentStatus `json:"status"`
	SystemPrompt    string            `json:"systemPrompt"`
	ModelProvider   string            `json:"modelProvider"`
	ModelName       string            `json:"modelName"`
	ModelParameters model.JSON        `json:"modelParameters"`
	OpeningDialogue string            `json:"openingDialogue"`
	Mode            model.AgentMode   `json:"mode"`
	DeepConfig      model.JSON        `json:"deepConfig"`
	RAGConfig       model.JSON        `json:"ragConfig"`
}
```

- [ ] **Step 5: 运行最小编译检查**

Run: `go test ./model ./app/internal/agents`
Expected: 通过或仅暴露后续步骤要修复的编译错误。

- [ ] **Step 6: Commit**

```bash
git add C:/code/MSZLU-AI/model/agents.go C:/code/MSZLU-AI/app/internal/agents/req.go
git commit -m "feat: add rag config to agent model"
```

### Task 2: 保存并读取 Agent 的 RAG 配置

**Files:**
- Modify: `C:\code\MSZLU-AI\app\internal\agents\service.go`

- [ ] **Step 1: 在创建 Agent 时写入 `ragConfig`**

```go
if req.RAGConfig != nil {
	agent.RAGConfig = req.RAGConfig
}
```

- [ ] **Step 2: 在更新 Agent 时支持更新 `ragConfig`**

```go
if req.RAGConfig != nil {
	agent.RAGConfig = req.RAGConfig
}
```

- [ ] **Step 3: 抽一个解析方法，统一读取默认值**

```go
func (j JSON) ToRAGConfig() *RAGConfig {
	cfg := &RAGConfig{
		Provider:        "local",
		ExternalKBIDs:   []uint64{},
		TopK:            4,
		StrategyProfile: "default",
		APIKeyRef:       "",
	}
	if provider, ok := j["provider"].(string); ok && provider != "" {
		cfg.Provider = provider
	}
	if topK, ok := j["topK"].(float64); ok && int(topK) > 0 {
		cfg.TopK = int(topK)
	}
	if strategy, ok := j["strategyProfile"].(string); ok && strategy != "" {
		cfg.StrategyProfile = strategy
	}
	if apiKeyRef, ok := j["apiKeyRef"].(string); ok {
		cfg.APIKeyRef = apiKeyRef
	}
	if kbIDs, ok := j["externalKbIds"].([]any); ok {
		for _, id := range kbIDs {
			switch value := id.(type) {
			case float64:
				if value > 0 {
					cfg.ExternalKBIDs = append(cfg.ExternalKBIDs, uint64(value))
				}
			}
		}
	}
	return cfg
}
```

- [ ] **Step 4: 运行编译检查**

Run: `go test ./model ./app/internal/agents`
Expected: 通过。

- [ ] **Step 5: Commit**

```bash
git add C:/code/MSZLU-AI/app/internal/agents/service.go C:/code/MSZLU-AI/model/agents.go
git commit -m "feat: persist agent rag config"
```

### Task 3: 扩展统一检索结果结构

**Files:**
- Modify: `C:\code\MSZLU-AI\app\shared\knowledge.go`
- Modify: `C:\code\MSZLU-AI\app\internal\knowledges\public_service.go`

- [ ] **Step 1: 扩展结果结构**

```go
type SearchKnowledgeBaseResult struct {
	Content    string                 `json:"content"`
	Score      float64                `json:"score,omitempty"`
	SourceName string                 `json:"sourceName,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
```

- [ ] **Step 2: 本地 knowledge 返回值补齐默认字段**

```go
results = append(results, &shared.SearchKnowledgeBaseResult{
	Content:    chunk.Content,
	Score:      0,
	SourceName: "",
	Metadata:   map[string]interface{}{},
})
```

- [ ] **Step 3: 运行相关测试或编译检查**

Run: `go test ./app/shared ./app/internal/knowledges`
Expected: 通过。

- [ ] **Step 4: Commit**

```bash
git add C:/code/MSZLU-AI/app/shared/knowledge.go C:/code/MSZLU-AI/app/internal/knowledges/public_service.go
git commit -m "refactor: unify knowledge search result fields"
```

### Task 4: 新增 RetrievalOps 远程 client

**Files:**
- Create: `C:\code\MSZLU-AI\app\internal\integrations\retrievalops\types.go`
- Create: `C:\code\MSZLU-AI\app\internal\integrations\retrievalops\client.go`
- Create: `C:\code\MSZLU-AI\app\internal\integrations\retrievalops\client_test.go`

- [ ] **Step 1: 定义请求与响应结构**

```go
type RetrieveRequest struct {
	Query           string                 `json:"query"`
	KBIDs           []uint64               `json:"kb_ids"`
	TopK            int                    `json:"top_k"`
	StrategyProfile string                 `json:"strategy_profile,omitempty"`
	MetadataFilter  map[string]interface{} `json:"metadata_filter,omitempty"`
}

type RetrieveResponse struct {
	RequestID       string         `json:"request_id"`
	Items           []RetrieveItem `json:"items"`
	StrategyVersion string         `json:"strategy_version,omitempty"`
}

type RetrieveItem struct {
	Content  string                 `json:"content"`
	Score    float64                `json:"score"`
	Citation map[string]interface{} `json:"citation"`
	Source   map[string]interface{} `json:"source"`
}
```

- [ ] **Step 2: 编写最小 client**

```go
type Client struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		endpoint: strings.TrimRight(baseURL, "/") + "/v1/retrieve",
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}
```

- [ ] **Step 3: 编写调用逻辑**

```go
func (c *Client) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if strings.TrimSpace(c.apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.apiKey))
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("retrievalops status=%d body=%s", resp.StatusCode, string(payload))
	}
	var envelope struct {
		Code int              `json:"code"`
		Data json.RawMessage  `json:"data"`
	}
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(payload, &envelope); err == nil && len(envelope.Data) > 0 {
		var result RetrieveResponse
		if err := json.Unmarshal(envelope.Data, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}
	var result RetrieveResponse
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
```

- [ ] **Step 4: 写成功路径测试**

```go
func TestClientRetrieveSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"request_id":"req-1","items":[{"content":"退款规则","score":0.9,"citation":{"file_name":"refund.md"},"source":{"route":"dense"}}]}}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "test-key", time.Second)
	resp, err := client.Retrieve(context.Background(), &RetrieveRequest{Query: "退款", KBIDs: []uint64{1}, TopK: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("len(resp.Items) = %d", len(resp.Items))
	}
}
```

- [ ] **Step 5: 写失败路径测试**

```go
func TestClientRetrieveHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	client := NewClient(server.URL, "bad-key", time.Second)
	_, err := client.Retrieve(context.Background(), &RetrieveRequest{Query: "退款", KBIDs: []uint64{1}, TopK: 4})
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 6: 运行测试**

Run: `go test ./app/internal/integrations/retrievalops -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add C:/code/MSZLU-AI/app/internal/integrations/retrievalops
git commit -m "feat: add retrievalops integration client"
```

### Task 5: 在 Agent 聊天链路中接入远程检索

**Files:**
- Modify: `C:\code\MSZLU-AI\app\internal\agents\service.go`
- Modify: `C:\code\MSZLU-AI\app\internal\agents\service.go`（`buildRagContext` / `searchKnowledgeBase` 所在位置）

- [ ] **Step 1: 新增远程检索方法**

```go
func (s *service) retrieveFromRetrievalOps(ctx context.Context, message string, cfg *model.RAGConfig) ([]*shared.SearchKnowledgeBaseResult, error) {
	client := retrievalops.NewClient(s.retrievalOpsBaseURL, s.retrievalOpsAPIKey, 10*time.Second)
	resp, err := client.Retrieve(ctx, &retrievalops.RetrieveRequest{
		Query:           message,
		KBIDs:           cfg.ExternalKBIDs,
		TopK:            cfg.TopK,
		StrategyProfile: cfg.StrategyProfile,
	})
	if err != nil {
		return nil, err
	}
	results := make([]*shared.SearchKnowledgeBaseResult, 0, len(resp.Items))
	for _, item := range resp.Items {
		sourceName, _ := item.Citation["file_name"].(string)
		results = append(results, &shared.SearchKnowledgeBaseResult{
			Content:    item.Content,
			Score:      item.Score,
			SourceName: sourceName,
			Metadata: map[string]interface{}{
				"citation":        item.Citation,
				"source":          item.Source,
				"request_id":      resp.RequestID,
				"strategy_version": resp.StrategyVersion,
			},
		})
	}
	return results, nil
}
```

- [ ] **Step 2: 在 `buildRagContext()` 中增加 provider 分发**

```go
ragCfg := agent.RAGConfig.ToRAGConfig()
if ragCfg.Provider == "retrievalops" && len(ragCfg.ExternalKBIDs) > 0 {
	results, err := s.retrieveFromRetrievalOps(ctx, message, ragCfg)
	if err != nil {
		logs.Errorf("retrieveFromRetrievalOps failed: %v", err)
	} else {
		allResult = append(allResult, results...)
	}
} else {
	for _, v := range agent.KnowledgeBases {
		results, err := s.searchKnowledgeBase(ctx, agent.CreatorID, message, v.ID)
		if err != nil {
			logs.Errorf("searchKnowledgeBase 搜索知识库失败: %v", err)
			continue
		}
		allResult = append(allResult, results...)
	}
}
```

- [ ] **Step 3: 保持失败降级为空上下文**

```go
if len(allResult) == 0 {
	return ""
}
```

- [ ] **Step 4: 运行编译检查**

Run: `go test ./app/internal/agents ./app/internal/integrations/retrievalops`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add C:/code/MSZLU-AI/app/internal/agents/service.go
git commit -m "feat: route agent rag context to retrievalops"
```

### Task 6: 增加配置来源与运行时初始化

**Files:**
- Modify: `C:\code\MSZLU-AI\app\etc\config.yml`
- Modify: `C:\code\MSZLU-AI\app\internal\inits\inits.go`
- Modify: `C:\code\MSZLU-AI\app\internal\agents\service.go`

- [ ] **Step 1: 在配置中增加 RetrievalOps 基础配置**

```yaml
retrievalops:
  base_url: http://localhost:8899
  api_key: rag_local_debug_token
  timeout_seconds: 10
```

- [ ] **Step 2: 在初始化阶段读取配置并注入 agent service**

```go
type RetrievalOpsConfig struct {
	BaseURL        string `yaml:"base_url"`
	APIKey         string `yaml:"api_key"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}
```

- [ ] **Step 3: 给 `service` 增加依赖字段**

```go	type service struct {
	repo               repository
	...
	retrievalOpsBaseURL string
	retrievalOpsAPIKey  string
}
```

- [ ] **Step 4: 运行编译检查**

Run: `go test ./app/internal/inits ./app/internal/agents`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add C:/code/MSZLU-AI/app/etc/config.yml C:/code/MSZLU-AI/app/internal/inits/inits.go C:/code/MSZLU-AI/app/internal/agents/service.go
git commit -m "chore: wire retrievalops runtime config"
```

### Task 7: 前端增加 Agent 的 RAG 配置表单

**Files:**
- Modify: `C:\code\MSZLU-AI\frontend\src\types\agent.ts`
- Modify: `C:\code\MSZLU-AI\frontend\src\api\*.ts`
- Modify: `C:\code\MSZLU-AI\frontend\src\views\AgentManagement.vue`
- 如有表单子组件，按实际组件路径同步修改

- [ ] **Step 1: 定义前端 `RAGConfig` 类型**

```ts
export interface RAGConfig {
  provider: 'local' | 'retrievalops'
  externalKbIds: number[]
  topK: number
  strategyProfile: string
  apiKeyRef: string
}
```

- [ ] **Step 2: 在 Agent 类型中增加 `ragConfig`**

```ts
ragConfig?: RAGConfig
```

- [ ] **Step 3: 在创建 / 更新 Agent 请求中传递 `ragConfig`**

```ts
ragConfig: form.ragConfig,
```

- [ ] **Step 4: 在 Agent 配置表单中增加字段**

```ts
ragConfig: {
  provider: 'local',
  externalKbIds: [],
  topK: 4,
  strategyProfile: 'default',
  apiKeyRef: '',
}
```

- [ ] **Step 5: 增加表单控件**

```vue
<el-form-item label="RAG Provider">
  <el-select v-model="form.ragConfig.provider">
    <el-option label="Local" value="local" />
    <el-option label="RetrievalOps" value="retrievalops" />
  </el-select>
</el-form-item>
<el-form-item label="External KB IDs">
  <el-input v-model="externalKbIdsInput" placeholder="例如：101,102" />
</el-form-item>
<el-form-item label="TopK">
  <el-input-number v-model="form.ragConfig.topK" :min="1" :max="20" />
</el-form-item>
<el-form-item label="Strategy Profile">
  <el-input v-model="form.ragConfig.strategyProfile" />
</el-form-item>
```

- [ ] **Step 6: 解析 `externalKbIds` 输入值**

```ts
const parseExternalKbIds = (value: string): number[] => {
  return value
    .split(',')
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isInteger(item) && item > 0)
}
```

- [ ] **Step 7: 运行前端类型检查**

Run: `pnpm --dir C:/code/MSZLU-AI/frontend type-check`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add C:/code/MSZLU-AI/frontend/src
git commit -m "feat: add agent rag provider config form"
```

### Task 8: 打通最小联调与回归验证

**Files:**
- Test: `C:\code\MSZLU-AI\app\internal\integrations\retrievalops\client_test.go`
- Test: `C:\code\MSZLU-AI\app\internal\agents\service.go` 相关新增测试文件（如需要则创建 `service_rag_test.go`）
- Modify: `c:\code\rag-retrievalOps\docs\2026-06-26-zhihangAI-rag-integration-design.md`

- [ ] **Step 1: 为 provider 分发补一个服务层测试**

```go
func TestBuildRagContextUsesRetrievalOpsProvider(t *testing.T) {
	agent := &model.Agent{
		Name: "客服助手",
		RAGConfig: model.JSON{
			"provider": "retrievalops",
			"externalKbIds": []uint64{101},
			"topK": 4,
			"strategyProfile": "default",
		},
	}
	_ = agent
}
```

- [ ] **Step 2: 运行后端关键测试**

Run: `go test ./app/internal/agents ./app/internal/integrations/retrievalops -v`
Expected: PASS

- [ ] **Step 3: 运行前端类型检查**

Run: `pnpm --dir C:/code/MSZLU-AI/frontend type-check`
Expected: PASS

- [ ] **Step 4: 更新设计文档中的“实际落地说明”**

```md
## 实现状态

- 已完成 Agent `ragConfig` 扩展
- 已完成 RetrievalOps client 接入
- 已完成 provider 分发
- 已完成前端 RAG 配置表单
- 第一版采用服务级 API Key
```

- [ ] **Step 5: Commit**

```bash
git add C:/code/MSZLU-AI c:/code/rag-retrievalOps/docs/2026-06-26-zhihangAI-rag-integration-design.md
git commit -m "feat: integrate zhihangAI with retrievalops"
```

### Task 9: 补充教学与交付文档

**Files:**
- Modify: `c:\code\rag-retrievalOps\docs\2026-06-26-zhihangAI-rag-integration-design.md`
- Create: `c:\code\rag-retrievalOps\docs\zhihangAI-rag-integration-handoff.md`

- [ ] **Step 1: 编写交付说明文档**

```md
# zhihangAI 与 rag-retrievalOps 集成交付说明

## 你现在能讲什么
- zhihangAI 是 Agent 平台
- rag-retrievalOps 是 RAG 中台
- 两者通过 provider 模式解耦集成

## 第一版做了什么
- Agent 新增 ragConfig
- 对话链路支持 RetrievalOps 检索
- 前端支持配置 provider

## 第一版没做什么
- citation 完整可视化
- 租户级 API Key
- 混合检索融合排序
```

- [ ] **Step 2: 在设计文档中补“给学员讲解建议”小节**

```md
## 给学员讲解建议

- 先讲分层定位
- 再讲调用链路
- 再讲为什么第一版选择最小侵入集成
- 最后讲后续演进路线
```

- [ ] **Step 3: 检查文档路径与命名统一**

Run: `git status --short`
Expected: 只出现本次相关文档与代码变更。

- [ ] **Step 4: Commit**

```bash
git add c:/code/rag-retrievalOps/docs
git commit -m "docs: add zhihangAI retrievalops handoff guide"
```

---

## 自检结果

### 1. 设计覆盖检查

已覆盖以下设计项：

- `zhihangAI` 和 `rag-retrievalOps` 的分层定位
- 第一版最小侵入集成策略
- `ragConfig` 数据模型设计
- provider 分发逻辑
- 远程 retrieve 接口复用
- 前端 Agent 配置页改造
- 两个样板 Agent 的实现支撑基础
- 文档沉淀与教学交付要求

### 2. 占位符检查

本计划未使用 `TBD`、`TODO`、`后续补充` 等占位语句；每个任务都给出了具体文件路径、代码骨架和验证命令。

### 3. 一致性检查

- `ragConfig` 字段名在模型、接口、前端保持一致。
- provider 统一使用 `local | retrievalops`。
- 外部知识库字段统一使用 `externalKbIds`。
- 远程检索接口统一使用 `/v1/retrieve`。

## 执行交接

计划已保存到 `docs/superpowers/plans/2026-06-26-zhihangAI-rag-integration-plan.md`。

两种执行方式：

**1. Subagent-Driven（推荐）** - 我按任务逐个分派子代理执行，每个任务做完就回看结果，适合稳步推进。  
**2. Inline Execution** - 我在当前会话里按计划连续执行，适合直接开工。

请选择一种。
package specialized_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	componenttool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"

	"interview-agents/internal/agents/interview/specialized"
	"interview-agents/internal/agents/tools"
	"interview-agents/internal/config"
	"interview-agents/internal/milvus"
)

func TestGoAgentRAGTrigger(t *testing.T) {
	ctx := context.Background()

	// 1. 加载 .env (当前在 backend/internal/agents/interview/specialized 目录)
	// 回退4级到工程根目录
	envPath := filepath.Join("..", "..", "..", "..", ".env")
	envPath = "/Users/lucas/work/code/go/go-eino-interview-agent-co-write/.env"
	err := godotenv.Load(envPath)
	if err != nil {
		t.Logf("load .env failed (ignoring if environment vars are already set): %v", err)
	}

	testAPIKey := strings.TrimSpace(os.Getenv("TEST_API_KEY"))
	if testAPIKey == "" {
		t.Skip("TEST_API_KEY not set, skipping integration test")
	}

	// 获取 Embedding配置，fallback 逻辑
	embModel := strings.TrimSpace(os.Getenv("EMBEDDING_MODEL"))
	if embModel == "" {
		embModel = "BAAI/bge-m3"
	}
	embBaseURL := strings.TrimSpace(os.Getenv("EMBEDDING_BASE_URL"))
	if embBaseURL == "" {
		embBaseURL = "https://api.siliconflow.cn/v1"
	}
	// 在测试环境下，直接使用 TestAPIKey 作为 Embedding Token，SiliconFlow 是通用的
	embAPIKey := testAPIKey

	fmt.Printf("Using Embedding API Key starting with: %s\n", embAPIKey[:10])

	// 2. 初始化测试用的 Milvus config，使用单独的 test collection
	collectionName := "test_knowledge_rag_temp"
	cfg := &config.Config{
		Embedding: config.EmbeddingConfig{
			APIKey:     embAPIKey,
			Model:      embModel,
			BaseURL:    embBaseURL,
			Timeout:    30 * time.Second,
			RetryTimes: 3,
			Dimensions: 1024, // BAAI/bge-m3 通常是 1024 维度
		},
		DocumentSplitter: config.SplitterConfig{
			ChunkSize:   500,
			OverlapSize: 50,
			Separators:  []string{"\n\n", "\n", " "},
		},
		Milvus: config.MilvusConfig{
			Address:        "127.0.0.1:19530",
			CollectionName: collectionName,
			DatabaseName:   "default",
			MetricType:     "COSINE",
			Username:       "root",
			Password:       "milvus", // default if any
			TopK:           3,
			ConnectTimeout: 10 * time.Second,
			SearchTimeout:  10 * time.Second,
		},
	}

	// 初始化 Milvus Manager
	t.Log("Initializing Milvus manager and storage...")
	mgr, err := milvus.InitMilvusManager(ctx, cfg)
	if err != nil {
		t.Fatalf("InitMilvusManager failed: %v", err)
	}

	// defer 先清理再关闭
	defer func() {
		t.Log("Cleaning up milvus test collection...")
		err := mgr.Client.DropCollection(ctx, collectionName)
		if err != nil {
			t.Logf("drop collection failed: %v", err)
		}
		mgr.Close()
	}()

	// 3. 构造测试文档并存入milvus
	docs := []*schema.Document{
		{
			ID: "doc_go_map",
			Content: `知识点：Go 语言中的 map 是一个哈希表的实现。它的底层结构是由 hmap 和 bmap 组成。
hmap 是 map 的核心结构，包含了 map 的状态和一些元信息。bmap 是 bucket（桶），每个 bucket 可以存储 8 个键值对。
当发生哈希冲突时，Go 会通过溢出桶（overflow bucket）形成链表来解决冲突。而且它支持扩容机制，由于一次性迁移可能影响性能，Go采用的是渐进式扩容。`,
			MetaData: map[string]any{"source": "unit_test_go_map"},
		},
	}
	_, err = mgr.IndexerService.Store(ctx, docs)
	if err != nil {
		t.Fatalf("Store docs failed: %v", err)
	}
	t.Log("Store test docs completed, waiting for indexing...")
	time.Sleep(3 * time.Second) // 等待索引生效

	// 4. 配置埋点监听器
	called := false
	tools.TestMilvusCallCallback = func(query string) {
		t.Logf("🎯🎯🎯 测试埋点触发了！大模型发起检索，Query=%s", query)
		called = true
	}
	// 执行结束后重置钩子
	defer func() { tools.TestMilvusCallCallback = nil }()

	// 5. 初始化 LLM 模型 (用 DeepSeek-V3)
	t.Log("Initializing DeepSeek-V3 ChatModel...")
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  testAPIKey,
		Model:   "deepseek-ai/DeepSeek-V3", // SiliconFlow 代理使用
		BaseURL: "https://api.siliconflow.cn/v1",
	})
	if err != nil {
		t.Fatalf("Failed to create chat model: %v", err)
	}

	// 6. 获取工具并构造与 NewGoSpecializedAgent 中一致的 Agent
	milvusTool, err := tools.GetMilvusRetrieverTool()
	if err != nil {
		t.Fatalf("GetMilvusRetrieverTool failed: %v", err)
	}

	baseAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "GoSpecializedAgent_Test",
		Description: "Go 专项面试官智能体",
		Instruction: specialized.GoSpecializedAgentInstruction,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []componenttool.BaseTool{milvusTool},
			},
		},
		MaxIterations: 5,
	})
	if err != nil {
		t.Fatalf("NewChatModelAgent failed: %v", err)
	}

	// 7. 发送用户消息
	// 给出一段存在知识含糊的候选人回答，诱导大模型去用 RAG 检索底层细节
	userMsg := schema.UserMessage("候选人：上次你让我自己设计哈希表，我现在说说我对Go语言里面自带的map底层的理解吧，我只记得它是一个普通的哈希表，遇到冲突就用链表去拉链，具体的它的那个bucket什么的我记不清了。请你点评我的回答并结合你的专业知识指出不对的地方或者追问我。请务必使用get_milvus_retriever检索")

	t.Log("Start generating from agent, waiting for AI reasoning and tool calls...")
	it := baseAgent.Run(ctx, &adk.AgentInput{
		Messages: []*schema.Message{userMsg},
	})

	for {
		event, ok := it.Next()
		if !ok {
			break
		}
		if event != nil && event.Err != nil {
			t.Fatalf("Agent generation failed: %v", event.Err)
		}
	}

	// 8. 断言与验证
	if !called {
		t.Error("❌ 大模型没有触发 get_milvus_retriever 检索知识库！当前的 Prompt 模型未能意图识别出需要检索。")
	} else {
		t.Log("✅ 成功验证：大模型顺利触发了知识库检索，RAG增强能力工作正常。")
	}
}

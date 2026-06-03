# 父子块元数据与切块协议升级实现教程

## 1. 背景

这次改动解决的不是“多加几个 metadata 字段”这么简单的问题，而是把一条原本容易断开的链路补齐了。

旧实现里，系统虽然已经能把长文档切成多个 chunk，但是有三个明显短板：

1. 切出来的子块只知道“自己是什么内容”，不知道“自己属于文档里的哪一段、哪一个标题、哪一个更大的父块”。
2. 导入链路和知识库入库链路会在切块之后再补基础元数据，这样一来，切块逻辑本身拿不到 `document_id`、`title`、`file_name` 这些信息，就没法在切块时顺手生成稳定的父子关系。
3. 检索阶段虽然会把命中的 chunk 返回给上层，但 `source` 里的引用协议还不够完整，前端、引用卡片、后续补全文本逻辑很难稳定地拿到 `parent_id`、`section_title`、`hierarchy_path`、offset 这些字段。

这次升级的核心目标，就是把这三个问题一起解决掉：

1. 在切块阶段直接生成父子块元数据。
2. 把切块接口从“只吃纯文本”升级成“吃完整 `schema.Document`”，让切块器能看到原始文档 metadata。
3. 把这些父子块字段一路带进 Milvus、检索结果、`source` 引用协议里，让后面的引用、补全、解释都能复用同一套字段。

你可以先把这次实现理解成一句话：

“不是先切完块再想办法补信息，而是让切块这一步自己成为父子关系元数据的生产者。”

## 2. 这篇教程会做什么

看完这篇教程后，你应该能从零复现下面这条完整链路：

1. 扩展文档元数据结构，让系统认识 `parent_id`、`child_id`、offset、层级路径等字段。
2. 升级通用切块接口和 Markdown 切块接口，让它们接收带 metadata 的 `schema.Document`。
3. 在切块阶段根据标题、段落、窗口大小生成父块，并把父子关系写回每个子块。
4. 调整 Markdown 导入器和知识库入库消费者，让它们在切块前就把基础 metadata 塞进原始文档。
5. 升级 dense / sparse / hybrid / rerank / dedupe 的结果协议，让 `source` 字段也携带父子块信息。
6. 用测试验证“切块时打标成功”和“检索返回时 source 协议正确”。

这次实现主要涉及这些文件：

1. `backend/internal/milvus/document_metadata.go`
2. `backend/internal/milvus/splitter/splitter.go`
3. `backend/internal/milvus/splitter/markdown.go`
4. `backend/internal/milvus/splitter/parent_child.go`
5. `backend/internal/milvus/importer.go`
6. `backend/internal/mq/consumer.go`
7. `backend/internal/milvus/retrieval/search.go`
8. `backend/internal/milvus/retrieval/sparse_search.go`
9. `backend/internal/milvus/retrieval/hybrid_search.go`
10. `backend/internal/milvus/retrieval/dedupe.go`
11. `backend/internal/milvus/retrieval/reranker.go`
12. `backend/internal/milvus/splitter/parent_child_test.go`
13. `backend/internal/milvus/retrieval/source_parent_child_test.go`

这次对初学者最容易“看起来像魔法”的地方主要有三个：

1. 为什么一定要把 metadata 提前到切块之前，而不是切完后再补。
2. 父块不是数据库里的真实独立文档，为什么还要给它生成 `parent_id`、offset 和策略版本。
3. 为什么 retrieval 层要把这些字段再抄一遍到 `source` 里，而不是只留在顶层 metadata。

后面我会专门把这三个点讲透。

## 3. 需要先理解的术语

### 3.1 子块

子块就是最终真正写进向量库、参与检索的那一小段文本。

比如一篇 4000 字的文档，按照 `chunk_size=1000` 切成 5 段，那么这 5 段就是 5 个子块。

这次实现里，子块会有这些关键字段：

1. `chunk_id`
2. `child_id`
3. `child_start_offset`
4. `child_end_offset`

这里 `chunk_id` 和 `child_id` 在这次实现里会保持一致。这样做的原因是兼容旧系统里“大家都认 chunk_id”的习惯，同时开始引入更清晰的父子块命名。

### 3.2 父块

父块不是额外存进 Milvus 的另一条记录，而是“描述这个子块所在较大上下文范围”的一个逻辑块。

你可以把它理解成：

1. 子块负责精确召回。
2. 父块负责告诉我们“这个子块属于哪一大段上下文”。

例如：

1. 子块内容可能只是一段“Milvus 如何写 metadata”。
2. 它的父块可能是 `Storage > Metadata Layout` 这一整节。

这就是为什么父块需要：

1. `parent_id`
2. `parent_start_offset`
3. `parent_end_offset`
4. `section_title`
5. `hierarchy_path`

这些字段不是为了好看，而是为了让后续系统知道“该回看哪一段更大的上下文”。

### 3.3 offset

offset 可以理解成“这段文本在原文里的字符位置区间”。

例如：

1. `child_start_offset = 120`
2. `child_end_offset = 260`

表示这个子块对应原文里 `[120, 260)` 这一段字符。

这类字段很重要，因为后续如果我们要做：

1. 引用高亮
2. 命中段落回显
3. 父块补全文本
4. 精确定位来源

都离不开 offset。

### 3.4 层级路径

层级路径就是 Markdown 标题树里的“从大标题走到当前标题”的完整路径。

例如文档是：

```md
# Handbook
## API Layer
### Auth
```

那么 `Auth` 这一节的层级路径就是：

```text
Handbook > API Layer > Auth
```

这次实现里，这个字段叫 `hierarchy_path`。

它的作用是让系统知道“命中的 chunk 在文档结构里到底处在哪个位置”。

### 3.5 切块协议升级

这里说的“切块协议”，不是网络协议，而是切块服务的输入输出约定。

旧约定更像这样：

1. 给我一段纯字符串。
2. 我只负责切。
3. metadata 后面再说。

新约定变成：

1. 给我一个完整的 `schema.Document`。
2. 里面既有 `Content`，也有 `MetaData`。
3. 我切块的时候顺手把父子块信息也生成好。

这个升级是整次实现里最关键的设计变化。

## 4. 整体流程

先看全局，再看代码，会容易很多。

这次升级后的完整数据流可以按下面 6 步理解：

1. 上游调用方先构造原始 `schema.Document`，把 `document_id`、`title`、`file_name`、`kb_id` 之类基础 metadata 先放进去。
2. `splitter` 不再只接收字符串，而是接收完整 `schema.Document`，并在切块后调用 `annotateSplitChunks`。
3. `annotateSplitChunks` 会先定位每个子块在原文里的 offset，再根据标题和段落构建父块窗口，然后把父子关系 metadata 回填到每个 chunk。
4. 导入器和 MQ 消费者拿到这些 chunk 后，直接写入 Milvus，不再在切块之后二次拼装基础 metadata。
5. 检索阶段从 Milvus 读回 chunk 后，会把父子块字段再整理进 `source` 字段，形成统一引用协议。
6. 前端、引用展示、后续父块补全逻辑，只需要认这一套统一字段，不需要再自己猜 chunk 对应哪一节。

你可以把这次实现粗略分成三层：

1. `生产层`：`splitter` 负责真正生成父子块 metadata。
2. `入库层`：`importer` 和 `consumer` 负责保证切块前就把基础 metadata 准备好。
3. `消费层`：`retrieval` 负责把这些字段稳定带回调用方。

## 5. 分步实现

## 第一步：先把元数据结构补齐

### 目标

先让系统的数据结构能表达父子块关系。否则后面的逻辑就算算出来了，也没有标准地方存。

### 文件

`backend/internal/milvus/document_metadata.go`

### 完整代码

```go
type DocumentMetadata struct {
	Language DocumentLanguage `json:"language"`

	Category DocumentCategory `json:"category"`

	FilePath string `json:"file_path"`

	FileName string `json:"file_name"`

	Title string `json:"title"`

	Source string `json:"source,omitempty"`

	OperatorAdminID uint   `json:"operator_admin_id"`
	KBScope         string `json:"kb_scope"`

	KBID uint64 `json:"kb_id"`

	DocumentID uint64 `json:"document_id"`

	ChunkIndex int `json:"chunk_index,omitempty"`

	TotalChunks int `json:"total_chunks,omitempty"`

	ChunkID string `json:"chunk_id,omitempty"`

	ParentID string `json:"parent_id,omitempty"`

	ChildID string `json:"child_id,omitempty"`

	ChildStartOffset int `json:"child_start_offset,omitempty"`

	ChildEndOffset int `json:"child_end_offset,omitempty"`

	ParentStartOffset int `json:"parent_start_offset,omitempty"`

	ParentEndOffset int `json:"parent_end_offset,omitempty"`

	SectionTitle string `json:"section_title,omitempty"`

	HierarchyPath string `json:"hierarchy_path,omitempty"`

	ParentTokenCount int `json:"parent_token_count,omitempty"`

	ParentBuildStrategy string `json:"parent_build_strategy,omitempty"`

	ParentBuildVersion string `json:"parent_build_version,omitempty"`

	ParentTruncated bool `json:"parent_truncated,omitempty"`

	ParentChildAvailable bool `json:"parent_child_available,omitempty"`

	CreatedAt string `json:"created_at"`

	Extra map[string]interface{} `json:"extra,omitempty"`
}

func NewKBDocumentMetadata(operatorAdminID uint, kbID, documentID uint64, fileName string) *DocumentMetadata {
	return &DocumentMetadata{
		OperatorAdminID: operatorAdminID,
		KBScope:         "global",
		KBID:            kbID,
		DocumentID:      documentID,
		FileName:        fileName,
		ChunkIndex:      0,
		TotalChunks:     0,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Extra:           make(map[string]interface{}),
	}
}

func (m *DocumentMetadata) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"language":   string(m.Language),
		"category":   string(m.Category),
		"file_path":  m.FilePath,
		"file_name":  m.FileName,
		"title":      m.Title,
		"created_at": m.CreatedAt,
	}

	if m.Source != "" {
		result["source"] = m.Source
	}

	if m.OperatorAdminID > 0 {
		result["operator_admin_id"] = m.OperatorAdminID
	}
	if m.KBScope != "" {
		result["kb_scope"] = m.KBScope
	}

	if m.KBID > 0 {
		result["kb_id"] = m.KBID
	}

	if m.DocumentID > 0 {
		result["document_id"] = m.DocumentID
	}

	if m.ChunkIndex >= 0 {
		result["chunk_index"] = m.ChunkIndex
	}

	if m.TotalChunks > 0 {
		result["total_chunks"] = m.TotalChunks
	}
	if m.ChunkID != "" {
		result["chunk_id"] = m.ChunkID
	}
	if m.ChildID != "" {
		result["child_id"] = m.ChildID
	}
	if m.ChildID != "" || m.ChunkID != "" {
		result["child_start_offset"] = m.ChildStartOffset
		result["child_end_offset"] = m.ChildEndOffset
	}
	if m.ParentID != "" {
		result["parent_id"] = m.ParentID
		result["parent_start_offset"] = m.ParentStartOffset
		result["parent_end_offset"] = m.ParentEndOffset
		result["parent_token_count"] = m.ParentTokenCount
	}
	if m.SectionTitle != "" {
		result["section_title"] = m.SectionTitle
	}
	if m.HierarchyPath != "" {
		result["hierarchy_path"] = m.HierarchyPath
	}
	if m.ParentBuildStrategy != "" {
		result["parent_build_strategy"] = m.ParentBuildStrategy
	}
	if m.ParentBuildVersion != "" {
		result["parent_build_version"] = m.ParentBuildVersion
	}
	if m.ParentTruncated {
		result["parent_truncated"] = true
	}
	if m.ParentChildAvailable {
		result["parent_child_available"] = true
	}

	for k, v := range m.Extra {
		result[k] = v
	}

	return result
}

func EnrichDocumentsWithMetadata(chunks []*schema.Document, baseMetadata *DocumentMetadata) []*schema.Document {
	totalChunks := len(chunks)

	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}
		chunkMetadata := *baseMetadata
		chunkMetadata.ChunkIndex = i
		chunkMetadata.TotalChunks = totalChunks

		if chunk.MetaData == nil {
			chunk.MetaData = make(map[string]interface{})
		}

		baseMap := chunkMetadata.ToMap()
		for k, v := range baseMap {
			if _, exists := chunk.MetaData[k]; !exists {
				chunk.MetaData[k] = v
			}
		}
	}

	return chunks
}
```

### 这段代码在做什么

这一步做了两件事：

1. 给 `DocumentMetadata` 增加了完整的父子块字段。
2. 让 `ToMap()` 能把这些字段真正输出到 `schema.Document.MetaData`。

其中最关键的是这几组字段：

1. 标识类：`chunk_id`、`child_id`、`parent_id`
2. 定位类：`child_start_offset`、`child_end_offset`、`parent_start_offset`、`parent_end_offset`
3. 结构类：`section_title`、`hierarchy_path`
4. 版本类：`parent_build_strategy`、`parent_build_version`
5. 状态类：`parent_truncated`、`parent_child_available`

### 为什么要这样写

如果只是临时在某个函数里往 `map[string]interface{}` 塞值，短期当然也能跑，但会有三个问题：

1. 没有统一字段定义，别的调用方根本不知道哪些字段是正式协议的一部分。
2. `ToMap()` 不补齐的话，很多字段根本进不了 `schema.Document.MetaData`。
3. 后续如果要把同一套字段复用到知识库导入、Milvus 返回、引用协议，就会到处复制粘贴。

所以这一步的本质，是先把“协议名字”正式定下来。

### 它如何衔接下一步

有了这些字段以后，切块器才有地方把父子关系写进去。接下来我们就要升级切块服务，让它能在切块时拿到原始文档 metadata。

## 第二步：升级切块服务的输入协议

### 目标

把切块服务从“只切文本”升级成“切完整文档”，这样它才能在切块时访问原始 metadata。

### 文件

1. `backend/internal/milvus/splitter/splitter.go`
2. `backend/internal/milvus/splitter/markdown.go`

### 完整代码

```go
func (s *DocumentSplitterService) Split(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("docs is empty")
	}

	results := make([]*schema.Document, 0)
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		splitResults, err := s.splitter.Transform(ctx, []*schema.Document{doc})
		if err != nil {
			return nil, fmt.Errorf("failed to split documents: %w", err)
		}
		results = append(results, s.annotateSplitChunks(doc, splitResults)...)
	}

	return results, nil
}
```

```go
func (s *DocumentSplitterService) SplitMarkdown(ctx context.Context, markdownContent string) ([]*schema.Document, error) {
	if markdownContent == "" {
		return nil, fmt.Errorf("markdown content is empty")
	}

	doc := &schema.Document{
		Content: markdownContent,
	}
	return s.SplitMarkdownDocument(ctx, doc)
}

func (s *DocumentSplitterService) SplitMarkdownDocument(ctx context.Context, doc *schema.Document) ([]*schema.Document, error) {
	if doc == nil || doc.Content == "" {
		return nil, fmt.Errorf("markdown document is empty")
	}

	markdownSeparators := []string{
		"\n\n\n",
		"\n## ",
		"\n### ",
		"\n#### ",
		"\n##### ",
		"\n###### ",
		"\n# ",
		"\n\n",
		"\n```",
		"\n---",
		"\n***",
		"\n- ",
		"\n* ",
		"\n1. ",
		"\n2. ",
		"\n3. ",
		"\n",
		"。", 
		"！", 
		"？", 
		". ",
		"! ",
		"? ",
	}

	markdownConfig := &recursive.Config{
		ChunkSize:   s.config.ChunkSize,
		OverlapSize: s.config.OverlapSize,
		Separators:  markdownSeparators,
		KeepType:    s.config.KeepType,
	}

	markdownSplitter, err := recursive.NewSplitter(ctx, markdownConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown splitter: %w", err)
	}

	results, err := markdownSplitter.Transform(ctx, []*schema.Document{doc})
	if err != nil {
		return nil, fmt.Errorf("failed to split markdown document: %w", err)
	}

	return s.annotateSplitChunks(doc, results), nil
}

func (s *DocumentSplitterService) SplitMarkdownDocuments(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("docs is empty")
	}

	markdownSeparators := []string{
		"\n\n\n",
		"\n## ",
		"\n### ",
		"\n#### ",
		"\n##### ",
		"\n###### ",
		"\n# ",
		"\n\n",
		"\n```",
		"\n---",
		"\n***",
		"\n- ",
		"\n* ",
		"\n1. ",
		"\n2. ",
		"\n3. ",
		"\n",
		"。",
		"！",
		"？",
		". ",
		"! ",
		"? ",
	}

	markdownConfig := &recursive.Config{
		ChunkSize:   s.config.ChunkSize,
		OverlapSize: s.config.OverlapSize,
		Separators:  markdownSeparators,
		KeepType:    s.config.KeepType,
	}

	markdownSplitter, err := recursive.NewSplitter(ctx, markdownConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown splitter: %w", err)
	}

	results := make([]*schema.Document, 0)
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		splitResults, err := markdownSplitter.Transform(ctx, []*schema.Document{doc})
		if err != nil {
			return nil, fmt.Errorf("failed to split markdown documents: %w", err)
		}
		results = append(results, s.annotateSplitChunks(doc, splitResults)...)
	}

	return results, nil
}
```

### 这段代码在做什么

这一步的关键变化只有一句话：

“切块完成之后，不是直接返回结果，而是统一进入 `annotateSplitChunks` 做父子块标注。”

另外新增的 `SplitMarkdownDocument` 也很重要，因为它允许上游直接传入一个已经带 metadata 的 `schema.Document`。

### 为什么要这样写

最简单的做法当然还是保留旧接口：`SplitMarkdown(ctx, string)`。但这样有一个根本问题：

1. 切块器只能看到文本，看不到 `document_id`、`title`、`file_name`。
2. 看不到这些字段，就没法生成稳定的 `documentKey`、`parent_id`、`hierarchy_path`。
3. 于是你只能切完块之后再补 metadata，而那时父子块关系已经错过了最自然的生成时机。

所以这一步不是“多写一个函数”，而是正式升级切块服务的输入契约。

### 它如何衔接下一步

现在切块器已经有机会看到完整文档了，下一步就是在 `annotateSplitChunks` 里真正把父子块关系算出来。

## 第三步：在切块阶段生成父子块元数据

### 目标

让切块器在返回每个子块之前，就把它对应的父块、标题路径、offset、策略版本全部算好。

### 文件

`backend/internal/milvus/splitter/parent_child.go`

### 完整代码

下面这组代码是这次实现的核心。为了方便初学者照着复现，我把关键实现块按实际调用顺序完整放出来。

```go
const (
	parentChildMetadataVersion = "phase3-parent-child-v1"
	parentStrategyHeading      = "heading_section"
	parentStrategyHeadingWin   = "heading_section_window"
	parentStrategyParagraph    = "paragraph_window"
	defaultSectionTitle        = "Document"
)

type textSpan struct {
	Start int
	End   int
}

type headingSection struct {
	Level         int
	Title         string
	HierarchyPath string
	Start         int
	End           int
}

type parentBlock struct {
	ID            string
	SectionTitle  string
	HierarchyPath string
	Start         int
	End           int
	Strategy      string
	TokenCount    int
	Truncated     bool
}
```

```go
func (s *DocumentSplitterService) annotateSplitChunks(original *schema.Document, chunks []*schema.Document) []*schema.Document {
	if len(chunks) == 0 {
		return chunks
	}

	originalContent := ""
	baseMeta := map[string]interface{}{}
	if original != nil {
		originalContent = original.Content
		baseMeta = cloneMetadataMap(original.MetaData)
	}

	chunkSpans := locateChunkOffsets(originalContent, chunks, s.config)
	blocks := buildParentBlocks(originalContent, baseMeta, chunkSpans, s.config)
	documentKey := resolveDocumentKey(baseMeta, originalContent)

	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}

		mergedMeta := cloneMetadataMap(baseMeta)
		for key, value := range cloneMetadataMap(chunk.MetaData) {
			mergedMeta[key] = value
		}

		chunkID := fmt.Sprintf("%s-child-%03d", documentKey, i)
		block := pickParentBlock(blocks, chunkSpans[i])
		parentID := ""
		sectionTitle := resolveBaseTitle(baseMeta)
		hierarchyPath := sectionTitle
		parentStart := 0
		parentEnd := 0
		parentTokenCount := 0
		parentStrategy := parentStrategyParagraph
		parentTruncated := false
		parentAvailable := false

		if block != nil {
			parentID = block.ID
			sectionTitle = block.SectionTitle
			hierarchyPath = block.HierarchyPath
			parentStart = block.Start
			parentEnd = block.End
			parentTokenCount = block.TokenCount
			parentStrategy = block.Strategy
			parentTruncated = block.Truncated
			parentAvailable = true
		}

		mergedMeta["chunk_index"] = i
		mergedMeta["total_chunks"] = len(chunks)
		mergedMeta["chunk_id"] = chunkID
		mergedMeta["child_id"] = chunkID
		mergedMeta["child_start_offset"] = chunkSpans[i].Start
		mergedMeta["child_end_offset"] = chunkSpans[i].End
		mergedMeta["section_title"] = sectionTitle
		mergedMeta["hierarchy_path"] = hierarchyPath
		mergedMeta["parent_child_available"] = parentAvailable
		mergedMeta["parent_build_version"] = parentChildMetadataVersion
		mergedMeta["parent_build_strategy"] = parentStrategy
		mergedMeta["parent_truncated"] = parentTruncated

		if parentAvailable {
			mergedMeta["parent_id"] = parentID
			mergedMeta["parent_start_offset"] = parentStart
			mergedMeta["parent_end_offset"] = parentEnd
			mergedMeta["parent_token_count"] = parentTokenCount
		}

		chunk.MetaData = mergedMeta
		if strings.TrimSpace(chunk.ID) == "" {
			chunk.ID = chunkID
		}
	}

	return chunks
}
```

```go
func locateChunkOffsets(content string, chunks []*schema.Document, cfg *recursive.Config) []textSpan {
	spans := make([]textSpan, len(chunks))
	if strings.TrimSpace(content) == "" {
		return spans
	}

	searchFrom := 0
	lastStart := 0
	overlapHint := 0
	if cfg != nil && cfg.OverlapSize > 0 {
		overlapHint = cfg.OverlapSize
	}

	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}
		piece := chunk.Content
		start := findChunkOffset(content, piece, searchFrom)
		if start < 0 && i > 0 {
			retryFrom := maxInt(0, spans[i-1].End-overlapHint-len(piece))
			start = findChunkOffset(content, piece, retryFrom)
		}
		if start < 0 {
			start = findChunkOffset(content, piece, 0)
		}
		if start < 0 {
			start = minInt(searchFrom, len(content))
		}

		end := start + len(piece)
		if end > len(content) {
			end = len(content)
		}
		if end < start {
			end = start
		}

		spans[i] = textSpan{Start: start, End: end}
		lastStart = start
		searchFrom = maxInt(lastStart+1, end-overlapHint)
	}

	return spans
}

func findChunkOffset(content, piece string, from int) int {
	if strings.TrimSpace(content) == "" || strings.TrimSpace(piece) == "" {
		return -1
	}
	if from < 0 {
		from = 0
	}
	if from >= len(content) {
		from = len(content) - 1
	}
	if from < 0 {
		from = 0
	}

	if idx := strings.Index(content[from:], piece); idx >= 0 {
		return from + idx
	}

	trimmed := strings.TrimSpace(piece)
	if trimmed != "" {
		if idx := strings.Index(content[from:], trimmed); idx >= 0 {
			return from + idx
		}
	}

	return -1
}
```

```go
func buildParentBlocks(content string, baseMeta map[string]interface{}, childSpans []textSpan, cfg *recursive.Config) []*parentBlock {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	parentCap := resolveParentCharCap(cfg)
	paragraphs := extractParagraphSpans(content)
	sections := extractHeadingSections(content)
	blocks := make([]*parentBlock, 0)
	documentKey := resolveDocumentKey(baseMeta, content)
	baseTitle := resolveBaseTitle(baseMeta)

	if len(sections) == 0 {
		baseBlock := splitRegionIntoParentBlocks(
			documentKey,
			baseTitle,
			baseTitle,
			parentStrategyParagraph,
			textSpan{Start: 0, End: len(content)},
			paragraphs,
			parentCap,
			content,
		)
		return baseBlock
	}

	for _, section := range sections {
		sectionSpan := textSpan{Start: section.Start, End: section.End}
		sectionParagraphs := filterParagraphsWithin(paragraphs, sectionSpan)
		strategy := parentStrategyHeading
		if section.End-section.Start > parentCap {
			strategy = parentStrategyHeadingWin
		}
		sectionBlocks := splitRegionIntoParentBlocks(
			documentKey,
			section.Title,
			section.HierarchyPath,
			strategy,
			sectionSpan,
			sectionParagraphs,
			parentCap,
			content,
		)
		blocks = append(blocks, sectionBlocks...)
	}

	if len(blocks) == 0 {
		return splitRegionIntoParentBlocks(
			documentKey,
			baseTitle,
			baseTitle,
			parentStrategyParagraph,
			textSpan{Start: 0, End: len(content)},
			paragraphs,
			parentCap,
			content,
		)
	}
	return blocks
}

func splitRegionIntoParentBlocks(
	documentKey string,
	sectionTitle string,
	hierarchyPath string,
	strategy string,
	region textSpan,
	paragraphs []textSpan,
	parentCap int,
	content string,
) []*parentBlock {
	if region.End < region.Start {
		region.End = region.Start
	}
	if parentCap <= 0 {
		parentCap = region.End - region.Start
	}

	var spans []textSpan
	if len(paragraphs) == 0 || region.End-region.Start <= parentCap {
		spans = []textSpan{region}
	} else {
		blockStart := region.Start
		blockEnd := region.Start
		for _, paragraph := range paragraphs {
			candidateEnd := paragraph.End
			if candidateEnd-blockStart > parentCap && blockEnd > blockStart {
				spans = append(spans, textSpan{Start: blockStart, End: blockEnd})
				blockStart = paragraph.Start
			}
			blockEnd = candidateEnd
		}
		if blockEnd <= blockStart {
			blockEnd = region.End
		}
		spans = append(spans, textSpan{Start: blockStart, End: minInt(blockEnd, region.End)})
	}

	blocks := make([]*parentBlock, 0, len(spans))
	for idx, span := range spans {
		if span.Start < region.Start {
			span.Start = region.Start
		}
		if span.End > region.End {
			span.End = region.End
		}
		if span.End <= span.Start {
			span.End = minInt(region.End, span.Start)
		}
		spanContent := sliceContent(content, span)
		blockID := fmt.Sprintf("%s-parent-%s-%03d", documentKey, shortHash(fmt.Sprintf("%s:%d:%d", hierarchyPath, span.Start, span.End)), idx)
		blocks = append(blocks, &parentBlock{
			ID:            blockID,
			SectionTitle:  firstNonEmptyString(sectionTitle, defaultSectionTitle),
			HierarchyPath: firstNonEmptyString(hierarchyPath, sectionTitle, defaultSectionTitle),
			Start:         span.Start,
			End:           span.End,
			Strategy:      strategy,
			TokenCount:    approximateTokenCount(spanContent),
			Truncated:     len(spans) > 1 || (region.End-region.Start) > (span.End-span.Start),
		})
	}

	return blocks
}
```

```go
func pickParentBlock(blocks []*parentBlock, child textSpan) *parentBlock {
	if len(blocks) == 0 {
		return nil
	}

	var bestContaining *parentBlock
	bestContainingWidth := math.MaxInt
	for _, block := range blocks {
		if block == nil {
			continue
		}
		if child.Start >= block.Start && child.Start < block.End {
			width := block.End - block.Start
			if width < bestContainingWidth {
				bestContaining = block
				bestContainingWidth = width
			}
		}
	}
	if bestContaining != nil {
		return bestContaining
	}

	childCenter := child.Start + (child.End-child.Start)/2
	best := blocks[0]
	bestDistance := math.MaxInt
	for _, block := range blocks {
		if block == nil {
			continue
		}
		distance := 0
		switch {
		case childCenter < block.Start:
			distance = block.Start - childCenter
		case childCenter > block.End:
			distance = childCenter - block.End
		default:
			distance = 0
		}
		if distance < bestDistance {
			best = block
			bestDistance = distance
		}
	}
	return best
}
```

```go
func extractHeadingSections(content string) []headingSection {
	type heading struct {
		level int
		title string
		start int
		path  string
	}

	headings := make([]heading, 0)
	stack := make([]heading, 0, 6)
	forEachLine(content, func(start, _ int, line string) {
		level, title, ok := parseMarkdownHeading(line)
		if !ok {
			return
		}
		for len(stack) > 0 && stack[len(stack)-1].level >= level {
			stack = stack[:len(stack)-1]
		}
		entry := heading{
			level: level,
			title: title,
			start: start,
		}
		stack = append(stack, entry)
		pathParts := make([]string, 0, len(stack))
		for _, item := range stack {
			pathParts = append(pathParts, item.title)
		}
		entry.path = strings.Join(pathParts, " > ")
		headings = append(headings, entry)
		stack[len(stack)-1] = entry
	})

	sections := make([]headingSection, 0, len(headings))
	for i, item := range headings {
		end := len(content)
		for j := i + 1; j < len(headings); j++ {
			if headings[j].level <= item.level {
				end = headings[j].start
				break
			}
		}
		sections = append(sections, headingSection{
			Level:         item.level,
			Title:         item.title,
			HierarchyPath: item.path,
			Start:         item.start,
			End:           end,
		})
	}
	return sections
}

func parseMarkdownHeading(line string) (int, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}

	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	if len(trimmed) <= level || trimmed[level] != ' ' {
		return 0, "", false
	}

	title := strings.TrimSpace(trimmed[level:])
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func extractParagraphSpans(content string) []textSpan {
	paragraphs := make([]textSpan, 0)
	inParagraph := false
	start := 0
	end := 0

	forEachLine(content, func(lineStart, lineEnd int, line string) {
		if strings.TrimSpace(line) == "" {
			if inParagraph {
				paragraphs = append(paragraphs, textSpan{Start: start, End: end})
				inParagraph = false
			}
			return
		}
		if !inParagraph {
			start = lineStart
			inParagraph = true
		}
		end = lineEnd
	})

	if inParagraph {
		paragraphs = append(paragraphs, textSpan{Start: start, End: end})
	}
	if len(paragraphs) == 0 && strings.TrimSpace(content) != "" {
		return []textSpan{{Start: 0, End: len(content)}}
	}
	return paragraphs
}

func filterParagraphsWithin(paragraphs []textSpan, region textSpan) []textSpan {
	out := make([]textSpan, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		if paragraph.End <= region.Start {
			continue
		}
		if paragraph.Start >= region.End {
			continue
		}
		start := maxInt(paragraph.Start, region.Start)
		end := minInt(paragraph.End, region.End)
		if end > start {
			out = append(out, textSpan{Start: start, End: end})
		}
	}
	return out
}
```

```go
func resolveParentCharCap(cfg *recursive.Config) int {
	chunkSize := 1000
	if cfg != nil && cfg.ChunkSize > 0 {
		chunkSize = cfg.ChunkSize
	}
	return maxInt(chunkSize*3, chunkSize)
}

func resolveDocumentKey(meta map[string]interface{}, content string) string {
	if meta != nil {
		for _, key := range []string{"document_id", "doc_id"} {
			if value := strings.TrimSpace(fmt.Sprint(meta[key])); value != "" && value != "<nil>" {
				return "doc-" + sanitizeIDComponent(value)
			}
		}
		for _, key := range []string{"file_name", "title"} {
			if value := strings.TrimSpace(fmt.Sprint(meta[key])); value != "" && value != "<nil>" {
				return sanitizeIDComponent(value) + "-" + shortHash(content)
			}
		}
	}
	if strings.TrimSpace(content) == "" {
		return "document"
	}
	return "document-" + shortHash(content)
}

func resolveBaseTitle(meta map[string]interface{}) string {
	if meta != nil {
		for _, key := range []string{"title", "file_name"} {
			if value := strings.TrimSpace(fmt.Sprint(meta[key])); value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return defaultSectionTitle
}

func cloneMetadataMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
```

### 这段代码在做什么

这个文件做的事可以分成 4 小步：

1. 先给每个子块定位它在原文中的 offset。
2. 再从原文里抽出标题结构和段落结构。
3. 基于标题节或段落窗口生成逻辑父块。
4. 最后为每个子块选出最匹配的父块，并把一整套 metadata 写回去。

如果把这个过程说得更直白一点，就是：

1. 先确认“子块在原文哪儿”。
2. 再确认“原文有哪些可当父块的大区间”。
3. 最后回答“这个子块应该挂在哪个大区间下面”。

### 为什么要这样写

这里有几个设计点很值得初学者重点理解。

第一，为什么要先做 `locateChunkOffsets`。

因为切块器本身只给了我们“切完的文本”，不会直接告诉我们“这段文本在原文第几个字符开始”。如果没有 offset，后面的父块匹配、引用高亮、上下文补全都没法做。

第二，为什么父块是“逻辑块”，而不是再写一份独立文档进库。

因为这次目标不是做双层索引，而是先让每个召回到的子块带上足够的“上下文坐标”。如果现在就把父块也单独进库，复杂度会一下上去很多，包括：

1. 父块和子块如何保持一致。
2. 父块是否也做 embedding。
3. 检索时先搜父块还是先搜子块。

当前版本先不碰这个复杂度，而是只把父块当成 metadata 里的上下文描述。

第三，为什么要区分 `heading_section`、`heading_section_window`、`paragraph_window`。

因为不是所有文档结构都一样：

1. 有标题结构时，优先按标题节建父块，语义最自然。
2. 某个标题节太长时，不能整节都当一个父块，否则上下文太大，所以要切成窗口，也就是 `heading_section_window`。
3. 没有标题结构时，就退回段落窗口策略，也就是 `paragraph_window`。

这也是为什么要记录 `parent_build_strategy`。它能告诉后面的人“这个父块是怎么生成出来的”。

第四，为什么要记录 `parent_build_version`。

因为父子块构建规则以后很可能会继续演进。版本号可以帮助我们区分：

1. 老数据是按哪套逻辑生成的。
2. 新数据是否需要回灌或重建。

### 它如何衔接下一步

现在切块器已经能在每个 chunk 上打出完整父子块 metadata。下一步就要保证所有进入切块器的调用方，都能在切块前先把基础 metadata 准备好。

## 第四步：升级导入和入库链路，让 metadata 在切块前就存在

### 目标

让 Markdown 导入器和知识库 MQ 消费者都在切块前构造原始 `schema.Document`，而不是切完以后再补 metadata。

### 文件

1. `backend/internal/milvus/importer.go`
2. `backend/internal/mq/consumer.go`

### 完整代码

```go
func (mi *MarkdownImporter) ImportFile(ctx context.Context, filePath string, opts *ImportOptions) (*ImportResult, error) {
	if opts == nil {
		opts = DefaultImportOptions()
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	if opts.MaxFileSize > 0 && int64(len(content)) > opts.MaxFileSize {
		return nil, fmt.Errorf("file %s exceeds max size %d bytes", filePath, opts.MaxFileSize)
	}

	language := opts.Language
	category := opts.Category

	metadata := NewDocumentMetadata(filePath, language, category)
	if opts.Source != "" {
		metadata.Source = opts.Source
	}

	title := extractTitleFromMarkdown(string(content))
	if title != "" {
		metadata.Title = title
	}

	doc := &schema.Document{
		Content:  string(content),
		MetaData: metadata.ToMap(),
	}

	chunks, err := mi.manager.SplitterService.SplitMarkdownDocument(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to split markdown file %s: %w", filePath, err)
	}

	for i, chunk := range chunks {
		if chunk.ID == "" {
			chunk.ID = generateChunkID(metadata.Language, metadata.Category, i)
		}
	}

	docIDs, err := mi.manager.IndexerService.Store(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to store documents to Milvus: %w", err)
	}

	return &ImportResult{
		TotalFiles:   1,
		SuccessFiles: 1,
		FailedFiles:  0,
		TotalChunks:  len(chunks),
		DocumentIDs:  docIDs,
		Errors:       nil,
	}, nil
}
```

```go
func (mi *MarkdownImporter) ImportText(ctx context.Context, content string, opts *TextImportOptions) (*ImportResult, error) {
	if opts == nil {
		opts = DefaultTextImportOptions()
	}

	if content == "" {
		return nil, fmt.Errorf("content is empty, nothing to import")
	}

	if opts.MaxSize > 0 && int64(len(content)) > opts.MaxSize {
		return nil, fmt.Errorf("content size %d bytes exceeds max size %d bytes", len(content), opts.MaxSize)
	}

	metadata := NewDocumentMetadata("memory://"+opts.Title, opts.Language, opts.Category)
	metadata.Title = opts.Title
	metadata.Source = opts.Source
	metadata.FilePath = ""
	metadata.FileName = opts.Title

	if opts.Title == "" || opts.Title == "未命名文档" {
		extractedTitle := extractTitleFromMarkdown(content)
		if extractedTitle != "" {
			metadata.Title = extractedTitle
		}
	}

	doc := &schema.Document{
		Content:  content,
		MetaData: metadata.ToMap(),
	}

	chunks, err := mi.manager.SplitterService.SplitMarkdownDocument(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to split text content: %w", err)
	}

	for i, chunk := range chunks {
		if chunk.ID == "" {
			chunk.ID = generateChunkID(metadata.Language, metadata.Category, i)
		}
	}

	docIDs, err := mi.manager.IndexerService.Store(ctx, chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to store documents to Milvus: %w", err)
	}

	return &ImportResult{
		TotalFiles:   1,
		SuccessFiles: 1,
		FailedFiles:  0,
		TotalChunks:  len(chunks),
		DocumentIDs:  docIDs,
		Errors:       nil,
	}, nil
}
```

```go
func ingestKnowledgeDocument(ctx context.Context, payload KnowledgeIngestPayload) (int, error) {
	rawText, err := extractKnowledgeRawText(ctx, payload.FilePath, payload.FileType)
	if err != nil {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeParse, "failed to extract source text", err)
	}

	manager, err := milvus.GetMilvusManager()
	if err != nil {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeMilvus, "failed to get milvus manager", err)
	}
	if manager.GetSplitterService() == nil || manager.GetIndexerService() == nil {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeMilvus, "milvus services are not initialized", nil)
	}

	docRecord, err := model.KBDocumentDao.GetByID(payload.DocumentID)
	if err != nil {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeUnknown, "failed to load source document", err)
	}

	baseMeta := milvus.NewKBDocumentMetadata(payload.OperatorAdminID, payload.KBID, payload.DocumentID, docRecord.FileName)
	doc := &schema.Document{
		Content:  rawText,
		MetaData: baseMeta.ToMap(),
	}
	chunks, err := manager.GetSplitterService().Split(ctx, []*schema.Document{doc})
	if err != nil {
		errorCode := classifyKnowledgeIngestError(err)
		return 0, buildKnowledgeIngestError(errorCode, "failed to split knowledge document", err)
	}
	if len(chunks) == 0 {
		return 0, buildKnowledgeIngestError(knowledgeIngestErrorTypeParse, "empty chunks after split", nil)
	}

	totalChunks := len(chunks)
	for i, chunk := range chunks {
		if chunk == nil {
			continue
		}
		if chunk.ID == "" {
			chunk.ID = fmt.Sprintf("kb_%d_doc_%d_chunk_%d_%d", payload.KBID, payload.DocumentID, i, time.Now().UnixNano())
		}
	}

	if _, err := manager.GetIndexerService().Store(ctx, chunks); err != nil {
		errorCode := classifyKnowledgeIngestError(err)
		return 0, buildKnowledgeIngestError(errorCode, "failed to store chunks to milvus", err)
	}

	return totalChunks, nil
}
```

### 这段代码在做什么

这一层做的事情很朴素，但非常关键：

1. 先构造原始 `schema.Document`。
2. 把基础 metadata 提前放进去。
3. 再调用升级后的切块接口。

也就是说，顺序从过去的：

1. 读文本
2. 切块
3. 给 chunk 补 metadata

变成了新的：

1. 读文本
2. 先构造带 metadata 的原始文档
3. 让切块器在切块时就拿到这些 metadata
4. 切完后直接入库

### 为什么要这样写

这是整次升级里最容易被低估的一步。

如果还沿用旧写法，在切块之后再补 metadata，会出问题：

1. `annotateSplitChunks` 看不到 `document_id`，就没法生成稳定的 `doc-xxx-child-000` 这类 ID。
2. 看不到 `title`、`file_name`，就没法构建像 `Handbook > API Layer` 这种更稳定的上下文标识。
3. 知识库入库链路会和 Markdown 导入链路走出两套不同的 metadata 生产逻辑，后面检索层会越来越难维护。

所以这一步本质上是在统一“metadata 的生产时机”。

### 它如何衔接下一步

现在进入 Milvus 的 chunk 已经自带完整父子块 metadata 了。下一步就要保证这些字段从 Milvus 读回来之后，不会在 retrieval 层丢失。

## 第五步：升级检索返回协议，把父子块字段写进 source

### 目标

让 dense、sparse、hybrid、rerank、dedupe 这些路径返回的文档，都把父子块字段带进统一的 `source` 协议。

### 文件

1. `backend/internal/milvus/retrieval/reranker.go`
2. `backend/internal/milvus/retrieval/search.go`
3. `backend/internal/milvus/retrieval/sparse_search.go`
4. `backend/internal/milvus/retrieval/hybrid_search.go`
5. `backend/internal/milvus/retrieval/dedupe.go`

### 完整代码

先看这次协议升级里最核心的公共函数：

```go
func ensureSourceMetadata(doc *schema.Document) map[string]interface{} {
	source := make(map[string]interface{})
	if doc != nil && doc.MetaData != nil {
		switch existing := doc.MetaData["source"].(type) {
		case map[string]interface{}:
			if existing != nil {
				for key, value := range existing {
					source[key] = value
				}
			}
		case string:
			if strings.TrimSpace(existing) != "" {
				source["origin"] = strings.TrimSpace(existing)
			}
		default:
			if existing != nil {
				if value := strings.TrimSpace(fmt.Sprint(existing)); value != "" {
					source["origin"] = value
				}
			}
		}
	}
	return source
}

func annotateParentChildSource(doc *schema.Document) {
	if doc == nil {
		return
	}
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]interface{})
	}

	source := ensureSourceMetadata(doc)
	copyMetadataFieldsToMap(source, doc.MetaData,
		"document_id",
		"chunk_id",
		"parent_id",
		"child_id",
		"chunk_index",
		"section_title",
		"hierarchy_path",
		"parent_start_offset",
		"parent_end_offset",
		"child_start_offset",
		"child_end_offset",
		"parent_build_strategy",
		"parent_build_version",
		"parent_token_count",
	)

	parentChildAvailable := false
	if value, ok := doc.MetaData["parent_child_available"]; ok {
		parentChildAvailable = castBool(value)
	} else {
		parentChildAvailable = strings.TrimSpace(readMetadataString(doc, "parent_id")) != "" &&
			strings.TrimSpace(readMetadataString(doc, "child_id")) != ""
		doc.MetaData["parent_child_available"] = parentChildAvailable
	}
	source["parent_child_available"] = parentChildAvailable
	doc.MetaData["source"] = source
}

func copyMetadataFieldsToMap(target map[string]interface{}, metadata map[string]interface{}, keys ...string) {
	if target == nil || metadata == nil {
		return
	}
	for _, key := range keys {
		if value, exists := metadata[key]; exists && value != nil {
			target[key] = value
		}
	}
}

func castBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(v)), "true")
	}
}
```

然后看 dense 检索结果是怎么接入这套协议的：

```go
if i < len(result.Scores) {
	doc.MetaData["score"] = result.Scores[i]
}
doc.MetaData["retriever_version"] = DenseRetrieverVersion
source := ensureSourceMetadata(doc)
source["route"] = routeDense
source["retriever_version"] = DenseRetrieverVersion
if collectionName != "" {
	source["collection"] = collectionName
}
doc.MetaData["source"] = source
annotateParentChildSource(doc)
```

sparse 检索有两处都接了这套逻辑：

```go
doc.MetaData["route"] = "sparse"
doc.MetaData["retriever_version"] = hybridRetrieverVersion
source := ensureSourceMetadata(doc)
source["route"] = routeSparse
source["retriever_version"] = hybridRetrieverVersion
if collection != "" {
	source["collection"] = collection
}
doc.MetaData["source"] = source
annotateParentChildSource(doc)
```

```go
doc.MetaData["route"] = routeSparse
doc.MetaData["sparse_score"] = hit.Score
doc.MetaData["score"] = hit.Score
doc.MetaData["retriever_version"] = hybridRetrieverVersion
source := ensureSourceMetadata(doc)
source["route"] = routeSparse
source["retriever_version"] = hybridRetrieverVersion
if collection != "" {
	source["collection"] = collection
}
doc.MetaData["source"] = source
annotateParentChildSource(doc)
attachRewriteMetadata(doc, req)
```

hybrid dense 分支、rerank、dedupe 也都统一调用了这一个公共函数：

```go
doc.MetaData["source"] = source
annotateParentChildSource(doc)
attachRewriteMetadata(doc, req)
```

```go
doc.MetaData["source"] = source
annotateParentChildSource(doc)
```

```go
doc.MetaData["source"] = source
annotateParentChildSource(doc)
```

### 这段代码在做什么

这一步的核心不是“再复制一遍 metadata”，而是把 retrieval 返回协议统一成下面这个形状：

1. 顶层 `MetaData` 里有父子块字段。
2. `source` 里也有同一批关键引用字段。

这样做之后，上层消费方如果只关心引用信息，只看 `source` 就够了。

### 为什么要这样写

这是初学者最容易问的一句：

“既然顶层 `MetaData` 已经有了，为什么还要写进 `source`？”

原因很实际：

1. 检索返回里，`source` 本来就是最适合承载“来源引用协议”的地方。
2. 前端或其他调用方经常只读取 `source` 做引用展示，不会深入解析整份顶层 metadata。
3. 统一把引用相关字段都放进 `source`，上层代码会简单很多。

另外 `ensureSourceMetadata` 兼容了老数据里 `source` 还是字符串的情况。这也很关键，因为线上不可能所有旧数据都立刻重建。这个兼容逻辑会把旧的字符串来源保存在 `source["origin"]` 里，避免升级后把老信息丢掉。

`parent_child_available` 也值得特别注意。

它的设计意图是：

1. 新数据如果有完整父子块字段，就明确标记为 `true`。
2. 老数据如果没有 `parent_id` 和 `child_id`，就明确标记为 `false`。

这样上层调用方就不需要自己猜“这个 chunk 到底是老协议还是新协议”。

### 它如何衔接下一步

到这里，生产、入库、检索三个层面都已经贯通了。最后一步就是用测试把这些行为锁住，防止后面回归。

## 第六步：用测试锁住切块行为和 source 协议

### 目标

验证两件最容易被改坏的事：

1. 切块时是否真的生成了父子块 metadata。
2. retrieval 返回时 `source` 里是否真的带上了这些字段。

### 文件

1. `backend/internal/milvus/splitter/parent_child_test.go`
2. `backend/internal/milvus/retrieval/source_parent_child_test.go`

### 完整代码

```go
func TestSplitMarkdownDocumentAnnotatesHeadingHierarchy(t *testing.T) {
	service := mustNewTestSplitter(t, 90, 10)
	doc := &schema.Document{
		Content: "# Handbook\n\n## API Layer\nThe API layer handles routing, validation, authentication, throttling, and audit logging for every request.\nIt also normalizes request metadata before dispatch.\n\n## Storage Layer\nThe storage layer persists vectors and metadata for retrieval.\n",
		MetaData: map[string]interface{}{
			"document_id": uint64(42),
			"title":       "Handbook",
		},
	}

	chunks, err := service.SplitMarkdownDocument(context.Background(), doc)
	if err != nil {
		t.Fatalf("SplitMarkdownDocument failed: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected markdown chunks, got 0")
	}

	foundStructuredChunk := false
	for _, chunk := range chunks {
		if chunk == nil || chunk.MetaData == nil {
			t.Fatalf("expected chunk metadata to be present")
		}
		if chunk.MetaData["document_id"] != uint64(42) {
			t.Fatalf("expected document_id to be preserved, got %v", chunk.MetaData["document_id"])
		}
		if chunk.MetaData["child_id"] == "" || chunk.MetaData["parent_id"] == "" {
			t.Fatalf("expected child_id/parent_id to be populated, got child=%v parent=%v", chunk.MetaData["child_id"], chunk.MetaData["parent_id"])
		}
		if chunk.MetaData["chunk_id"] != chunk.MetaData["child_id"] {
			t.Fatalf("expected chunk_id to align with child_id")
		}
		if available, ok := chunk.MetaData["parent_child_available"].(bool); !ok || !available {
			t.Fatalf("expected parent_child_available=true, got %v", chunk.MetaData["parent_child_available"])
		}
		if section := asString(t, chunk.MetaData["section_title"]); section == "API Layer" {
			foundStructuredChunk = true
			if got := asString(t, chunk.MetaData["hierarchy_path"]); got != "Handbook > API Layer" {
				t.Fatalf("expected hierarchy path for API chunk, got %q", got)
			}
		}
	}

	if !foundStructuredChunk {
		t.Fatalf("expected at least one chunk to inherit API Layer hierarchy metadata")
	}
}

func TestSplitPreservesChildParentOffsets(t *testing.T) {
	service := mustNewTestSplitter(t, 70, 8)
	content := "First paragraph introduces the document.\n\nSecond paragraph contains the most relevant evidence.\n\nThird paragraph closes the explanation."
	doc := &schema.Document{
		Content: content,
		MetaData: map[string]interface{}{
			"document_id": uint64(1001),
			"title":       "Offset Spec",
		},
	}

	chunks, err := service.Split(context.Background(), []*schema.Document{doc})
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected chunks, got 0")
	}

	for _, chunk := range chunks {
		if chunk == nil || chunk.MetaData == nil {
			t.Fatalf("expected chunk metadata")
		}
		childStart := asInt(t, chunk.MetaData["child_start_offset"])
		childEnd := asInt(t, chunk.MetaData["child_end_offset"])
		parentStart := asInt(t, chunk.MetaData["parent_start_offset"])
		parentEnd := asInt(t, chunk.MetaData["parent_end_offset"])

		if childStart < 0 || childEnd < childStart || childEnd > len(content) {
			t.Fatalf("invalid child offsets: start=%d end=%d len=%d", childStart, childEnd, len(content))
		}
		if parentStart > childStart || parentEnd < childEnd {
			t.Fatalf("expected parent span to contain child span, parent=[%d,%d) child=[%d,%d)", parentStart, parentEnd, childStart, childEnd)
		}

		childSlice := content[childStart:childEnd]
		if !strings.Contains(childSlice, strings.TrimSpace(chunk.Content)) && !strings.Contains(chunk.Content, strings.TrimSpace(childSlice)) {
			t.Fatalf("expected chunk content to align with located child offsets, chunk=%q child_slice=%q", chunk.Content, childSlice)
		}
	}
}

func TestLongHeadingSectionUsesTruncatedParentWindow(t *testing.T) {
	service := mustNewTestSplitter(t, 40, 5)
	content := "# Guide\n\n## Deep Dive\nParagraph one explains the retrieval model in detail.\n\nParagraph two keeps adding more structured evidence for the same section.\n\nParagraph three extends the section so the parent block must be windowed.\n\nParagraph four pushes the section well beyond the configured parent window.\n"
	doc := &schema.Document{
		Content: content,
		MetaData: map[string]interface{}{
			"document_id": uint64(77),
			"title":       "Guide",
		},
	}

	chunks, err := service.SplitMarkdownDocument(context.Background(), doc)
	if err != nil {
		t.Fatalf("SplitMarkdownDocument failed: %v", err)
	}

	foundTruncated := false
	for _, chunk := range chunks {
		if chunk == nil || chunk.MetaData == nil {
			continue
		}
		if truncated, ok := chunk.MetaData["parent_truncated"].(bool); ok && truncated {
			foundTruncated = true
			if got := asString(t, chunk.MetaData["parent_build_strategy"]); got != parentStrategyHeadingWin {
				t.Fatalf("expected heading window strategy, got %q", got)
			}
			parentWidth := asInt(t, chunk.MetaData["parent_end_offset"]) - asInt(t, chunk.MetaData["parent_start_offset"])
			if parentWidth >= len(content) {
				t.Fatalf("expected truncated parent window smaller than full document, got %d vs %d", parentWidth, len(content))
			}
		}
	}

	if !foundTruncated {
		t.Fatalf("expected at least one chunk to record a truncated parent window")
	}
}
```

```go
func TestAnnotateParentChildSourceCopiesCitationFields(t *testing.T) {
	doc := &schema.Document{
		MetaData: map[string]interface{}{
			"source":                 "feishu",
			"document_id":            uint64(9),
			"chunk_id":               "doc-9-child-000",
			"child_id":               "doc-9-child-000",
			"parent_id":              "doc-9-parent-001",
			"section_title":          "Storage",
			"hierarchy_path":         "Guide > Storage",
			"parent_child_available": true,
		},
	}

	annotateParentChildSource(doc)

	source, ok := doc.MetaData["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source map, got %T", doc.MetaData["source"])
	}
	if source["origin"] != "feishu" {
		t.Fatalf("expected source origin to preserve legacy string value, got %v", source["origin"])
	}
	if source["child_id"] != "doc-9-child-000" {
		t.Fatalf("expected child_id in source, got %v", source["child_id"])
	}
	if source["parent_id"] != "doc-9-parent-001" {
		t.Fatalf("expected parent_id in source, got %v", source["parent_id"])
	}
	if source["section_title"] != "Storage" {
		t.Fatalf("expected section_title in source, got %v", source["section_title"])
	}
	if source["hierarchy_path"] != "Guide > Storage" {
		t.Fatalf("expected hierarchy_path in source, got %v", source["hierarchy_path"])
	}
	if source["parent_child_available"] != true {
		t.Fatalf("expected parent_child_available=true, got %v", source["parent_child_available"])
	}
}

func TestAnnotateParentChildSourceMarksLegacyChunksUnavailable(t *testing.T) {
	doc := &schema.Document{
		MetaData: map[string]interface{}{
			"document_id": uint64(3),
			"chunk_id":    "legacy-3",
		},
	}

	annotateParentChildSource(doc)

	source, ok := doc.MetaData["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected source map, got %T", doc.MetaData["source"])
	}
	if source["parent_child_available"] != false {
		t.Fatalf("expected legacy chunk to be marked parent_child_available=false, got %v", source["parent_child_available"])
	}
	if doc.MetaData["parent_child_available"] != false {
		t.Fatalf("expected doc metadata to record parent_child_available=false, got %v", doc.MetaData["parent_child_available"])
	}
}
```

### 这段代码在做什么

这些测试分别锁住了不同风险点：

1. `TestSplitMarkdownDocumentAnnotatesHeadingHierarchy`
   验证标题层级路径、`parent_id`、`child_id`、`parent_child_available` 是否正确生成。
2. `TestSplitPreservesChildParentOffsets`
   验证 child / parent 的 offset 是否合理，父块是否真的包住子块。
3. `TestLongHeadingSectionUsesTruncatedParentWindow`
   验证过长标题节是否真的退化成窗口化父块，并正确记录 `heading_section_window`。
4. `TestAnnotateParentChildSourceCopiesCitationFields`
   验证 retrieval 层是否真的把关键引用字段复制进 `source`。
5. `TestAnnotateParentChildSourceMarksLegacyChunksUnavailable`
   验证老 chunk 在没有父子字段时，会被明确标记为 `parent_child_available=false`。

### 为什么要这样写

这些测试没有去追求“覆盖率看起来更高”，而是盯住了最容易线上出事故的行为：

1. 标题路径算错，会导致引用解释错位。
2. offset 算错，会导致高亮和父块补全文本出错。
3. 超长标题节不做窗口化，会把父块变得太大，后续补全上下文会失控。
4. `source` 不带字段，上层引用协议就会悄悄失效。
5. 老数据不明确标记可用性，上层逻辑就会出现一堆“猜协议版本”的分支。

### 它如何衔接下一步

到这里，功能实现和回归保护就都具备了。接下来只剩验证和总结。

## 6. 如何验证

这次实现最直接的验证方式有三种。

### 6.1 跑单元测试

在 `backend` 目录执行：

```bash
go test ./internal/milvus/splitter ./internal/milvus/retrieval
```

这次我已经实际跑过，结果通过。

### 6.2 看切块后的 metadata

如果你想手动验证，可以重点看某个 chunk 的这些字段是否存在：

1. `chunk_id`
2. `child_id`
3. `parent_id`
4. `child_start_offset`
5. `child_end_offset`
6. `parent_start_offset`
7. `parent_end_offset`
8. `section_title`
9. `hierarchy_path`
10. `parent_build_strategy`
11. `parent_build_version`
12. `parent_child_available`

如果是 Markdown 标题结构清晰的文档，`hierarchy_path` 应该能看到类似：

```text
Handbook > API Layer
```

### 6.3 看 retrieval 返回的 source

检索结果里重点确认 `doc.MetaData["source"]` 是否是一个 map，并且包含：

1. `document_id`
2. `chunk_id`
3. `parent_id`
4. `child_id`
5. `section_title`
6. `hierarchy_path`
7. `parent_child_available`

如果是老数据，没有父子块信息，也应该看到：

```text
parent_child_available = false
```

这说明兼容逻辑已经生效。

## 7. 取舍与后续优化

这版实现已经把主链路打通了，但它有意保留了一些边界，没有一步做到最复杂。

### 7.1 这版主要优化了什么

这版主要优化的是“一致性”和“可落地性”：

1. 父子块 metadata 在切块阶段统一生产。
2. 导入、知识库入库、检索返回都走同一套字段协议。
3. 旧数据也能继续工作，不会因为 `source` 不是 map 就直接崩掉。

### 7.2 这版暂时没有解决什么

下面这些事情，这次没有继续做深：

1. 父块本身还没有独立建索引，也没有单独入库。
2. offset 现在按字符串匹配定位，如果未来遇到更多复杂归一化场景，可能还要再增强。
3. `parent_token_count` 现在是近似计算，不是精确 tokenizer 结果。
4. 这次主要兼容了 Markdown 标题结构和普通段落，还没有针对表格、复杂代码块、富文本结构做更细粒度建模。

### 7.3 下一步自然的演进方向

如果后面要继续做，可以沿着这几个方向迭代：

1. 基于 `parent_id + offset` 实现真正的父块补全文本能力。
2. 把父块从逻辑 metadata 升级成可选的独立索引层。
3. 引入更稳定的 tokenizer，替换近似 token 统计。
4. 在前端引用卡片里直接消费 `section_title`、`hierarchy_path`、offset，做更强的可解释展示。

## 8. 小结

这次升级最重要的不是新增了多少字段，而是把“父子块信息应该在哪一层产生、怎么一路带下去”这件事彻底理顺了。

你可以把整个实现记成下面三句话：

1. 父子块 metadata 应该在切块时生成，而不是切完后补。
2. 想在切块时生成这些信息，切块接口就必须接收完整文档，而不是只有纯文本。
3. 想让上层稳定消费这些信息，retrieval 返回协议就必须把关键字段统一带进 `source`。

这样一来，后面的引用、高亮、父块补全、可解释性展示，才真正有了稳定地基。

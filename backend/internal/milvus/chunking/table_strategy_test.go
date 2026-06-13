package chunking

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"interview-agents/internal/documentparser"
)

func TestTableAwareStrategyAddsAtomicTableChunks(t *testing.T) {
	doc := &documentparser.NormalizedDocument{
		ContentMarkdown: "# Fields\n\n| 字段 | 含义 |\n|---|---|\n| id | 编号 |\n",
		Source:          documentparser.NormalizedSource{FileName: "fields.pdf", FileType: "pdf"},
		Tables: []documentparser.NormalizedTable{
			{
				ID:            "t-001",
				Page:          2,
				MarkdownStart: 10,
				MarkdownEnd:   55,
				Rows: []documentparser.TableRow{
					{Cells: []documentparser.TableCell{{Text: "字段", IsHeader: true}, {Text: "含义", IsHeader: true}}},
					{Cells: []documentparser.TableCell{{Text: "id"}, {Text: "编号"}}},
				},
				Quality: documentparser.TableQuality{Status: "ok"},
			},
		},
		Quality:   documentparser.ParseQuality{Status: "ok", Score: 1},
		Extractor: documentparser.ExtractorInfo{Provider: "docling", Version: "v1"},
	}
	base := &recordingStrategy{name: "structure-aware"}
	strategy := NewTableAwareStrategy(base)

	chunks, err := strategy.Split(context.Background(), Request{
		Document:       doc,
		BaseMeta:       map[string]interface{}{"document_id": uint64(9)},
		NormalizedPath: "/tmp/fields.pdf.normalized.json",
	})
	if err != nil {
		t.Fatalf("Split returned error: %v", err)
	}
	if base.calls != 1 {
		t.Fatalf("expected delegate to be called once, got %d", base.calls)
	}

	var tableChunk *schema.Document
	for _, chunk := range chunks {
		if chunk.MetaData["chunking_unit"] == "table" {
			tableChunk = chunk
			break
		}
	}
	if tableChunk == nil {
		t.Fatalf("expected an atomic table chunk")
	}
	if tableChunk.MetaData["chunking_strategy"] != "table-aware" {
		t.Fatalf("chunking_strategy = %v", tableChunk.MetaData["chunking_strategy"])
	}
	if tableChunk.MetaData["document_id"] != uint64(9) {
		t.Fatalf("document_id = %v", tableChunk.MetaData["document_id"])
	}
	if tableChunk.MetaData["page_start"] != 2 || tableChunk.MetaData["page_end"] != 2 {
		t.Fatalf("page metadata = %v/%v", tableChunk.MetaData["page_start"], tableChunk.MetaData["page_end"])
	}
	if got := tableChunk.MetaData["table_ids"].([]string); len(got) != 1 || got[0] != "t-001" {
		t.Fatalf("table_ids = %v", got)
	}
	if tableChunk.MetaData["child_start_offset"] != 10 || tableChunk.MetaData["child_end_offset"] != 55 {
		t.Fatalf("child offsets = %v/%v", tableChunk.MetaData["child_start_offset"], tableChunk.MetaData["child_end_offset"])
	}
	if tableChunk.Content == "" || tableChunk.Content == doc.ContentMarkdown {
		t.Fatalf("expected dedicated table retrieval content, got %q", tableChunk.Content)
	}
}

func TestTableAwareStrategyKeepsLargeTableChunksWithinMilvusContentLimit(t *testing.T) {
	markdown := strings.TrimSpace(`
## 公共云地域单价**（元/百万次****）**

| **地域** | **计费阶梯** | **月度消息收发累计次数（亿次）** | **普通消息** | **定时/延时消息** | **事务消息** | **定时/延时消息** | **事务消息** | **顺序消息** |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **发送、消费** | **消费** | **消费** | **发送** | **发送** | **发送、消费** |
| 华东1（杭州）、华东2（上海）、华南1（深圳）、华北1（青岛）、华北2（北京）、华北3（张家口）、华北5（呼和浩特）、西南1（成都）、华北6（乌兰察布）、华南2（河源）、华南3（广州）、华东6（福州-本地地域-关停中）、华东5 （南京-本地地域-关停中）、郑州（联通云）、中国香港、新加坡、日本（东京）、德国（法兰克福）、英国（伦敦）、美国（硅谷）、美国（弗吉尼亚）、韩国（首尔）、泰国（曼谷）、马来西亚（吉隆坡）、印度尼西亚（雅加达）、菲律宾（马尼拉） | 第一阶梯 | (0,10\\] | 2   |   |   | 10  |   |   |
| 第二阶梯 | (10,50\\] | 1.2 |   |   | 6   |   |   |
| 第三阶梯 | (50,200\\] | 1   |   |   | 5   |   |   |
| 第四阶梯 | \\>200 | 0.8 |   |   | 4   |   |   |
| 阿联酋（迪拜） | 第一阶梯 | (0,10\\] | 4   |   |   | 20  |   |   |
| 第二阶梯 | (10,50\\] | 2.4 |   |   | 12  |   |   |
| 第三阶梯 | (50,200\\] | 2   |   |   | 10  |   |   |
| 第四阶梯 | \\>200 | 1.6 |   |   | 9   |   |   |
| 美国（亚特兰大） | 第一阶梯 | (0,10\\] | 2.7 |   |   | 13.5 |   |   |
| 第二阶梯 | (10,50\\] | 1.62 |   |   | 8.1 |   |   |
| 第三阶梯 | (50,200\\] | 1.35 |   |   | 6.75 |   |   |
| 第四阶梯 | \\>200 | 1.08 |   |   | 5.4 |   |   |
`)
	doc, err := documentparser.NormalizeLocal(context.Background(), documentparser.LocalRequest{
		FileName: "rocketmq-pricing.md",
		FileType: "md",
		Content:  []byte(markdown),
	})
	if err != nil {
		t.Fatalf("NormalizeLocal returned error: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected one parsed table, got %d", len(doc.Tables))
	}

	strategy := NewTableAwareStrategy(&shortChunkStrategy{})
	chunks, err := strategy.Split(context.Background(), Request{Document: doc})
	if err != nil {
		t.Fatalf("Split returned error: %v", err)
	}

	tableChunks := make([]*schema.Document, 0)
	for _, chunk := range chunks {
		if chunk == nil || chunk.MetaData["chunking_unit"] != "table" {
			continue
		}
		tableChunks = append(tableChunks, chunk)
		if length := len(chunk.Content); length > 4096 {
			t.Fatalf("table chunk content length = %d, want <= 4096", length)
		}
	}
	if len(tableChunks) < 2 {
		t.Fatalf("expected large table retrieval content to be split, got %d table chunks", len(tableChunks))
	}
	combined := strings.Join(func() []string {
		contents := make([]string, 0, len(tableChunks))
		for _, chunk := range tableChunks {
			contents = append(contents, chunk.Content)
		}
		return contents
	}(), "\n")
	if !strings.Contains(combined, "美国（亚特兰大）") || !strings.Contains(combined, "5.4") {
		t.Fatalf("expected split table chunks to preserve later rows, got %q", combined)
	}
}

type shortChunkStrategy struct{}

func (s *shortChunkStrategy) Split(_ context.Context, _ Request) ([]*schema.Document, error) {
	return []*schema.Document{{Content: "delegate", MetaData: map[string]interface{}{}}}, nil
}

package splitter

import (
	"strings"

	"interview-agents/internal/milvus/chunkmeta"

	"github.com/cloudwego/eino/schema"
)

func (s *DocumentSplitterService) shouldGenerateAgenticShadow(doc *schema.Document) bool {
	if s == nil || !s.agenticConfig.Enabled {
		return false
	}
	if strings.TrimSpace(s.agenticConfig.Mode) != chunkmeta.AgenticChunkingModeShadow {
		return false
	}
	if doc == nil {
		return false
	}
	if len([]rune(strings.TrimSpace(doc.Content))) > s.agenticConfig.MaxDocumentChars {
		return false
	}
	if len(s.agenticConfig.AllowedKBIDs) == 0 {
		return true
	}
	if doc.MetaData == nil {
		return false
	}
	if kbID, ok := doc.MetaData["kb_id"].(uint64); ok {
		_, allowed := s.agenticConfig.AllowedKBIDs[kbID]
		return allowed
	}
	return false
}

package retrieval

import (
	"strings"

	"github.com/bytedance/sonic"
)

func parseMilvusMetadata(raw interface{}) map[string]interface{} {
	switch value := raw.(type) {
	case nil:
		return map[string]interface{}{}
	case map[string]interface{}:
		return cloneMetadataMap(value)
	case string:
		return parseMilvusMetadataJSON([]byte(value))
	case []byte:
		return parseMilvusMetadataJSON(value)
	default:
		return map[string]interface{}{}
	}
}

func parseMilvusMetadataJSON(data []byte) map[string]interface{} {
	if len(data) == 0 || strings.TrimSpace(string(data)) == "" {
		return map[string]interface{}{}
	}

	var metadata map[string]interface{}
	if err := sonic.Unmarshal(data, &metadata); err != nil {
		return map[string]interface{}{}
	}
	if metadata == nil {
		return map[string]interface{}{}
	}
	return metadata
}

func cloneMetadataMap(source map[string]interface{}) map[string]interface{} {
	if source == nil {
		return nil
	}

	cloned := make(map[string]interface{}, len(source))
	for key, value := range source {
		cloned[key] = cloneMetadataValue(value)
	}
	return cloned
}

func cloneMetadataValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneMetadataMap(typed)
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for i, item := range typed {
			cloned[i] = cloneMetadataValue(item)
		}
		return cloned
	default:
		return typed
	}
}

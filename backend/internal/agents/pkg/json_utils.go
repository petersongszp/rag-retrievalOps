package pkg

import (
	"encoding/json"
)

// ExtractJSONFromResponse 从文本中提取 JSON 字符串
func ExtractJSONFromResponse(text string) string {
	// 查找对象格式 {...}
	start := -1
	braceCount := 0

	for i := 0; i < len(text); i++ {
		if text[i] == '{' {
			if start == -1 {
				start = i
			}
			braceCount++
		} else if text[i] == '}' {
			braceCount--
			if start != -1 && braceCount == 0 {
				jsonStr := text[start : i+1]
				// 尝试验证 JSON 是否有效
				var temp interface{}
				if err := json.Unmarshal([]byte(jsonStr), &temp); err == nil {
					return jsonStr
				}
			}
		}
	}

	return ""
}

// ExtractJSONArrayFromResponse 从文本中提取 JSON 数组字符串
func ExtractJSONArrayFromResponse(text string) string {
	// 查找数组格式 [...]
	start := -1
	bracketCount := 0

	for i := 0; i < len(text); i++ {
		if text[i] == '[' {
			if start == -1 {
				start = i
			}
			bracketCount++
		} else if text[i] == ']' {
			bracketCount--
			if start != -1 && bracketCount == 0 {
				jsonStr := text[start : i+1]
				// 尝试验证 JSON 是否有效
				var temp interface{}
				if err := json.Unmarshal([]byte(jsonStr), &temp); err == nil {
					return jsonStr
				}
			}
		}
	}

	return ""
}

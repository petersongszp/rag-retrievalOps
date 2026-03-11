package retrieval

import (
	"fmt"
	"strings"
)

// BuildFilterExpr 构建过滤表达式
func BuildFilterExpr(opts *RetrieveOptions) string {
	if opts == nil {
		return ""
	}

	// 如果提供了自定义表达式，直接使用
	if opts.Expr != "" {
		return opts.Expr
	}

	var conditions []string

	// 语言类型过滤
	if opts.Language != "" {
		// JSON 字段过滤：使用 Milvus JSON 字段访问语法 metadata['language']
		condition := fmt.Sprintf("metadata['language'] == '%s'", string(opts.Language))
		conditions = append(conditions, condition)
	}

	// 文档分类过滤
	if opts.Category != "" {
		// JSON 字段过滤：使用 Milvus JSON 字段访问语法 metadata['category']
		condition := fmt.Sprintf("metadata['category'] == '%s'", string(opts.Category))
		conditions = append(conditions, condition)
	}

	if len(conditions) == 0 {
		return ""
	}

	// 多个条件使用 AND 连接
	return strings.Join(conditions, " && ")
}

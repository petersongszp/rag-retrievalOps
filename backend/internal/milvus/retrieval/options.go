package retrieval

// DocumentLanguage 文档内容类型
type DocumentLanguage string

const (
	LanguageGolang     DocumentLanguage = "golang"
	LanguageJava       DocumentLanguage = "java"
	LanguageMiddleware DocumentLanguage = "middleware" // 中间件相关文档
)

// DocumentCategory 文档类别分类
type DocumentCategory string

const (
	CategorySpecialized   DocumentCategory = "专项" // 专项
	CategoryComprehensive DocumentCategory = "综合" // 综合
	CategoryBasic         DocumentCategory = "基础" // 基础
)

// RetrieveOptions 检索选项
type RetrieveOptions struct {
	// 语言类型过滤（可选）
	Language DocumentLanguage
	// 文档分类过滤（可选）
	Category DocumentCategory
	// 自定义过滤表达式（可选，优先级高于 Language 和 Category）
	Expr string
	// TopK 返回结果数量（可选，使用配置中的默认值）
	TopK int
	// 数据库名称（可选，如果指定则使用指定的数据库）
	Database string
	// 集合名称（可选，如果指定则使用指定的集合）
	Collection string
}

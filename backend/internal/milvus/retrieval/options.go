package retrieval

type DocumentLanguage string

const (
	LanguageGolang     DocumentLanguage = "golang"
	LanguageJava       DocumentLanguage = "java"
	LanguageMiddleware DocumentLanguage = "middleware"
)

type DocumentCategory string

const (
	CategorySpecialized   DocumentCategory = "专项"
	CategoryComprehensive DocumentCategory = "综合"
	CategoryBasic         DocumentCategory = "基础"
)

type RetrieveOptions struct {
	Language         DocumentLanguage
	Category         DocumentCategory
	Expr             string
	TopK             int
	CandidateTopK    int
	Database         string
	Collection       string
	RequestID        string
	KBScope          string
	ActiveGlobalKBID uint64
	OriginalQuery    string
	RewriteQuery     string
	FinalQuery       string
	RewriteStrategy  string
	RewriteApplied   bool
}

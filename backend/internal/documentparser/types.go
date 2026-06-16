package documentparser

import (
	"errors"
	"fmt"
	"strings"
)

const NormalizerVersion = "document-normalizer-v1"

type NormalizedDocument struct {
	ContentMarkdown    string               `json:"content_markdown"`
	ContentMarkdownRaw string               `json:"content_markdown_raw,omitempty"`
	Source             NormalizedSource     `json:"source"`
	Blocks             []NormalizedBlock    `json:"blocks,omitempty"`
	Tables             []NormalizedTable    `json:"tables,omitempty"`
	OmittedImages      []OmittedImage       `json:"omitted_images,omitempty"`
	Quality            ParseQuality         `json:"quality"`
	Extractor          ExtractorInfo        `json:"extractor"`
	Canonicalization   CanonicalizationInfo `json:"canonicalization,omitempty"`
}

type NormalizedSource struct {
	FileName   string `json:"file_name"`
	FileType   string `json:"file_type"`
	SourcePath string `json:"source_path,omitempty"`
	PageCount  int    `json:"page_count,omitempty"`
}

type NormalizedBlock struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Page          int       `json:"page,omitempty"`
	Text          string    `json:"text,omitempty"`
	MarkdownStart int       `json:"markdown_start,omitempty"`
	MarkdownEnd   int       `json:"markdown_end,omitempty"`
	BBox          []float64 `json:"bbox,omitempty"`
	Confidence    float64   `json:"confidence,omitempty"`
	TableID       string    `json:"table_id,omitempty"`
}

type NormalizedTable struct {
	ID            string       `json:"id"`
	Page          int          `json:"page,omitempty"`
	MarkdownStart int          `json:"markdown_start,omitempty"`
	MarkdownEnd   int          `json:"markdown_end,omitempty"`
	MergedCells   bool         `json:"merged_cells,omitempty"`
	Nested        bool         `json:"nested,omitempty"`
	ParentTableID string       `json:"parent_table_id,omitempty"`
	Rows          []TableRow   `json:"rows"`
	Quality       TableQuality `json:"quality"`
}

type TableRow struct {
	Cells []TableCell `json:"cells"`
}

type TableCell struct {
	Text     string `json:"text"`
	Rowspan  int    `json:"rowspan,omitempty"`
	Colspan  int    `json:"colspan,omitempty"`
	IsHeader bool   `json:"is_header,omitempty"`
}

type OmittedImage struct {
	Page   int       `json:"page,omitempty"`
	Reason string    `json:"reason"`
	BBox   []float64 `json:"bbox,omitempty"`
}

type ParseQuality struct {
	Status   string   `json:"status"`
	Score    float64  `json:"score,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

type TableQuality struct {
	Status   string   `json:"status"`
	Warnings []string `json:"warnings,omitempty"`
}

type ExtractorInfo struct {
	Provider string `json:"provider"`
	Version  string `json:"version"`
}

type CanonicalizationInfo struct {
	Version       string   `json:"version,omitempty"`
	AppliedRules  []string `json:"applied_rules,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	RawSHA1       string   `json:"raw_sha1,omitempty"`
	CanonicalSHA1 string   `json:"canonical_sha1,omitempty"`
}

type ProviderError struct {
	Code      string `json:"error_code"`
	Message   string `json:"message"`
	Stage     string `json:"stage,omitempty"`
	Page      int    `json:"page,omitempty"`
	Retryable bool   `json:"retryable"`
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	if e.Stage != "" && e.Page > 0 {
		return fmt.Sprintf("%s: %s at %s page %d", e.Code, e.Message, e.Stage, e.Page)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (d *NormalizedDocument) Validate() error {
	if d == nil {
		return errors.New("normalized document is nil")
	}
	if strings.TrimSpace(d.ContentMarkdown) == "" {
		return errors.New("normalized markdown content is empty")
	}
	if strings.TrimSpace(d.Source.FileName) == "" {
		return errors.New("source file name is empty")
	}
	if !IsSupportedType(d.Source.FileType) {
		return fmt.Errorf("unsupported normalized file type: %s", d.Source.FileType)
	}
	if strings.TrimSpace(d.Extractor.Provider) == "" {
		return errors.New("extractor provider is empty")
	}
	if strings.TrimSpace(d.Extractor.Version) == "" {
		return errors.New("extractor version is empty")
	}
	return nil
}

func NormalizeFileType(fileType string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileType)), ".")
}

func IsLocalType(fileType string) bool {
	switch NormalizeFileType(fileType) {
	case "txt", "md", "markdown":
		return true
	default:
		return false
	}
}

func IsProviderType(fileType string) bool {
	switch NormalizeFileType(fileType) {
	case "pdf", "docx", "html", "htm":
		return true
	default:
		return false
	}
}

func IsSupportedType(fileType string) bool {
	return IsLocalType(fileType) || IsProviderType(fileType)
}

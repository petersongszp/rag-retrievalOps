package kb

import (
	"mime/multipart"
	"testing"
)

func TestValidateKnowledgeFileSupportsNormalizedFormats(t *testing.T) {
	cases := []struct {
		name     string
		wantType string
	}{
		{name: "guide.pdf", wantType: "pdf"},
		{name: "guide.txt", wantType: "txt"},
		{name: "guide.md", wantType: "md"},
		{name: "guide.markdown", wantType: "markdown"},
		{name: "guide.docx", wantType: "docx"},
		{name: "guide.html", wantType: "html"},
		{name: "guide.htm", wantType: "htm"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateKnowledgeFile(&multipart.FileHeader{
				Filename: tc.name,
				Size:     128,
			})
			if err != nil {
				t.Fatalf("validateKnowledgeFile returned error: %v", err)
			}
			if got != tc.wantType {
				t.Fatalf("file type = %q, want %q", got, tc.wantType)
			}
		})
	}
}

func TestValidateKnowledgeFileRejectsUnsupportedFormat(t *testing.T) {
	_, err := validateKnowledgeFile(&multipart.FileHeader{
		Filename: "sheet.xlsx",
		Size:     128,
	})
	if err == nil {
		t.Fatalf("xlsx should be rejected")
	}
}

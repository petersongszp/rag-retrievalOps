package documentparser

import "testing"

func TestFileTypeRouting(t *testing.T) {
	cases := []struct {
		fileType      string
		wantLocal     bool
		wantProvider  bool
		wantSupported bool
	}{
		{fileType: "txt", wantLocal: true, wantSupported: true},
		{fileType: "md", wantLocal: true, wantSupported: true},
		{fileType: "markdown", wantLocal: true, wantSupported: true},
		{fileType: "pdf", wantProvider: true, wantSupported: true},
		{fileType: "docx", wantProvider: true, wantSupported: true},
		{fileType: "html", wantProvider: true, wantSupported: true},
		{fileType: "htm", wantProvider: true, wantSupported: true},
		{fileType: "xlsx", wantSupported: false},
	}

	for _, tc := range cases {
		t.Run(tc.fileType, func(t *testing.T) {
			if got := IsLocalType(tc.fileType); got != tc.wantLocal {
				t.Fatalf("IsLocalType(%q) = %v, want %v", tc.fileType, got, tc.wantLocal)
			}
			if got := IsProviderType(tc.fileType); got != tc.wantProvider {
				t.Fatalf("IsProviderType(%q) = %v, want %v", tc.fileType, got, tc.wantProvider)
			}
			if got := IsSupportedType(tc.fileType); got != tc.wantSupported {
				t.Fatalf("IsSupportedType(%q) = %v, want %v", tc.fileType, got, tc.wantSupported)
			}
		})
	}
}

func TestNormalizedDocumentValidate(t *testing.T) {
	valid := &NormalizedDocument{
		ContentMarkdown: "# Title\n\nBody",
		Source: NormalizedSource{
			FileName: "guide.md",
			FileType: "md",
		},
		Extractor: ExtractorInfo{
			Provider: "local",
			Version:  "document-normalizer-v1",
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid document returned error: %v", err)
	}

	invalid := &NormalizedDocument{
		Source: NormalizedSource{
			FileName: "empty.md",
			FileType: "md",
		},
	}
	if err := invalid.Validate(); err == nil {
		t.Fatalf("empty content should fail validation")
	}
}

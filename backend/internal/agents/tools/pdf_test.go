package tools

import (
	"context"
	"strings"
	"testing"
)

const testPDFPath = "C:\\Users\\LittleBear\\Desktop\\GoTest.pdf"

func TestExtractResume_Success(t *testing.T) {
	ctx := context.Background()

	t.Run("pdf_merge_mode", func(t *testing.T) {
		req := &ResumeExtractionRequest{
			FilePath:  testPDFPath,
			EnableOCR: true,
			Language:  "en",
		}

		result, err := ExtractResume(ctx, req)
		if err != nil {
			t.Fatalf("PDF简历解析失败: %v", err)
		}

		if !result.Success {
			t.Errorf("期望成功，实际失败: %s", result.ErrorMsg)
		}
		if result.TotalPages <= 0 {
			t.Errorf("期望总页数>0，实际=%d", result.TotalPages)
		}
		if result.RawText == "" {
			t.Error("提取的原始文本为空")
		}
		if result.Meta == nil {
			t.Error("Meta不应为nil")
		}
	})

	t.Run("chinese_pdf", func(t *testing.T) {
		req := &ResumeExtractionRequest{
			FilePath:  testPDFPath,
			EnableOCR: true,
			Language:  "zh",
		}

		result, err := ExtractResume(ctx, req)
		if err != nil {
			t.Fatalf("中文PDF简历解析失败: %v", err)
		}

		if !result.Success {
			t.Errorf("期望成功，实际失败: %s", result.ErrorMsg)
		}
	})

	t.Run("ocr_disabled", func(t *testing.T) {
		req := &ResumeExtractionRequest{
			FilePath:  testPDFPath,
			EnableOCR: false,
			Language:  "en",
		}

		result, err := ExtractResume(ctx, req)
		if err != nil {
			t.Fatalf("OCR禁用模式解析失败: %v", err)
		}

		if !result.Success {
			t.Errorf("期望成功，实际失败: %s", result.ErrorMsg)
		}
		if result.IsOCR {
			t.Error("OCR已禁用，但结果标记为使用了OCR")
		}
	})
}

func TestExtractResume_StructuredFields(t *testing.T) {
	ctx := context.Background()

	req := &ResumeExtractionRequest{
		FilePath:  testPDFPath,
		EnableOCR: true,
		Language:  "en",
	}

	result, err := ExtractResume(ctx, req)
	if err != nil {
		t.Fatalf("简历解析失败: %v", err)
	}

	if !result.Success {
		t.Fatalf("期望成功，实际失败: %s", result.ErrorMsg)
	}

	t.Run("structured_fields_populated", func(t *testing.T) {
		field := result.Structured
		if field.Email != "" {
			if !strings.Contains(field.Email, "@") {
				t.Errorf("邮箱格式不正确: %s", field.Email)
			}
		}
	})
}

func TestExtractResume_Error(t *testing.T) {
	ctx := context.Background()

	t.Run("empty_file_path", func(t *testing.T) {
		req := &ResumeExtractionRequest{FilePath: ""}
		result, err := ExtractResume(ctx, req)
		if err == nil {
			t.Error("空路径：期望返回错误，实际无错误")
		}
		if result.Success {
			t.Error("空路径：期望失败，实际成功")
		}
		if result.ErrorMsg == "" {
			t.Error("空路径：未返回错误信息")
		}
	})

	t.Run("file_not_exist", func(t *testing.T) {
		req := &ResumeExtractionRequest{FilePath: "/path/does/not/exist.pdf"}
		result, err := ExtractResume(ctx, req)
		if err == nil {
			t.Error("文件不存在：期望返回错误，实际无错误")
		}
		if result.Success {
			t.Error("文件不存在：期望失败，实际成功")
		}
		if result.ErrorMsg == "" {
			t.Error("文件不存在：未返回错误信息")
		}
	})

	t.Run("unsupported_format", func(t *testing.T) {
		req := &ResumeExtractionRequest{FilePath: "/path/to/file.txt"}
		result, err := ExtractResume(ctx, req)
		if err == nil {
			t.Error("不支持的格式：期望返回错误，实际无错误")
		}
		if result.Success {
			t.Error("不支持的格式：期望失败，实际成功")
		}
	})
}

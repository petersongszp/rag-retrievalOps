package tools

import (
	"context"
	"testing"
)

// 测试前请准备：
// 1. 一个文本型PDF文件（可复制文字），填写绝对路径到下方
// 2. 确保测试环境能访问该文件（无权限问题）
const testPDFPath = "C:\\Users\\LittleBear\\Desktop\\GoTest.pdf" // 替换为你的测试PDF绝对路径

// TestConvertPDFToText_Success 正常转换测试（合并模式+分页模式）
func TestConvertPDFToText_Success(t *testing.T) {
	ctx := context.Background()

	// 测试1：合并模式（默认）
	t.Run("merge_mode", func(t *testing.T) {
		req := &PDFToTextRequest{
			FilePath: testPDFPath,
			ToPages:  false,
		}

		result, err := ConvertPDFToText(ctx, req)
		if err != nil {
			t.Fatalf("合并模式转换失败: %v", err)
		}

		if !result.Success {
			t.Error("合并模式：期望成功，实际失败")
		}
		if result.TotalPages <= 0 {
			t.Errorf("合并模式：期望总页数>0，实际=%d", result.TotalPages)
		}
		if result.Content == "" {
			t.Error("合并模式：转换后文本为空")
		}
	})

	// 测试2：分页模式
	t.Run("page_mode", func(t *testing.T) {
		req := &PDFToTextRequest{
			FilePath: testPDFPath,
			ToPages:  true,
		}

		result, err := ConvertPDFToText(ctx, req)
		if err != nil {
			t.Fatalf("分页模式转换失败: %v", err)
		}

		if !result.Success {
			t.Error("分页模式：期望成功，实际失败")
		}
		if len(result.Pages) != result.TotalPages {
			t.Errorf("分页模式：页数不匹配，期望=%d，实际=%d", result.TotalPages, len(result.Pages))
		}
		for _, page := range result.Pages {
			if page.PageNum <= 0 {
				t.Errorf("分页模式：无效页码=%d", page.PageNum)
			}
			if page.Content == "" {
				t.Errorf("分页模式：第%d页文本为空", page.PageNum)
			}
		}
	})
}

// TestConvertPDFToText_Error 异常场景测试
func TestConvertPDFToText_Error(t *testing.T) {
	ctx := context.Background()

	// 测试用例：参数错误（空路径）
	t.Run("empty_file_path", func(t *testing.T) {
		req := &PDFToTextRequest{FilePath: ""}
		result, err := ConvertPDFToText(ctx, req)
		if err == nil {
			t.Error("空路径：期望返回错误，实际无错误")
		}
		if result.Success {
			t.Error("空路径：期望失败，实际成功")
		}
		if result.ErrorMsg != "参数错误：必须传入 file_path（本地PDF文件的绝对路径）" {
			t.Errorf("空路径：错误信息不匹配，实际=%s", result.ErrorMsg)
		}
	})

	// 测试用例：文件不存在
	t.Run("file_not_exist", func(t *testing.T) {
		req := &PDFToTextRequest{FilePath: "/path/does/not/exist.pdf"}
		result, err := ConvertPDFToText(ctx, req)
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

	// 测试用例：非PDF文件（可选，可替换为本地一个非PDF文件路径）
	t.Run("non_pdf_file", func(t *testing.T) {
		nonPDFPath := "/path/to/your/non-pdf.txt" // 替换为非PDF文件路径
		req := &PDFToTextRequest{FilePath: nonPDFPath}
		result, err := ConvertPDFToText(ctx, req)
		if err == nil {
			t.Error("非PDF文件：期望返回错误，实际无错误")
		}
		if result.Success {
			t.Error("非PDF文件：期望失败，实际成功")
		}
	})
}

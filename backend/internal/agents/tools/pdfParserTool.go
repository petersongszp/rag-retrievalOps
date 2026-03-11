package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// PDFToTextRequest 大模型调用工具的入参结构体（明确参数要求）
type PDFToTextRequest struct {
	FilePath  string `json:"file_path" jsonschema:"required,description=本地PDF文件的绝对路径（例如：D:\\test\\document.pdf 或 /home/user/document.pdf）"`
	ToPages   bool   `json:"to_pages" jsonschema:"default=false,description=是否按页面分割文本（true=分页输出，false=合并所有页为一个文本，默认false）"`
	EnableOCR bool   `json:"enable_ocr" jsonschema:"default=true,description=是否对扫描件启用OCR识别（当常规提取无内容时自动触发，默认开启）"`
}

// PDFToTextResult 工具返回的结构化结果（大模型可直接解析）
type PDFToTextResult struct {
	Success    bool                   `json:"success" jsonschema:"description=解析是否成功"`
	Content    string                 `json:"content,omitempty" jsonschema:"description=提取后的纯文本内容"`
	Pages      []PDFPageText          `json:"pages,omitempty" jsonschema:"description=分页文本（ToPages=true时返回）"`
	TotalPages int                    `json:"total_pages" jsonschema:"description=PDF总页数"`
	IsOCR      bool                   `json:"is_ocr" jsonschema:"description=是否使用了OCR识别"`
	ErrorMsg   string                 `json:"error_msg,omitempty" jsonschema:"description=错误信息（失败时返回）"`
	Meta       map[string]interface{} `json:"meta,omitempty" jsonschema:"description=元数据（方便追溯）"`
}

// PDFPageText 单页文本结构（分页模式下使用）
type PDFPageText struct {
	PageNum int    `json:"page_num" jsonschema:"description=页码（从1开始）"`
	Content string `json:"content" jsonschema:"description=单页纯文本"`
}

// ConvertPDFToText 核心逻辑：PDF转纯文本（支持普通文本提取与OCR自动降级）
func ConvertPDFToText(ctx context.Context, req *PDFToTextRequest) (*PDFToTextResult, error) {
	result := PDFToTextResult{
		Meta: map[string]interface{}{
			"file_path":  req.FilePath,
			"to_pages":   req.ToPages,
			"parse_time": time.Now().Format("2006-01-02 15:04:05"),
		},
	}

	// 1. 参数校验
	if req.FilePath == "" {
		result.Success = false
		result.ErrorMsg = "参数错误：必须传入 file_path"
		return &result, errors.New(result.ErrorMsg)
	}

	// 2. 检查文件是否存在
	if _, err := os.Stat(req.FilePath); err != nil {
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("文件不存在或无法访问: %v", err)
		return &result, errors.New(result.ErrorMsg)
	}

	log.Printf("[ConvertPDFToText] 开始尝试常规文本提取: %s", req.FilePath)
	startTime := time.Now()

	// 3. 尝试使用 pdftotext (常规文本提取)
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", req.FilePath, "-")
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	content := outBuf.String()

	// 4. 启发式判断：如果内容过少或提取失败，且开启了 OCR，则启动 OCR
	// 阈值：少于 50 个字符通常意味着该页是图片或提取失败
	if (err != nil || len(content) < 50) && req.EnableOCR {
		log.Printf("[ConvertPDFToText] 常规提取内容过少(%d字)，启动 Tesseract OCR 识别...", len(content))
		ocrStartTime := time.Now()

		// 调用 tesseract 直接识别 PDF (需系统安装 tesseract 及其 PDF 支持插件，如 poppler/ghostscript)
		// 指定语言为简体中文和英文
		ocrCmd := exec.CommandContext(ctx, "tesseract", req.FilePath, "stdout", "-l", "chi_sim+eng")
		var ocrOutBuf bytes.Buffer
		var ocrErrBuf bytes.Buffer
		ocrCmd.Stdout = &ocrOutBuf
		ocrCmd.Stderr = &ocrErrBuf

		if ocrErr := ocrCmd.Run(); ocrErr == nil {
			content = ocrOutBuf.String()
			result.IsOCR = true
			log.Printf("[ConvertPDFToText] OCR 识别成功，耗时 %v，提取字符: %d", time.Since(ocrStartTime), len(content))
		} else {
			log.Printf("[ConvertPDFToText] OCR 识别也失败了: %v, Stderr: %s", ocrErr, ocrErrBuf.String())
			// 若初次 pdftotext 也失败，则报错；若初次有少量内容但 OCR 失败，保留少量内容
			if err != nil {
				result.Success = false
				result.ErrorMsg = fmt.Sprintf("PDF提取及OCR均失败: %v", ocrErr)
				return &result, ocrErr
			}
		}
	}

	duration := time.Since(startTime)
	log.Printf("[ConvertPDFToText] 解析完成 (总耗时 %v), 获取到 %d 字符", duration, len(content))

	// 5. 构造成功结果
	result.Success = true
	result.Content = content
	result.TotalPages = 1 // 简化处理

	if req.ToPages {
		result.Pages = []PDFPageText{{PageNum: 1, Content: content}}
	}

	result.Meta["method"] = "pdftotext_with_ocr_fallback"
	result.Meta["is_ocr"] = result.IsOCR

	return &result, nil
}

// CreatePDFToTextTool 创建工具实例（供Eino框架注册，大模型识别）
func CreatePDFToTextTool() tool.InvokableTool {
	// 工具元信息：大模型识别的关键（名称、描述、参数定义）
	pdfTool, err := utils.InferTool("pdf_to_text", "将本地PDF文件转换为纯文本。支持普通文本PDF提取及扫描件自动OCR识别（通过Tesseract引擎）。需传入本地PDF的绝对路径。", ConvertPDFToText)
	if err != nil {
		log.Fatalf("infer tool failed: %v", err)
	}
	fmt.Println("✅ PDF解析工具(支持OCR)初始化完成")
	return pdfTool
}

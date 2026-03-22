package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/ledongthuc/pdf"
)

// ResumeExtractionRequest 英文简历解析入参（扩展原有PDF参数）
type ResumeExtractionRequest struct {
	FilePath  string `json:"file_path" jsonschema:"required,description=本地简历文件的绝对路径（PDF/DOCX格式，例如：/home/user/resume.pdf 或 C:\\resume.docx）"`
	EnableOCR bool   `json:"enable_ocr" jsonschema:"default=true,description=是否对扫描件PDF启用OCR识别（默认开启）"`
	Language  string `json:"language" jsonschema:"default=en,description=简历语言（en=英文，zh=中文，默认en）"`
}

// ResumeField 简历字段结构化结果
type ResumeField struct {
	Name           string   `json:"name" description="姓名"`
	Email          string   `json:"email" description="邮箱"`
	Phone          string   `json:"phone" description="电话号码（含国际区号）"`
	LinkedIn       string   `json:"linkedin" description="LinkedIn链接"`
	GitHub         string   `json:"github" description="GitHub链接"`
	Education      []string `json:"education" description="教育背景（学历/学校/专业/时间）"`
	WorkExperience []string `json:"work_experience" description="工作经历（公司/职位/时间/职责）"`
	Skills         []string `json:"skills" description="技能列表"`
	Location       string   `json:"location" description="所在地"`
	Summary        string   `json:"summary" description="个人简介/职业概述"`
}

// ResumeExtractionResult 简历解析最终结果
type ResumeExtractionResult struct {
	Success    bool                   `json:"success" description="解析是否成功"`
	RawText    string                 `json:"raw_text" description="提取的原始文本"`
	Structured ResumeField            `json:"structured" description="结构化的简历字段"`
	TotalPages int                    `json:"total_pages" description="文件总页数（PDF）/总段落（Word）"`
	IsOCR      bool                   `json:"is_ocr" description="是否使用了OCR识别"`
	ErrorMsg   string                 `json:"error_msg,omitempty" description="错误信息"`
	Meta       map[string]interface{} `json:"meta,omitempty" description="元数据"`
}

// 预编译正则表达式（英文简历字段匹配）
var (
	// 邮箱匹配
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	// 国际电话匹配（支持+xx xxxx xxxx、(xxx) xxx-xxxx等格式）
	phoneRegex = regexp.MustCompile(`(\+[0-9]{1,3}\s?)?(\([0-9]{1,4}\)|[0-9]{1,4})[-.\s]?[0-9]{1,4}[-.\s]?[0-9]{1,9}`)
	// LinkedIn链接匹配
	linkedinRegex = regexp.MustCompile(`(https?:\/\/)?(www\.)?linkedin\.com\/(in|profile)\/[a-zA-Z0-9\-_]+`)
	// GitHub链接匹配
	githubRegex = regexp.MustCompile(`(https?:\/\/)?(www\.)?github\.com\/[a-zA-Z0-9\-_]+`)
	// 教育背景关键词（英文）
	educationKeywords = []string{"Education", "Academic Background", "Degree", "University", "College", "School"}
	// 工作经历关键词（英文）
	workExpKeywords = []string{"Work Experience", "Professional Experience", "Employment History", "Career Summary", "Job Experience"}
	// 技能关键词（英文）
	skillKeywords = []string{"Skills", "Technical Skills", "Core Competencies", "Proficiencies", "Expertise"}
	// 个人简介关键词
	summaryKeywords = []string{"Summary", "Professional Summary", "Career Objective", "About Me", "Profile"}
)

// ConvertFileToText 通用文件转文本（支持PDF/DOCX）
func ConvertFileToText(ctx context.Context, req *ResumeExtractionRequest) (string, int, bool, error) {
	var rawText string
	var totalPages int
	var isOCR bool

	// 1. 判断文件类型
	dotIndex := strings.LastIndex(req.FilePath, ".")
	var fileExt string
	if dotIndex > 0 && dotIndex < len(req.FilePath)-1 {
		fileExt = strings.ToLower(req.FilePath[dotIndex+1:])
	} else {
		fileExt = ""
	}

	// 2. 不同文件类型的文本提取逻辑
	switch fileExt {
	case "pdf":
		text, pages, ocrUsed, err := extractPDFText(ctx, req)
		if err != nil {
			return "", 0, false, err
		}
		rawText = text
		totalPages = pages
		isOCR = ocrUsed

	case "docx":
		text, err := extractDOCXText(ctx, req.FilePath)
		if err != nil {
			return "", 0, false, err
		}
		rawText = text
		totalPages = countDocxParagraphs(text) // 用段落数模拟页数

	default:
		return "", 0, false, fmt.Errorf("不支持的文件格式：%s（仅支持PDF/DOCX）", fileExt)
	}

	// 3. 文本清洗（去除多余空白、特殊字符）
	rawText = cleanResumeText(rawText)
	return rawText, totalPages, isOCR, nil
}

// extractPDFText PDF文本提取（优化英文OCR+精准页数计算）
func extractPDFText(ctx context.Context, req *ResumeExtractionRequest) (string, int, bool, error) {
	// 检查文件存在性
	if _, err := os.Stat(req.FilePath); err != nil {
		return "", 0, false, fmt.Errorf("文件不存在或无法访问: %v", err)
	}

	// 第一步：尝试pdftotext提取（优化英文排版）
	cmd := exec.CommandContext(ctx, "pdftotext", "-enc", "UTF-8", "-layout", req.FilePath, "-")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	content := outBuf.String()
	totalPages := getPDFPageCount(req.FilePath) // 精准获取PDF页数

	// 第二步：判断是否需要OCR（英文简历阈值调整为100字符）
	isOCR := false
	if (err != nil || len(strings.TrimSpace(content)) < 100) && req.EnableOCR {
		log.Printf("[ResumeExtraction] PDF常规提取内容不足，启动OCR（英文优化）")
		// 优化OCR语言配置：优先英文，支持多语言
		lang := "eng"
		if req.Language == "zh" {
			lang = "chi_sim+eng"
		}
		ocrCmd := exec.CommandContext(ctx, "tesseract", req.FilePath, "stdout", "-l", lang, "--oem", "1", "--psm", "6")
		var ocrOutBuf, ocrErrBuf bytes.Buffer
		ocrCmd.Stdout = &ocrOutBuf
		ocrCmd.Stderr = &ocrErrBuf

		if ocrErr := ocrCmd.Run(); ocrErr == nil {
			content = ocrOutBuf.String()
			isOCR = true
		} else {
			log.Printf("[ResumeExtraction] OCR失败: %v", ocrErr)
			if err != nil {
				return "", 0, false, fmt.Errorf("PDF提取及OCR均失败: %v", ocrErr)
			}
		}
	}

	return content, totalPages, isOCR, nil
}

// extractDOCXText Word(docx)文本提取（依赖pandoc）
func extractDOCXText(ctx context.Context, filePath string) (string, error) {
	// 使用pandoc将docx转为纯文本（需系统安装pandoc）
	cmd := exec.CommandContext(ctx, "pandoc", "-s", filePath, "-t", "plain", "--wrap=none")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("DOCX转文本失败（需安装pandoc）: %v, stderr: %s", err, errBuf.String())
	}
	return outBuf.String(), nil
}

// getPDFPageCount 获取PDF精准页数
func getPDFPageCount(filePath string) int {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		log.Printf("[getPDFPageCount] 获取页数失败: %v，默认返回1", err)
		return 1
	}
	defer func() {
		_ = f.Close()
	}()
	return r.NumPage()
}

// countDocxParagraphs 统计DOCX文本段落数（模拟页数）
func countDocxParagraphs(text string) int {
	paragraphs := strings.Split(text, "\n\n")
	count := 0
	for _, p := range paragraphs {
		if len(strings.TrimSpace(p)) > 0 {
			count++
		}
	}
	return count
}

// cleanResumeText 简历文本清洗（去除多余空白、特殊字符）
func cleanResumeText(text string) string {
	// 替换多个空白为单个空格
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")
	// 去除不可见字符
	text = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			return r
		}
		return -1
	}, text)
	return strings.TrimSpace(text)
}

// ExtractEnglishResumeFields 英文简历字段结构化提取dd
func ExtractEnglishResumeFields(rawText string) ResumeField {
	var field ResumeField
	text := strings.ToLower(rawText) // 统一转为小写便于匹配

	// 1. 基础联系信息提取
	field.Email = extractFirstMatch(emailRegex, rawText)
	field.Phone = extractFirstMatch(phoneRegex, rawText)
	field.LinkedIn = extractFirstMatch(linkedinRegex, rawText)
	field.GitHub = extractFirstMatch(githubRegex, rawText)

	// 2. 姓名提取（简单规则：文本开头的大写单词组合，适配英文姓名格式）
	name := extractNameFromResume(rawText)
	if name != "" {
		field.Name = name
	}

	// 3. 教育背景提取（传入小写text，避免重复转小写）
	field.Education = extractSectionByKeywords(rawText, educationKeywords, text)

	// 4. 工作经历提取
	field.WorkExperience = extractSectionByKeywords(rawText, workExpKeywords, text)

	// 5. 技能提取
	field.Skills = extractSectionByKeywords(rawText, skillKeywords, text)

	// 6. 个人简介提取
	summary := extractSectionByKeywords(rawText, summaryKeywords, text)
	if len(summary) > 0 {
		field.Summary = strings.Join(summary, " ")
	}

	// 7. 所在地提取（简单匹配城市/国家关键词）
	locationRegex := regexp.MustCompile(`(?i)(Location|Based in):?\s*([A-Za-z\s]+)`)
	locMatch := locationRegex.FindStringSubmatch(rawText)
	if len(locMatch) >= 3 {
		field.Location = strings.TrimSpace(locMatch[2])
	}

	return field
}

// extractFirstMatch 提取第一个正则匹配结果
func extractFirstMatch(re *regexp.Regexp, text string) string {
	matches := re.FindStringSubmatch(text)
	if len(matches) > 0 {
		return strings.TrimSpace(matches[0])
	}
	return ""
}

// extractNameFromResume 提取英文简历姓名（开头大写单词）
func extractNameFromResume(text string) string {
	// 拆分文本行为行数组
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 匹配开头的大写单词组合（英文姓名特征）
		nameRegex := regexp.MustCompile(`^[A-Z][a-zA-Z]+(\s+[A-Z][a-zA-Z]+)+`)
		nameMatch := nameRegex.FindString(line)
		if nameMatch != "" && len(nameMatch) > 2 && len(nameMatch) < 50 {
			return nameMatch
		}
	}
	return ""
}

// extractSectionByKeywords 按关键词提取简历章节内容
func extractSectionByKeywords(rawText string, keywords []string, lowerText string) []string {
	lines := strings.Split(rawText, "\n")
	var result []string
	inTargetSection := false
	sectionStartIdx := -1

	// 第一步：找到目标章节的起始位置（用提前转好的小写文本匹配，提升性能）
	for i, line := range lines {
		lineLower := strings.ToLower(line)
		// 匹配关键词（支持关键词+冒号/换行）
		for _, kw := range keywords {
			kwLower := strings.ToLower(kw)
			if strings.Contains(lowerText, kwLower) && strings.Contains(lineLower, kwLower) && len(strings.TrimSpace(line)) < 100 {
				inTargetSection = true
				sectionStartIdx = i + 1 // 从下一行开始提取
				break
			}
		}
		if inTargetSection {
			break
		}
	}

	// 第二步：提取章节内容直到下一个关键词章节
	if sectionStartIdx != -1 && sectionStartIdx < len(lines) {
		for i := sectionStartIdx; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			lineLower := strings.ToLower(line)
			// 遇到下一个简历章节关键词则停止
			nextSection := false
			allKeywords := append(educationKeywords, append(workExpKeywords, skillKeywords...)...)
			for _, kw := range allKeywords {
				if strings.Contains(lineLower, strings.ToLower(kw)) {
					nextSection = true
					break
				}
			}
			if nextSection || line == "" || i == len(lines)-1 {
				break
			}
			// 过滤无效行（过短/纯符号）
			if len(line) > 3 && !strings.Contains(line, "==========") {
				result = append(result, line)
			}
		}
	}

	return result
}

// ExtractResume 核心入口：英文简历解析（PDF/DOCX）
func ExtractResume(ctx context.Context, req *ResumeExtractionRequest) (*ResumeExtractionResult, error) {
	result := ResumeExtractionResult{
		Meta: map[string]interface{}{
			"file_path":  req.FilePath,
			"language":   req.Language,
			"parse_time": time.Now().Format("2006-01-02 15:04:05"),
		},
	}

	// 1. 参数校验
	if req.FilePath == "" {
		result.Success = false
		result.ErrorMsg = "参数错误：必须传入file_path"
		return &result, errors.New(result.ErrorMsg)
	}

	// 2. 文件转文本
	rawText, totalPages, isOCR, err := ConvertFileToText(ctx, req)
	if err != nil {
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("文本提取失败: %v", err)
		return &result, err
	}

	// 3. 结构化字段提取
	structured := ExtractEnglishResumeFields(rawText)

	// 4. 构造成功结果
	result.Success = true
	result.RawText = rawText
	result.Structured = structured
	result.TotalPages = totalPages
	result.IsOCR = isOCR
	result.Meta["is_ocr"] = isOCR
	result.Meta["raw_text_length"] = len(rawText)

	log.Printf("[ExtractResume] 简历解析完成，提取字段：姓名=%s，邮箱=%s，页数=%d", structured.Name, structured.Email, totalPages)
	return &result, nil
}

// CreateResumeExtractionTool 创建简历解析工具实例（供Eino框架注册）
func CreateResumeExtractionTool() tool.InvokableTool {
	resumeTool, err := utils.InferTool(
		"resume_extraction",
		"解析海外英文简历（支持PDF/DOCX格式），提取姓名、邮箱、电话、LinkedIn、工作经历、教育背景、技能等结构化字段。需传入本地简历文件的绝对路径。",
		ExtractResume,
	)
	if err != nil {
		log.Fatalf("创建简历解析工具失败: %v", err)
	}
	fmt.Println("✅ 英文简历解析工具初始化完成")
	return resumeTool
}

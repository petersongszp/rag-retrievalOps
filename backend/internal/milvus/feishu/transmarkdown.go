package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
)

// SDK 使用文档：https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/server-side-sdk/golang-sdk-guide/preparations
// 复制该 Demo 后, 需要将 "YOUR_APP_ID", "YOUR_APP_SECRET" 替换为自己应用的 APP_ID, APP_SECRET.
// 以下示例代码默认根据文档示例值填充，如果存在代码问题，请在 API 调试台填上相关必要参数后再复制代码使用

// 飞书文档块的顶层结构（API 响应的 data.items 数组元素）
type FeishuBlock struct {
	BlockID   string        `json:"block_id"`           // 块ID
	BlockType int           `json:"block_type"`         // 块类型
	ParentID  string        `json:"parent_id"`          // 父块ID（用于处理嵌套，如列表）
	Page      *PageBlock    `json:"page,omitempty"`     // block_type=1：文档标题块
	Text      *TextBlock    `json:"text,omitempty"`     // block_type=2：普通文本块
	Heading1  *HeadingBlock `json:"heading1,omitempty"` // block_type=3：一级标题
	Heading2  *HeadingBlock `json:"heading2,omitempty"` // block_type=4：二级标题
	Heading3  *HeadingBlock `json:"heading3,omitempty"` // block_type=5：三级标题
	Heading4  *HeadingBlock `json:"heading4,omitempty"` // block_type=6：四级标题
	Heading5  *HeadingBlock `json:"heading5,omitempty"` // block_type=7：五级标题
	Heading6  *HeadingBlock `json:"heading6,omitempty"` // block_type=8：六级标题
	Heading7  *HeadingBlock `json:"heading7,omitempty"` // block_type=9：七级标题
	Heading8  *HeadingBlock `json:"heading8,omitempty"` // block_type=10：八级标题
	Heading9  *HeadingBlock `json:"heading9,omitempty"` // block_type=11：九级标题
	Bullet    *BulletBlock  `json:"bullet,omitempty"`   // block_type=12：无序列表
	Ordered   *OrderedBlock `json:"ordered,omitempty"`  // block_type=13：有序列表
	Code      *CodeBlock    `json:"code,omitempty"`     // block_type=14：代码块
	Quote     *QuoteBlock   `json:"quote,omitempty"`    // block_type=15：引用块
	Callout   *CalloutBlock `json:"callout,omitempty"`  // block_type=16：标注块
	Divider   *DividerBlock `json:"divider,omitempty"`  // block_type=17：分割线
	Image     *ImageBlock   `json:"image,omitempty"`    // block_type=18：图片
	Table     *TableBlock   `json:"table,omitempty"`    // block_type=19：表格
}

// PageBlock：文档标题块（block_type=1）
type PageBlock struct {
	Elements []TextElement `json:"elements"`
	Style    struct{}      `json:"style"` // 暂无需处理样式
}

// TextBlock：普通文本块（block_type=2）
type TextBlock struct {
	Elements []TextElement `json:"elements"`
	Style    struct{}      `json:"style"`
}

// HeadingBlock：标题块（heading1-heading9 通用）
type HeadingBlock struct {
	Elements []TextElement `json:"elements"`
	Style    struct{}      `json:"style"`
}

// BulletBlock：无序列表块
type BulletBlock struct {
	Elements []TextElement `json:"elements"`
	Style    struct{}      `json:"style"`
}

// OrderedBlock：有序列表块
type OrderedBlock struct {
	Elements []TextElement `json:"elements"`
	Style    struct{}      `json:"style"`
}

// CodeBlock：代码块
type CodeBlock struct {
	Elements []TextElement `json:"elements"`
	Language int           `json:"language"` // 语言类型
	Wrap     bool          `json:"wrap"`
	Style    struct{}      `json:"style"`
}

// QuoteBlock：引用块
type QuoteBlock struct {
	Elements []TextElement `json:"elements"`
	Style    struct{}      `json:"style"`
}

// CalloutBlock：标注块
type CalloutBlock struct {
	Elements []TextElement `json:"elements"`
	Style    struct{}      `json:"style"`
}

// DividerBlock：分割线块
type DividerBlock struct {
	Style struct{} `json:"style"`
}

// ImageBlock：图片块
type ImageBlock struct {
	Token  string   `json:"token"`
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Style  struct{} `json:"style"`
}

// TableBlock：表格块
type TableBlock struct {
	Rows    int      `json:"rows"`
	Columns int      `json:"columns"`
	Style   struct{} `json:"style"`
}

// DriveListFilesResponse：飞书云文档文件列表响应（简化）
type DriveListFilesResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Files []DriveFile `json:"files"`
	} `json:"data"`
}

// DriveFile：飞书云文档文件信息
type DriveFile struct {
	Name         string `json:"name"`
	Token        string `json:"token"`
	Type         string `json:"type"`
	URL          string `json:"url"`
	CreatedTime  string `json:"created_time,omitempty"`
	ModifiedTime string `json:"modified_time,omitempty"`
	OwnerID      string `json:"owner_id,omitempty"`
	ParentToken  string `json:"parent_token,omitempty"`
}

// TextElement：文本元素（text_run/mention_user 等）
type TextElement struct {
	TextRun     *TextRun     `json:"text_run,omitempty"`     // 普通文本
	MentionUser *MentionUser `json:"mention_user,omitempty"` // 提及用户
}

// TextRun：普通文本内容
type TextRun struct {
	Content          string            `json:"content"` // 文本内容
	TextElementStyle *TextElementStyle `json:"text_element_style,omitempty"`
}

// MentionUser：提及用户
type MentionUser struct {
	UserID           string            `json:"user_id"` // 用户ID（可通过飞书API查询用户名，这里先占位）
	TextElementStyle *TextElementStyle `json:"text_element_style,omitempty"`
}

// TextElementStyle：文本样式（粗体/斜体等，暂无需处理，保留结构）
type TextElementStyle struct {
	Bold          bool `json:"bold"`
	InlineCode    bool `json:"inline_code"`
	Italic        bool `json:"italic"`
	Strikethrough bool `json:"strikethrough"`
	Underline     bool `json:"underline"`
}

// 转换飞书块列表为 Markdown 字符串
func BlocksToMarkdown(blocks []FeishuBlock) string {
	var mdBuilder strings.Builder

	// 递归解析单个块
	var parseBlock func(block FeishuBlock)
	parseBlock = func(block FeishuBlock) {
		switch block.BlockType {
		case 1: // 文档标题（page 块）
			if block.Page != nil {
				content := parseTextElements(block.Page.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("# %s\n\n", content))
				}
			}
		case 2: // 普通文本块（text 块）
			if block.Text != nil {
				content := parseTextElements(block.Text.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("%s\n\n", content))
				}
			}
		case 3: // 一级标题（heading1 块）
			if block.Heading1 != nil {
				content := parseTextElements(block.Heading1.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("# %s\n\n", content))
				}
			}
		case 4: // 二级标题（heading2 块）
			if block.Heading2 != nil {
				content := parseTextElements(block.Heading2.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("## %s\n\n", content))
				}
			}
		case 5: // 三级标题（heading3 块）
			if block.Heading3 != nil {
				content := parseTextElements(block.Heading3.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("### %s\n\n", content))
				}
			}
		case 6: // 四级标题（heading4 块）
			if block.Heading4 != nil {
				content := parseTextElements(block.Heading4.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("#### %s\n\n", content))
				}
			}
		case 7: // 五级标题（heading5 块）
			if block.Heading5 != nil {
				content := parseTextElements(block.Heading5.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("##### %s\n\n", content))
				}
			}
		case 8: // 六级标题（heading6 块）
			if block.Heading6 != nil {
				content := parseTextElements(block.Heading6.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("###### %s\n\n", content))
				}
			}
		case 9: // 七级标题（heading7 块）
			if block.Heading7 != nil {
				content := parseTextElements(block.Heading7.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("####### %s\n\n", content))
				}
			}
		case 10: // 八级标题（heading8 块）
			if block.Heading8 != nil {
				content := parseTextElements(block.Heading8.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("######## %s\n\n", content))
				}
			}
		case 11: // 九级标题（heading9 块）
			if block.Heading9 != nil {
				content := parseTextElements(block.Heading9.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("######### %s\n\n", content))
				}
			}
		case 12: // 无序列表（bullet 块）
			if block.Bullet != nil {
				content := parseTextElements(block.Bullet.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("- %s\n", content))
				}
			}
		case 13: // 有序列表（ordered 块）
			if block.Ordered != nil {
				content := parseTextElements(block.Ordered.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("1. %s\n", content))
				}
			}
		case 14: // 代码块（code 块）
			if block.Code != nil {
				content := parseTextElements(block.Code.Elements)
				language := getCodeLanguage(block.Code.Language)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", language, content))
				}
			}
		case 15: // 引用块（quote 块）
			if block.Quote != nil {
				content := parseTextElements(block.Quote.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("> %s\n\n", strings.ReplaceAll(content, "\n", "\n> ")))
				}
			}
		case 16: // 标注块（callout 块）
			if block.Callout != nil {
				content := parseTextElements(block.Callout.Elements)
				if content != "" {
					mdBuilder.WriteString(fmt.Sprintf("> 💡 %s\n\n", content))
				}
			}
		case 17: // 分割线（divider 块）
			mdBuilder.WriteString("---\n\n")
		case 18: // 图片（image 块）
			if block.Image != nil {
				mdBuilder.WriteString(fmt.Sprintf("![image](%s)\n\n", block.Image.Token))
			}
		case 19: // 表格（table 块）
			if block.Table != nil {
				mdBuilder.WriteString(fmt.Sprintf("<!-- 表格: %dx%d -->\n\n", block.Table.Rows, block.Table.Columns))
			}
		default:
			// 未知块类型，尝试从各种可能的字段提取文本内容
			content := extractTextFromBlock(block)
			if content != "" {
				mdBuilder.WriteString(fmt.Sprintf("%s\n\n", content))
			} else {
				// 如果都没有，至少输出块类型信息
				mdBuilder.WriteString(fmt.Sprintf("<!-- 未处理的块类型: %d, block_id: %s -->\n\n", block.BlockType, block.BlockID))
			}
		}
	}

	// 遍历所有块解析
	for _, block := range blocks {
		parseBlock(block)
	}

	return mdBuilder.String()
}

// 从块中提取文本内容（尝试所有可能的字段）
func extractTextFromBlock(block FeishuBlock) string {
	// 尝试从 Text 字段提取
	if block.Text != nil {
		return parseTextElements(block.Text.Elements)
	}
	// 尝试从各种标题字段提取
	if block.Heading1 != nil {
		return parseTextElements(block.Heading1.Elements)
	}
	if block.Heading2 != nil {
		return parseTextElements(block.Heading2.Elements)
	}
	if block.Heading3 != nil {
		return parseTextElements(block.Heading3.Elements)
	}
	if block.Heading4 != nil {
		return parseTextElements(block.Heading4.Elements)
	}
	if block.Heading5 != nil {
		return parseTextElements(block.Heading5.Elements)
	}
	if block.Heading6 != nil {
		return parseTextElements(block.Heading6.Elements)
	}
	if block.Heading7 != nil {
		return parseTextElements(block.Heading7.Elements)
	}
	if block.Heading8 != nil {
		return parseTextElements(block.Heading8.Elements)
	}
	if block.Heading9 != nil {
		return parseTextElements(block.Heading9.Elements)
	}
	// 尝试从列表字段提取
	if block.Bullet != nil {
		return parseTextElements(block.Bullet.Elements)
	}
	if block.Ordered != nil {
		return parseTextElements(block.Ordered.Elements)
	}
	// 尝试从代码块提取
	if block.Code != nil {
		return parseTextElements(block.Code.Elements)
	}
	// 尝试从引用块提取
	if block.Quote != nil {
		return parseTextElements(block.Quote.Elements)
	}
	// 尝试从标注块提取
	if block.Callout != nil {
		return parseTextElements(block.Callout.Elements)
	}
	// 尝试从 Page 块提取
	if block.Page != nil {
		return parseTextElements(block.Page.Elements)
	}
	return ""
}

// 获取代码语言名称
func getCodeLanguage(langCode int) string {
	langMap := map[int]string{
		1:  "plaintext",
		2:  "abap",
		3:  "ada",
		4:  "arduino",
		5:  "autoit",
		6:  "c",
		7:  "clojure",
		8:  "coffeescript",
		9:  "cpp",
		10: "csharp",
		11: "css",
		12: "dart",
		13: "delphi",
		14: "dockerfile",
		15: "erlang",
		16: "fortran",
		17: "foxpro",
		18: "go",
		19: "groovy",
		20: "haskell",
		21: "html",
		22: "java",
		23: "javascript",
		24: "json",
		25: "julia",
		26: "kotlin",
		27: "latex",
		28: "lisp",
		29: "lua",
		30: "matlab",
		31: "nginx",
		32: "objectivec",
		33: "perl",
		34: "php",
		35: "powershell",
		36: "prolog",
		37: "protobuf",
		38: "python",
		39: "r",
		40: "ruby",
		41: "rust",
		42: "scala",
		43: "scheme",
		44: "shell",
		45: "sql",
		46: "swift",
		47: "thrift",
		48: "typescript",
		49: "vb",
		50: "verilog",
		51: "vhdl",
		52: "xml",
		53: "yaml",
	}
	if lang, ok := langMap[langCode]; ok {
		return lang
	}
	return "plaintext"
}

// sanitizeFileName 将文档名转换为安全的文件名
func sanitizeFileName(name string) string {
	// 去掉路径不支持的字符
	invalid := regexp.MustCompile(`[\\/:*?"<>|]`)
	name = invalid.ReplaceAllString(name, "_")
	name = strings.TrimSpace(name)
	if name == "" {
		return "untitled"
	}
	return name
}

// FetchDocumentToMarkdown 获取单个 docx 文档并转换为 Markdown，返回 markdown 字符串
func FetchDocumentToMarkdown(ctx context.Context, appID, appSecret, documentID, userAccessToken string) (string, error) {
	// 创建 Client
	client := lark.NewClient(appID, appSecret)

	// 创建请求对象
	req := larkdocx.NewListDocumentBlockReqBuilder().
		DocumentId(documentID).
		PageSize(500).
		DocumentRevisionId(-1).
		Build()

	// 发起请求
	resp, err := client.Docx.V1.DocumentBlock.List(ctx, req, larkcore.WithUserAccessToken(userAccessToken))
	if err != nil {
		return "", fmt.Errorf("请求失败：%w", err)
	}

	// 服务端错误处理
	if !resp.Success() {
		return "", fmt.Errorf("飞书API返回错误：logId=%s, error=%s", resp.RequestId(), larkcore.Prettify(resp.CodeError))
	}

	// 将响应转换为 JSON 以便解析
	respJSON, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("序列化响应失败：%w", err)
	}

	// 解析 JSON 响应
	var apiResponse struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Items []FeishuBlock `json:"items"`
		} `json:"data"`
	}

	err = json.Unmarshal(respJSON, &apiResponse)
	if err != nil {
		return "", fmt.Errorf("解析响应失败：%w", err)
	}

	if apiResponse.Code != 0 {
		return "", fmt.Errorf("飞书API返回错误：code=%d, msg=%s", apiResponse.Code, apiResponse.Msg)
	}

	if len(apiResponse.Data.Items) == 0 {
		return "", fmt.Errorf("未获取到任何文档块，请检查文档ID和权限")
	}

	// 转换为 Markdown
	mdContent := BlocksToMarkdown(apiResponse.Data.Items)

	return mdContent, nil
}

// DocumentResult 文档处理结果
type DocumentResult struct {
	Name     string // 文档名称
	Token    string // 文档 token
	Markdown string // Markdown 内容
	Error    error  // 处理错误（如果有）
}

// processSingleDoc 处理单个文档并保存为文件（用于测试）
func processSingleDoc(ctx context.Context, appID, appSecret, docToken, docName, userAccessToken string) error {
	mdContent, err := FetchDocumentToMarkdown(ctx, appID, appSecret, docToken, userAccessToken)
	if err != nil {
		return err
	}

	fileName := sanitizeFileName(docName)
	outputPath := fmt.Sprintf("%s_%s.md", fileName, docToken)

	if err := os.WriteFile(outputPath, []byte(mdContent), 0644); err != nil {
		return fmt.Errorf("写入 Markdown 文件失败（%s）: %w", outputPath, err)
	}

	fmt.Printf("文档 \"%s\" 转换成功，输出：%s\n", docName, outputPath)
	return nil
}

// 解析文本元素（TextElement 数组 → 字符串）
// 处理普通文本（text_run）和提及用户（mention_user）
func parseTextElements(elements []TextElement) string {
	var contentBuilder strings.Builder
	for _, elem := range elements {
		if elem.TextRun != nil {
			// 普通文本：直接拼接内容
			content := elem.TextRun.Content
			// 处理样式（粗体、斜体等）
			if elem.TextRun.TextElementStyle != nil {
				style := elem.TextRun.TextElementStyle
				if style.Bold {
					content = fmt.Sprintf("**%s**", content)
				}
				if style.Italic {
					content = fmt.Sprintf("*%s*", content)
				}
				if style.InlineCode {
					content = fmt.Sprintf("`%s`", content)
				}
				if style.Strikethrough {
					content = fmt.Sprintf("~~%s~~", content)
				}
			}
			contentBuilder.WriteString(content)
		} else if elem.MentionUser != nil {
			// 提及用户：转为 @用户名（这里 user_id 可替换为真实用户名，需调用飞书用户API）
			// 临时方案：用 user_id 后缀占位，实际可通过飞书 API 查询用户名
			userID := elem.MentionUser.UserID
			if len(userID) > 6 {
				userName := fmt.Sprintf("用户_%s", userID[len(userID)-6:])
				contentBuilder.WriteString(fmt.Sprintf("@%s", userName))
			} else {
				contentBuilder.WriteString(fmt.Sprintf("@%s", userID))
			}
		}
	}
	return contentBuilder.String()
}

// 从 URL 中提取 token（支持 folder 和 docx）
func extractTokenFromURL(url string) string {
	if url == "" {
		return ""
	}
	// 处理 folder URL: https://awq7m8b63wy.feishu.cn/drive/folder/SGQ0fYKoRlOWi7dwri5clDGrnme
	if strings.Contains(url, "/drive/folder/") {
		parts := strings.Split(url, "/drive/folder/")
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}
	// 处理 docx URL: https://awq7m8b63wy.feishu.cn/docx/GCnddftYuoVvflx5YFIc8m9mnpe
	if strings.Contains(url, "/docx/") {
		parts := strings.Split(url, "/docx/")
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}
	// 兜底：取 URL 最后一段
	parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return ""
}

// FetchFolderDocumentsToMarkdown 递归获取文件夹下所有 docx 文档并转换为 Markdown，返回所有文档的结果
func FetchFolderDocumentsToMarkdown(ctx context.Context, appID, appSecret, folderToken, userAccessToken string) ([]DocumentResult, error) {
	client := lark.NewClient(appID, appSecret)
	var results []DocumentResult

	err := processFolderRecursive(ctx, client, appID, appSecret, folderToken, "根文件夹", userAccessToken, 0, &results)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// processFolderRecursive 递归处理文件夹（支持嵌套文件夹），收集所有文档的 markdown
func processFolderRecursive(ctx context.Context, client *lark.Client, appID, appSecret, folderToken string, folderName string, userAccessToken string, depth int, results *[]DocumentResult) error {
	indent := strings.Repeat("  ", depth)
	fmt.Printf("%s📁 处理文件夹: %s (token: %s)\n", indent, folderName, folderToken)

	// 列出文件夹下的所有文件
	listReq := larkdrive.NewListFileReqBuilder().
		FolderToken(folderToken).
		OrderBy("EditedTime").
		Direction("DESC").
		PageSize(200).
		Build()

	listResp, err := client.Drive.V1.File.List(ctx, listReq, larkcore.WithUserAccessToken(userAccessToken))
	if err != nil {
		return fmt.Errorf("获取文件夹 %s 的文件列表失败: %v", folderName, err)
	}
	if !listResp.Success() {
		return fmt.Errorf("获取文件夹 %s 的文件列表接口失败: logId=%s, err=%s", folderName, listResp.RequestId(), larkcore.Prettify(listResp.CodeError))
	}

	listJSON, err := json.Marshal(listResp)
	if err != nil {
		return fmt.Errorf("序列化文件夹 %s 的文件列表响应失败: %v", folderName, err)
	}

	var driveResp DriveListFilesResponse
	if err := json.Unmarshal(listJSON, &driveResp); err != nil {
		return fmt.Errorf("解析文件夹 %s 的文件列表响应失败: %v", folderName, err)
	}
	if driveResp.Code != 0 {
		return fmt.Errorf("文件夹 %s 的文件列表接口返回错误: code=%d, msg=%s", folderName, driveResp.Code, driveResp.Msg)
	}

	if len(driveResp.Data.Files) == 0 {
		fmt.Printf("%s  (空文件夹)\n", indent)
		return nil
	}

	fmt.Printf("%s  发现 %d 个项目\n", indent, len(driveResp.Data.Files))

	docCount := 0
	folderCount := 0

	// 遍历处理每个文件/文件夹
	for _, f := range driveResp.Data.Files {
		if f.Type == "folder" {
			// 递归处理子文件夹
			folderCount++
			subFolderToken := f.Token
			if subFolderToken == "" {
				subFolderToken = extractTokenFromURL(f.URL)
			}
			if subFolderToken == "" {
				fmt.Printf("%s  ⚠️  跳过文件夹 \"%s\"，未找到 token\n", indent, f.Name)
				continue
			}
			if err := processFolderRecursive(ctx, client, appID, appSecret, subFolderToken, f.Name, userAccessToken, depth+1, results); err != nil {
				fmt.Printf("%s  ❌ 文件夹 \"%s\" 处理失败: %v\n", indent, f.Name, err)
			}
		} else if f.Type == "docx" {
			// 处理 docx 文档
			docCount++
			docToken := f.Token
			if docToken == "" {
				docToken = extractTokenFromURL(f.URL)
			}
			if docToken == "" {
				fmt.Printf("%s  ⚠️  跳过文档 \"%s\"，未找到 token\n", indent, f.Name)
				continue
			}
			fmt.Printf("%s  📄 (%d) 处理文档: %s\n", indent, docCount, f.Name)

			// 调用 FetchDocumentToMarkdown 获取 markdown
			mdContent, err := FetchDocumentToMarkdown(ctx, appID, appSecret, docToken, userAccessToken)
			result := DocumentResult{
				Name:     f.Name,
				Token:    docToken,
				Markdown: mdContent,
				Error:    err,
			}
			*results = append(*results, result)

			if err != nil {
				fmt.Printf("%s    ❌ 文档 \"%s\" 处理失败: %v\n", indent, f.Name, err)
			} else {
				fmt.Printf("%s    ✅ 文档 \"%s\" 转换成功 (%d 字符)\n", indent, f.Name, len(mdContent))
			}
		} else {
			fmt.Printf("%s  ⏭️  跳过非 docx 文件: %s (type: %s)\n", indent, f.Name, f.Type)
		}
	}

	fmt.Printf("%s✅ 文件夹 %s 处理完成 (文档: %d, 子文件夹: %d)\n", indent, folderName, docCount, folderCount)
	return nil
}

func Test() ([]DocumentResult, error) {
	ctx := context.Background()

	// TODO: 这些配置建议换成环境变量或配置文件
	const appID = "cli_a9afad5abfb85bc0"
	const appSecret = "RDIAVuYOukhGNdZcn1zO9dLJS8up7rYL"
	const userAccessToken = "u-daQsAlLPRambYOI54uf0Zrhk16vlggUVpE2aEB6w0LKf"
	const folderToken = "PyOifPcHPldVPodJaxVce2LBnSb"

	fmt.Println("🚀 开始递归处理飞书文件夹...")
	fmt.Println("=" + strings.Repeat("=", 60))

	// 批量获取所有文档的 Markdown
	results, err := FetchFolderDocumentsToMarkdown(ctx, appID, appSecret, folderToken, userAccessToken)
	if err != nil {
		fmt.Printf("❌ 处理失败: %v\n", err)
		return nil, err
	}

	fmt.Printf("成功获取 %d 个文档块，开始转换为 Markdown...\n", len(results))
	return results, nil
}

// testSaveDocuments 测试函数：保存所有文档为文件并打印统计信息（用于单元测试）
func testSaveDocuments(results []DocumentResult) {
	successCount := 0
	for _, result := range results {
		if result.Error != nil {
			fmt.Printf("⚠️  文档 \"%s\" 处理失败: %v\n", result.Name, result.Error)
			continue
		}

		// 保存为文件（仅用于测试）
		fileName := sanitizeFileName(result.Name)
		outputPath := fmt.Sprintf("%s_%s.md", fileName, result.Token)
		if err := os.WriteFile(outputPath, []byte(result.Markdown), 0644); err != nil {
			fmt.Printf("⚠️  保存文件失败 \"%s\": %v\n", outputPath, err)
			continue
		}
		successCount++
		fmt.Printf("💾 已保存: %s (%d 字符)\n", outputPath, len(result.Markdown))
	}

	fmt.Printf("\n📊 统计: 成功 %d/%d 个文档\n", successCount, len(results))

	// 示例：如何使用返回的 markdown 数据
	fmt.Println("\n📝 示例：如何使用返回的 Markdown 数据：")
	for i, result := range results {
		if i >= 3 { // 只显示前3个示例
			break
		}
		if result.Error == nil {
			preview := result.Markdown
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			fmt.Printf("  文档 %d: %s\n", i+1, result.Name)
			fmt.Printf("    Token: %s\n", result.Token)
			fmt.Printf("    Markdown 预览: %s\n", preview)
		}
	}
}

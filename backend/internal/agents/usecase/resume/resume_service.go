package resume

import (
	"context"
	"encoding/json"
	"fmt"
	"interview-agents/internal/agents/llm"
	"interview-agents/internal/agents/pkg"
	"interview-agents/internal/agents/tools"
	"interview-agents/internal/model"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

// ResumeParseResult 简历解析结果
type ResumeParseResult struct {
	BasicInfo struct {
		Name      string `json:"name"`
		WorkYears string `json:"work_years"`
		Contact   string `json:"contact"`
	} `json:"basic_info"`
	Education []struct {
		School         string `json:"school"`
		Major          string `json:"major"`
		Degree         string `json:"degree"`
		GraduationYear string `json:"graduation_year"`
	} `json:"education"`
	WorkExperience []struct {
		Company          string `json:"company"`
		Position         string `json:"position"`
		Duration         string `json:"duration"`
		Responsibilities string `json:"responsibilities"`
	} `json:"work_experience"`
	TechStack           []string      `json:"tech_stack"`
	Projects            []interface{} `json:"projects"`
	Skills              []string      `json:"skills"`
	Certifications      []string      `json:"certifications"`
	Strengths           string        `json:"strengths"`
	PotentialWeaknesses string        `json:"potential_weaknesses"`
	// RecommendedDifficulty       string        `json:"recommended_difficulty"`
	// InterviewFocusAreas         []string      `json:"interview_focus_areas"`
	// SuggestedQuestionDirections []string      `json:"suggested_questions_directions"`
}

// ParseResume 调用简历解析流水线解析简历
// 参数说明：
//   - ctx: 上下文
//   - userId: 用户ID
//   - resumeFilePath: 上传的简历文件路径（已保存到 backend/uploads/resumes）
//
// 返回解析结果
func ParseResume(ctx context.Context, userId uint, resumeFilePath string) (*ResumeParseResult, error) {
	// 添加 300 秒 (5分钟) 超时
	log.Printf("验证生效了吗---")
	timeoutCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	log.Printf("[ParseResume] 开始执行简历解析流水线，用户ID: %d, 文件: %s", userId, resumeFilePath)

	// --- 步骤 1: PDF 转文本 (Go 直接调用) ---
	startTime := time.Now()
	log.Printf("[步骤1/3] 开始解析 PDF 文件...")

	pdfReq := tools.PDFToTextRequest{
		FilePath: resumeFilePath,
		ToPages:  false,
	}
	pdfResult, err := tools.ConvertPDFToText(timeoutCtx, &pdfReq)
	if err != nil {
		log.Printf("[步骤1失败] PDF解析失败: %v", err)
		return nil, fmt.Errorf("pdf parsing failed: %w", err)
	}
	if !pdfResult.Success {
		log.Printf("[步骤1失败] PDF工具返回失败: %s", pdfResult.ErrorMsg)
		return nil, fmt.Errorf("pdf tool failed: %s", pdfResult.ErrorMsg)
	}

	pdfDuration := time.Since(startTime)
	contentLen := len(pdfResult.Content)
	log.Printf("[步骤1完成] PDF 解析成功，文本长度: %d 字符，耗时: %v", contentLen, pdfDuration)

	if contentLen == 0 {
		return nil, fmt.Errorf("parsed pdf content is empty")
	}

	// --- 步骤 2: 构建提示词 ---
	log.Printf("[步骤2/3] 正在构建大模型分析指令...")

	systemPrompt := `你是一个专业的简历分析专家。你的任务是根据提供的简历内容，提取关键信息用于面试准备。

任务要求：
1. **[思维链 CoT]** 在生成最终 JSON 之前，请先在 <thinking>...</thinking> 标签内进行一步步的深度思考和分析。
   - 分析简历的整体结构和质量。
   - 逐个识别关键字段（如姓名、学校、公司），并处理可能存在的格式混乱或OCR错误。
   - 对模棱两可的信息进行逻辑推理（例如：区分"项目经历"和"工作经历"）。
   - 检查提取的数据是否完整和真实。
2. 提取简历中的所有关键信息（基本信息、教育背景、工作经历、技术栈、项目经验、技能、证书等）。
3. 分析候选人的背景特点（技术方向、行业经验、核心竞争力）。
4. 必须返回标准的 JSON 格式，JSON 数据必须位于思考过程之后。
5. 必须填充真实数据，不要留空。

JSON 格式要求（请严格遵守此结构）：
{
  "basic_info": { "name": "...", "work_years": "...", "contact": "..." },
  "education": [ { "school": "...", "major": "...", "degree": "...", "graduation_year": "..." } ],
  "work_experience": [ { "company": "...", "position": "...", "duration": "...", "responsibilities": "..." } ],
  "tech_stack": [ "..." ],
  "projects": [ { "name": "...", "description": "...", "tech_stack": ["..."], "contribution": "..." } ],
  "skills": [ "..." ],
  "certifications": [ "..." ],
  "strengths": "...",
  "potential_weaknesses": "..."
}`

	userContent := fmt.Sprintf("这是候选人的简历内容：\n\n%s", pdfResult.Content)

	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.User,
			Content: userContent,
		},
	}

	// --- 步骤 3: 大模型分析 ---
	llmStartTime := time.Now()
	log.Printf("[步骤3/3] 正在初始化大模型并请求分析 (这可能需要几十秒)...")

	// 获取 ChatModel
	chatModel, err := llm.CreatOpenAiChatModel(timeoutCtx, userId)
	if err != nil {
		log.Printf("[步骤3失败] 创建大模型实例失败: %v", err)
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	// 调用 Generate (增加重试机制)
	var respMsg *schema.Message
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		// 为每次请求设置独立的超时时间 (例如 90秒)，总超时由 timeoutCtx (300秒) 控制
		// 这样如果一次请求卡住，可以有机会重试
		callCtx, callCancel := context.WithTimeout(timeoutCtx, 90*time.Second)

		log.Printf("[步骤3] 正在尝试第 %d/%d 次调用大模型...", i+1, maxRetries)
		respMsg, err = chatModel.Generate(callCtx, messages)
		callCancel() // 及时释放资源

		if err == nil {
			break // 成功，跳出循环
		}

		// 检查是否是父上下文超时（总耗时超过300秒）
		if timeoutCtx.Err() != nil {
			log.Printf("[步骤3失败] 总超时 (300s) 已在第 %d 次尝试时触发", i+1)
			err = timeoutCtx.Err() // 确保返回正确的错误
			break
		}

		log.Printf("[步骤3] 第 %d 次尝试失败: %v", i+1, err)

		if i < maxRetries-1 {
			waitTime := time.Duration(1<<i) * time.Second // 指数退避: 1s, 2s
			log.Printf("[步骤3] 等待 %v 后重试...", waitTime)
			select {
			case <-time.After(waitTime):
				// 继续重试
			case <-timeoutCtx.Done():
				// 等待期间总超时
				err = timeoutCtx.Err()
				break
			}
		}
	}

	if err != nil {
		log.Printf("[步骤3失败] 大模型生成最终失败: %v", err)
		return nil, fmt.Errorf("llm generation failed after retries: %w", err)
	}

	llmDuration := time.Since(llmStartTime)
	totalDuration := time.Since(startTime)
	log.Printf("[步骤3完成] 大模型分析完成，耗时: %v (总耗时: %v)", llmDuration, totalDuration)

	lastMessage := respMsg.Content
	// 只打印前200个字符避免日志过长
	logContent := lastMessage
	if len(logContent) > 200 {
		logContent = logContent[:200] + "..."
	}
	log.Printf("[ParseResume] 智能体响应内容(前200字符): %s", logContent)

	parseResult := parseResumeResponse(lastMessage)
	if parseResult == nil {
		log.Printf("[ParseResume] 无法解析简历响应")
		return nil, fmt.Errorf("failed to parse resume response")
	}

	// 验证解析结果是否有效（不能全是空数据）
	if !isValidResumeResult(parseResult) {
		log.Printf("[ParseResume] 解析结果无效（全是空数据），请检查简历文件是否正确")
		return nil, fmt.Errorf("resume parsing result is empty or invalid")
	}

	log.Printf("[ParseResume] 简历解析成功")
	return parseResult, nil
}

// parseResumeResponse 从智能体响应解析简历数据
func parseResumeResponse(agentResponse string) *ResumeParseResult {
	result := &ResumeParseResult{}

	// 处理 CoT (Chain of Thought) 输出
	// 如果响应中包含 <thinking>...</thinking>，需要先剥离，避免干扰 JSON 解析
	// 虽然 pkg.pkg.ExtractJSONFromResponse 可以找到 JSON，但为了日志清晰和避免误判，最好分离
	jsonContent := agentResponse
	if start := strings.Index(agentResponse, "<thinking>"); start != -1 {
		if end := strings.Index(agentResponse, "</thinking>"); end != -1 {
			log.Printf("[parseResumeResponse] 检测到思维链内容:\n%s", agentResponse[start:end+11])
			// 移除 thinking 部分，保留剩下的部分（通常 JSON 在后面）
			jsonContent = agentResponse[end+11:]
		}
	}

	// 尝试直接解析 JSON
	if err := json.Unmarshal([]byte(jsonContent), result); err != nil {
		log.Printf("[parseResumeResponse] 直接解析 JSON 失败: %v，尝试提取 JSON", err)
		// 尝试从文本中提取 JSON
		jsonStr := pkg.ExtractJSONFromResponse(jsonContent)
		if jsonStr == "" {
			// 如果剥离后找不到，尝试从原始响应找（兜底）
			jsonStr = pkg.ExtractJSONFromResponse(agentResponse)
		}

		if jsonStr == "" {
			log.Printf("[parseResumeResponse] 无法提取 JSON，原始响应: %s", agentResponse)
			return nil
		}

		log.Printf("[parseResumeResponse] 提取的 JSON: %s", jsonStr)
		// 尝试解析提取的 JSON
		if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
			log.Printf("[parseResumeResponse] 解析提取的 JSON 失败: %v", err)
			return nil
		}
	}

	return result
}

// isValidResumeResult 检查解析结果是否有效（不能全是空数据）
func isValidResumeResult(result *ResumeParseResult) bool {
	if result == nil {
		return false
	}

	// 检查基本信息是否有内容
	if result.BasicInfo.Name != "" || result.BasicInfo.WorkYears != "" || result.BasicInfo.Contact != "" {
		return true
	}

	// 检查教育背景
	if len(result.Education) > 0 {
		return true
	}

	// 检查工作经历
	if len(result.WorkExperience) > 0 {
		return true
	}

	// 检查技术栈
	if len(result.TechStack) > 0 {
		return true
	}

	// 检查项目经验
	if len(result.Projects) > 0 {
		return true
	}

	// 检查技能
	if len(result.Skills) > 0 {
		return true
	}

	// 检查证书
	if len(result.Certifications) > 0 {
		return true
	}

	// 检查其他字段
	if result.Strengths != "" || result.PotentialWeaknesses != "" {
		return true
	}

	// // 检查面试关注领域
	// if len(result.InterviewFocusAreas) > 0 {
	// 	return true
	// }

	// // 检查建议的提问方向
	// if len(result.SuggestedQuestionDirections) > 0 {
	// 	return true
	// }

	// 如果所有字段都是空的，返回 false
	return false
}

// saveResumeToDatabase 将简历解析结果保存到数据库
// 参数说明：
//   - ctx: 上下文
//   - userId: 用户ID
//   - resumeFilePath: 原始 PDF 文件路径（已保存到 backend/uploads/resumes）
//   - fileSize: 文件大小
//   - parseResult: 解析后的简历数据
func saveResumeToDatabase(ctx context.Context, userId uint, resumeFilePath string, fileSize int64, parseResult *ResumeParseResult) (uint64, error) {
	// 将解析结果转换为 JSON 字符串存储
	contentJSON, err := json.Marshal(parseResult)
	if err != nil {
		log.Printf("[saveResumeToDatabase] 序列化简历数据失败: %v", err)
		return 0, fmt.Errorf("failed to marshal resume data: %w", err)
	}

	// 从文件路径中提取文件名
	fileName := filepath.Base(resumeFilePath)
	log.Printf("[saveResumeToDatabase] 原始文件路径: %s, 提取的文件名: %s", resumeFilePath, fileName)

	// 创建简历记录
	// Content 字段存储解析后的 JSON 数据，用于快速查询
	// FileName 字段存储原始 PDF 文件名
	// FileType 字段标记为 "pdf"，表示这是 PDF 简历
	resumeRecord := &model.Resume{
		UserID:    userId,
		Content:   string(contentJSON),
		FileName:  fileName,
		FileSize:  fileSize,
		FileType:  "pdf",
		IsDefault: 1,
		Deleted:   0,
	}

	// 调用 DAO 方法保存到数据库
	resumeID, err := model.ResumeDao.CreateResume(resumeRecord)
	if err != nil {
		log.Printf("[saveResumeToDatabase] 创建简历记录失败: %v", err)
		return 0, fmt.Errorf("failed to create resume record: %w", err)
	}

	log.Printf("[saveResumeToDatabase] 简历记录已保存，ID: %d, 用户ID: %d, 文件路径: %s", resumeID, userId, resumeFilePath)
	return resumeID, nil
}

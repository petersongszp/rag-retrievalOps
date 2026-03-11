namespace go interview

// ==================== 基础数据结构 ====================

// 流式面试事件（SSE 推送）
struct StreamEvent {
    1: required string type              // 事件类型：session_id, start, question, ready_for_answer, heartbeat, topic_complete, error, complete
    2: optional string session_id        // 会话ID
    3: optional string message           // 消息内容
    4: optional map<string, string> data // 事件数据（JSON 格式字段）
    5: optional i64 timestamp            // 事件时间戳（毫秒）
}

// 问题数据
struct QuestionData {
    1: required i32 order               // 问题顺序
    2: required string question_text    // 问题内容
    3: required string eval_dimension   // 评估维度
    4: optional string difficulty       // 难度级别
    5: optional map<string, string> metadata  // 扩展元数据
}

// 对话记录
struct DialogueRecord {
    1: required i32 order               // 对话顺序
    2: required string speaker_type     // 发言人类型：interviewer, candidate
    3: required string content          // 对话内容
    4: required i32 display_order       // 显示顺序（用于排序）
    5: optional i64 timestamp           // 对话时间戳
    6: optional map<string, string> metadata  // 扩展元数据
}

// 面试会话
struct InterviewSession {
    1: required string session_id       // 会话ID
    2: required i32 user_id             // 用户ID
    3: required i64 record_id           // 面试记录ID
    4: required string type             // 面试类型：综合面试、专项面试
    5: required string domain           // 面试领域
    6: required string difficulty       // 校招/社招
    7: optional i64 resume_id           // 简历ID
    8: optional bool has_resume         // 是否有简历
    9: required i64 start_time          // 开始时间戳
    10: optional i64 end_time           // 结束时间戳
    11: required string status          // 会话状态：active, paused, completed, failed
    12: optional list<QuestionData> questions  // 问题列表
    13: optional list<DialogueRecord> dialogues  // 对话列表
    14: optional map<string, string> metadata   // 扩展元数据
}

// ==================== 请求和响应结构 ====================

// 启动面试流请求
struct InterviewStartInterviewRequest {
    1: required string type (api.body="type")              // 面试类型
    2: required string domain (api.body="domain")          // 面试领域
    3: required string difficulty (api.body="difficulty")  // 校招/社招
    4: optional string company_name (api.body="company_name")    // 公司名称
    5: optional string position_name (api.body="position_name")  // 岗位名称
    6: optional i64 resume_id (api.body="resume_id")       // 简历ID
    7: optional map<string, string> metadata (api.body="metadata")  // 扩展参数
}

// 启动面试流响应
struct InterviewStartInterviewResponse {
    1: required string session_id       // 会话ID
    2: required i64 record_id           // 面试记录ID
    3: required string message          // 响应消息
    4: required i64 start_time          // 面试开始时间戳（毫秒）
    5: optional map<string, string> metadata  // 扩展数据
}

// 提交面试答案请求
struct InterviewSubmitInterviewAnswerRequest {
    1: required string session_id (api.body="session_id")  // 会话ID
    2: required string answer (api.body="answer")          // 用户答案
    3: optional string action (api.body="action")          // 操作类型：answer, continue, quit
    4: optional map<string, string> metadata (api.body="metadata")  // 扩展参数
}

// 提交面试答案响应
struct InterviewSubmitInterviewAnswerResponse {
    1: required string status           // 状态：received, error
    2: optional string message          // 消息说明
    3: optional string session_id       // 会话ID
    4: optional i32 question_index      // 当前问题索引
    5: optional bool is_last_question   // 是否为最后一个问题
    6: optional map<string, string> metadata  // 扩展数据
}

// 获取会话信息请求
struct InterviewGetSessionRequest {
    1: required string session_id (api.query="session_id")  // 会话ID
}

// 获取会话信息响应
struct InterviewGetSessionResponse {
    1: required InterviewSession session  // 会话信息
    2: optional i32 current_question_index  // 当前问题索引
    3: optional string current_question_text  // 当前问题文本
    4: optional i32 answered_count       // 已回答问题数
    5: optional i32 total_count          // 总问题数
    6: optional i64 elapsed_time         // 已用时间（秒）
    7: optional map<string, string> metadata  // 扩展数据
}

// 结束面试请求
struct InterviewEndInterviewRequest {
    1: required string session_id (api.body="session_id")  // 会话ID
    2: optional string reason (api.body="reason")          // 结束原因
    3: optional map<string, string> metadata (api.body="metadata")  // 扩展参数
}

// 结束面试响应
struct InterviewEndInterviewResponse {
    1: required string status           // 状态
    2: optional string message          // 消息说明
    3: optional i64 duration            // 面试时长（秒）
    4: optional i64 end_time            // 面试结束时间戳（毫秒）
    5: optional i32 total_questions     // 总问题数
    6: optional i32 answered_questions  // 已回答问题数
    7: optional map<string, string> metadata  // 扩展数据
}

// ==================== 面试记录相关结构 ====================

// 面试记录 DTO
struct InterviewInterviewRecordDTO {
    1: required i64 id                  // 记录ID
    2: required i32 user_id             // 用户ID
    3: required string type             // 面试类型
    4: required string difficulty       // 难度级别
    5: required string domain           // 面试领域
    6: optional string company_name     // 公司名称
    7: optional string position_name    // 岗位名称
    8: required string status           // 面试状态
    9: optional i64 duration            // 面试耗时（秒）
    10: optional i64 resume_id          // 简历ID
    11: optional double score           // 面试评分
    12: optional string report          // 面试报告
    13: optional i64 created_at         // 创建时间
    14: optional i64 updated_at         // 更新时间
    15: optional i64 completed_at       // 完成时间
    16: optional i32 total_questions    // 总问题数
    17: optional i32 answered_questions // 已回答问题数
    18: optional map<string, string> metadata  // 扩展元数据
}

// ==================== 评估相关结构 ====================

// 评估维度
struct InterviewEvaluationDimension {
    1: required string dimension_name  // 维度名称（如：技术能力、沟通能力等）
    2: required string evaluation      // 该维度的评估内容
    3: required i32 score              // 该维度的评分（0-100）
}

// 获取面试评估请求
struct GetInterviewEvaluationRequest {
    1: required i64 report_id (api.query="report_id")  // 面试报告ID
}

// 获取面试评估响应
struct GetInterviewEvaluationResponse {
    1: required string comment                                    // 整体评价
    2: required list<InterviewEvaluationDimension> dimensions      // 各维度评估列表
}

// ==================== 答题记录相关结构 ====================

// 答题记录中的单条对话
struct InterviewAnswerRecordMessage {
    1: required i32 order       // 对话顺序
    2: required string question // 提问内容
    3: required string answer   // 回答内容
}

// 答题记录中的评论信息
struct InterviewAnswerRecordComment {
    1: required i32 score           // 评分
    2: required string key_points   // 关键点
    3: required string difficulty   // 难度等级
    4: required string strengths    // 优势
    5: required string weaknesses   // 不足
    6: required string suggestion   // 建议
    7: required string know_points  // 知识点
    8: required string thinking     // 思考过程
    9: required string reference    // 参考答案
}

// 单个答题记录
struct InterviewAnswerRecord {
    1: required i32 order                                    // 问题顺序
    2: required string content                               // 问题内容
    3: required InterviewAnswerRecordComment comment           // 评论信息
    4: required list<InterviewAnswerRecordMessage> message     // 对话列表
}

// 获取答题记录请求
struct GetInterviewAnswerRecordRequest {
    1: required i64 report_id (api.query="report_id")  // 面试报告ID
}

// 获取答题记录响应
struct GetInterviewAnswerRecordResponse {
    1: required list<InterviewAnswerRecord> records  // 答题记录列表
}

// ==================== 面试记录列表相关结构 ====================

// 获取面试记录列表请求
struct InterviewListRecordsRequest {
    1: optional i32 page      (api.query="page")       // 页码，默认 1
    2: optional i32 page_size (api.query="page_size")  // 每页数量，默认 10
}

// 获取面试记录列表响应
struct InterviewListRecordsResponse {
    1: required list<InterviewInterviewRecordDTO> records   // 面试记录列表
    2: required i64 total                                 // 总条数
    3: required i32 page                                  // 当前页码
    4: required i32 page_size                             // 每页数量
}




// ==================== 服务定义 ====================

service InterviewService {
    // ==================== 流式面试核心接口 ====================

    // 启动面试流程（SSE 流式）
    InterviewStartInterviewResponse StartInterviewStream(1: InterviewStartInterviewRequest request) (
        api.post="/api/interview/stream/start",
        api.category="interview",
        api.gen_path="interview"
    )

    // 提交面试答案
    InterviewSubmitInterviewAnswerResponse SubmitInterviewAnswer(1: InterviewSubmitInterviewAnswerRequest request) (
        api.post="/api/interview/answer/submit",
        api.category="interview",
        api.gen_path="interview"
    )

    // 获取会话信息
    InterviewGetSessionResponse GetSession(1: InterviewGetSessionRequest request) (
        api.get="/api/interview/session/info",
        api.category="interview",
        api.gen_path="interview"
    )

    // 结束面试
    InterviewEndInterviewResponse EndInterview(1: InterviewEndInterviewRequest request) (
        api.post="/api/interview/interview/end",
        api.category="interview",
        api.gen_path="interview"
    )

    // ==================== 评估和记录相关接口 ====================

    // 获取面试评估
    GetInterviewEvaluationResponse GetInterviewEvaluation(1: GetInterviewEvaluationRequest request) (
        api.get="/api/interview/evaluation",
        api.category="interview",
        api.gen_path="interview"
    )

    // 获取答题记录
    GetInterviewAnswerRecordResponse GetInterviewAnswerRecord(1: GetInterviewAnswerRecordRequest request) (
        api.get="/api/interview/answer-record",
        api.category="interview",
        api.gen_path="interview"
    )

    // 获取面试记录列表
    InterviewListRecordsResponse ListInterviewRecords(1: InterviewListRecordsRequest request) (
        api.get="/api/interview/records",
        api.category="interview",
        api.gen_path="interview"
    )
}
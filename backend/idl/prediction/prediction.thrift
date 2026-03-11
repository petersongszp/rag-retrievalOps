namespace go prediction

// 押题请求
struct PredictRequest {
    1: required i64 resume_id (api.form="resume_id")
    2: required string prediction_type (api.form="prediction_type") // 校招/社招
    3: required string language (api.form="language") // java/go
    4: required string job_title (api.form="job_title") // 前端/后端
    5: required string difficulty (api.form="difficulty") // 入门/进阶
    6: optional string company_name (api.form="company_name") // 公司名称
}

// 押题问题详情
struct PredictionQuestion {
    1: required i64 id
    2: required string question
    3: required string content
    4: required string focus
    5: required string thinking_path
    6: required string reference_answer
    7: required string follow_up
    8: required i32 sort
}

// 押题响应
struct PredictResponse {
    1: required i64 record_id
    2: required list<PredictionQuestion> questions
}

// 获取押题记录列表请求
struct ListPredictionRequest {
    1: optional i32 page (api.query="page", default="1")
    2: optional i32 size (api.query="size", default="10")
}

// 押题记录摘要
struct PredictionRecordItem {
    1: required i64 id
    2: required string created_at
    3: required string job_title
    4: required string difficulty
    5: required string company
    6: required string prediction_type
    7: required string language
}

// 列表响应
struct ListPredictionResponse {
    1: required list<PredictionRecordItem> list
    2: required i64 total
    3: required i32 page
    4: required i32 size
}

// 获取详情请求
struct GetPredictionDetailRequest {
    1: required i64 id (api.path="id")
}

// 详情响应
struct GetPredictionDetailResponse {
    1: required i64 id
    2: required list<PredictionQuestion> questions
}

service PredictionService {
    // 开始押题
    PredictResponse Predict(1: PredictRequest request) (
        api.post="/api/prediction/start",
        api.category="prediction",
        api.gen_path="prediction"
    )

    // 获取列表
    ListPredictionResponse ListPredictions(1: ListPredictionRequest request) (
        api.get="/api/prediction/list",
        api.category="prediction",
        api.gen_path="prediction"
    )

    // 获取详情
    GetPredictionDetailResponse GetPredictionDetail(1: GetPredictionDetailRequest request) (
        api.get="/api/prediction/:id",
        api.category="prediction",
        api.gen_path="prediction"
    )
}

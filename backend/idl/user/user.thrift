namespace go user

// ==================== 1. 创建用户模型 ====================

// 创建用户模型请求
struct CreateUserModelRequest {
    1: required string name (api.form="name")
    2: required string model_key (api.form="model_key")
    3: required string protocol (api.form="protocol")
    4: required string base_url (api.form="base_url")
    5: required string api_key (api.form="api_key")
    6: required string provider_name (api.form="provider_name")
    7: optional i64 meta_id (api.form="meta_id")
    8: optional string default_params (api.form="default_params")  // JSON string
    9: optional string config_json (api.form="config_json")        // JSON string
    10: optional i32 scope (api.form="scope")                      // 默认 7
    11: optional i32 status (api.form="status")                    // 默认 1
    12: optional i32 is_default (api.form="is_default")            // 是否为默认（0=不是, 1=是）
}

// 创建用户模型响应
struct CreateUserModelResponse {
    1: required string state
}

// ==================== 2. 获取用户模型列表 ====================

// 获取用户模型列表请求
struct ListUserModelsRequest {
    1: optional i32 status (api.query="status")                    // 状态筛选
    2: optional i32 scope (api.query="scope")                      // 使用范围筛选
    3: optional string protocol (api.query="protocol")             // 协议类型筛选
    4: optional string provider_name (api.query="provider_name")   // 提供商名称筛选
    5: optional string keyword (api.query="keyword")               // 关键词搜索
    6: optional i32 page (api.query="page", api.vd="$>=1")         // 页码，默认 1
    7: optional i32 size (api.query="size", api.vd="$>=1&&$<=100")  // 每页数量，默认 20
}

// 用户模型列表项
struct UserModelItem {
    1: required i64 id
    2: required string name
    3: required string model_key
    4: required string protocol
    5: required string base_url
    6: required string provider_name
    7: optional i64 meta_id
    8: optional string default_params
    9: optional string config_json
    10: required i32 scope
    11: required i32 status
    12: required i64 created_at
    13: required i64 updated_at
    14: required bool has_secret
    15: optional string secret_hint
    16: required i32 is_default
}

// 获取用户模型列表响应
struct ListUserModelsResponse {
    1: required list<UserModelItem> list
    2: required i64 total
    3: required i32 page
    4: required i32 size
}

// ==================== 3. 获取用户模型详情 ====================

// ID 请求结构
struct IDRequest {
    1: required i64 id (api.path="id")
}

// 用户模型详细信息
struct UserModelDetail {
    1: required i64 id
    2: required string name
    3: required string model_key
    4: required string protocol
    5: required string base_url
    6: required string provider_name
    7: optional i64 meta_id
    8: optional string default_params     // JSON string
    9: optional string config_json        // JSON string
    10: required i32 scope
    11: required i32 status
    12: required i64 created_at
    13: required i64 updated_at
    14: required bool has_secret          // 是否已配置密钥
    15: optional string secret_hint       // 密钥脱敏提示
    16: required i32 is_default           // 是否为默认（0=不是, 1=是）
}

// 获取用户模型详情响应
struct GetUserModelResponse {
    1: required UserModelDetail data
}

// ==================== 4. 更新用户模型 ====================

// 更新用户模型请求
struct UpdateUserModelRequest {
    1: required i64 id (api.path="id")                            // 添加ID字段
    2: required string name (api.form="name")
    3: required string model_key (api.form="model_key")
    4: required string protocol (api.form="protocol")
    5: required string base_url (api.form="base_url")
    6: optional string api_key (api.form="api_key")               // 可选，不传则不更新密钥
    7: required string provider_name (api.form="provider_name")
    8: optional i64 meta_id (api.form="meta_id")
    9: optional string default_params (api.form="default_params")  // JSON string
    10: optional string config_json (api.form="config_json")        // JSON string
    11: optional i32 scope (api.form="scope")
    12: optional i32 status (api.form="status")
    13: optional i32 is_default (api.form="is_default")            // 是否为默认（0=不是, 1=是）
}


// 更新用户模型响应
struct UpdateUserModelResponse {
}

// ==================== 5. 删除用户模型 ====================

// 删除用户模型响应
struct DeleteUserModelResponse {
}

// ==================== 6. 用户认证与资料相关 ====================

// 注册请求
struct RegisterRequest {
    1: required string username (api.form="username")
    2: required string email (api.form="email")
    3: required string password (api.form="password")
}

// 登录请求
struct LoginRequest {
    1: required string email (api.form="email")
    2: required string password (api.form="password")
}

// 用户资料
struct UserProfile {
    1: required i64 id
    2: required string username
    3: required string email
    4: required string role
    5: optional string wechat_open_id
    6: optional string wechat_union_id
    7: optional string nickname
    8: optional string avatar
    9: optional i64 created_at
    10: optional i64 updated_at
}

// 登录/注册响应
struct LoginResponse {
    1: required string token
    2: required UserProfile user
}

// 空请求
struct EmptyRequest {}

// 获取资料响应
struct GetProfileResponse {
    1: required UserProfile data
}

// 更新资料请求
struct UpdateProfileRequest {
    1: optional string username (api.form="username")
    2: optional string email (api.form="email")
}

// 更新资料响应
struct UpdateProfileResponse {
    1: required UserProfile data
}

// 微信登录二维码响应
struct WechatLoginQRResponse {
    1: required string login_url
}

// 微信回调请求
struct WechatCallbackRequest {
    1: required string code (api.query="code")
    2: optional string state (api.query="state")
}

// 用户密码找回请求
struct ForgotPasswordRequest {
    1: required string email (api.form="email")
}

// 用户密码找回响应
struct ForgotPasswordResponse {}

// 重置密码请求
struct ResetPasswordRequest {
    1: required string token (api.form="token")
    2: required string password (api.form="password")
    3: required string confirm_password (api.form="confirm_password")
}

// 重置密码响应
struct ResetPasswordResponse {}

// ==================== 7. 检查用户是否配置了模型 ====================
struct CheckUserModelConfiguredResponse {
    1: required bool configured  // 是否已配置并启用默认模型（is_default = 1）
}

// 服务定义
service UserService {
    // 1. 创建用户模型
       CreateUserModelResponse CreateUserModel(1: CreateUserModelRequest request) (
           api.post="/api/user/create/model",
           api.category="user",
           api.gen_path="user"
       )

       // 2. 获取用户模型列表
       ListUserModelsResponse ListUserModels(1: ListUserModelsRequest request) (
           api.get="/api/user/model/list",
           api.category="user",
           api.gen_path="user"
       )

       // 3. 获取用户模型详情
       GetUserModelResponse GetUserModel(1: IDRequest request) (
           api.get="/api/user/model/details/:id",
           api.category="user",
           api.gen_path="user"
       )

       // 4. 更新用户模型
       UpdateUserModelResponse UpdateUserModel(1: UpdateUserModelRequest request) (
           api.put="/api/user/model/update/:id",
           api.category="user",
           api.gen_path="user"
       )

       // 5. 删除用户模型
       DeleteUserModelResponse DeleteUserModel(1: IDRequest request) (
           api.delete="/api/user/model/delete/:id",
           api.category="user",
           api.gen_path="user"
       )

       // 6. 用户注册
       LoginResponse Register(1: RegisterRequest request) (
           api.post="/api/user/register",
           api.category="user",
           api.gen_path="user"
       )

       // 7. 用户登录
       LoginResponse Login(1: LoginRequest request) (
           api.post="/api/user/login",
           api.category="user",
           api.gen_path="user"
       )

       // 8. 获取用户资料
       GetProfileResponse GetProfile(1: EmptyRequest request) (
           api.get="/api/user/profile",
           api.category="user",
           api.gen_path="user"
       )

       // 9. 更新用户资料
       UpdateProfileResponse UpdateProfile(1: UpdateProfileRequest request) (
           api.put="/api/user/profile",
           api.category="user",
           api.gen_path="user"
       )

       // 10. 获取微信登录二维码
       WechatLoginQRResponse WechatLogin(1: EmptyRequest request) (
           api.get="/api/user/wechat/login",
           api.category="user",
           api.gen_path="user"
       )

       // 11. 微信登录回调
       LoginResponse WechatCallback(1: WechatCallbackRequest request) (
           api.get="/api/user/wechat/callback",
           api.category="user",
           api.gen_path="user"
       )
       // 12. 检查用户是否配置了模型
       CheckUserModelConfiguredResponse CheckUserModelConfigured(1: EmptyRequest request) (
           api.get="/api/user/model/check",
           api.category="user",
           api.gen_path="user"
       )

       // 13. 忘记密码
       ForgotPasswordResponse ForgotPassword(1: ForgotPasswordRequest request) (
           api.post="/api/user/password/forgot",
           api.category="user",
           api.gen_path="user"
       )

       // 14. 重置密码
       ResetPasswordResponse ResetPassword(1: ResetPasswordRequest request) (
           api.post="/api/user/password/reset",
           api.category="user",
           api.gen_path="user"
       )
}

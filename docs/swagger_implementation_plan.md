# 接口文档自动生成方案

## 实现思路

基于对项目代码的分析，我推荐使用以下方案来实现接口文档的自动生成：

### 1. 技术选型

- **主要工具**：Swagger/OpenAPI + hertz-contrib/swagger
- **文档格式**：OpenAPI 3.0（兼容API Fox导入）
- **实现方式**：代码注释驱动 + 中间件集成

### 2. 核心优势

- **自动化**：新添加的接口只需按照规范添加注释，自动包含在文档中
- **实时更新**：文档与代码保持同步，避免文档过时
- **易于维护**：注释与代码放在一起，便于维护和更新
- **支持API Fox导入**：生成的OpenAPI规范可直接导入API Fox

### 3. 实现步骤

#### 3.1 添加依赖

```bash
# 添加hertz-contrib/swagger依赖
go get -u github.com/hertz-contrib/swagger
```

#### 3.2 创建Swagger配置文件

在项目中创建一个专门的文档配置文件，用于定义API的基本信息。

#### 3.3 为处理器方法添加Swagger注释

为所有的HTTP处理器方法添加符合OpenAPI规范的注释，包括：
- 接口描述
- 请求参数说明
- 响应结构定义
- 状态码说明

#### 3.4 在Hertz中集成Swagger中间件

修改`pkg/hertz/hertz_manager.go`，在初始化过程中集成Swagger中间件。

#### 3.5 提供Swagger UI访问端点

配置Swagger UI的访问路径，便于开发人员查看和测试API。

### 4. 注释规范示例

为了确保文档自动生成的准确性，我会制定一套注释规范，例如：

```go
// Register 用户注册接口
// @Summary 用户注册
// @Description 用户通过邮箱和密码进行注册
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param request body service.RegisterRequest true "注册请求参数"
// @Success 200 {object} map[string]interface{} "注册成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Router /api/v1/user/register [post]
func (h *UserHandler) Register(c context.Context, ctx *app.RequestContext) {
    // 实现代码...
}
```

### 5. 文档生成和导出

- 开发环境：通过Swagger UI实时查看接口文档
- 生产环境：可导出为OpenAPI规范文件（JSON/YAML），便于导入API Fox

### 6. 实施计划

1. 首先设置基础环境和配置
2. 为现有API添加Swagger注释
3. 集成Swagger中间件
4. 测试文档生成和API Fox导入功能
5. 编写团队使用文档，确保后续新增接口也遵循相同规范

## 后续维护

为了确保接口文档的持续更新，我会建议：

- 在代码审查流程中添加文档检查
- 为开发团队提供Swagger注释编写指南
- 考虑添加自动化测试，确保所有API都有对应的文档

这个方案可以有效解决接口文档自动生成的需求，同时确保新添加的接口也能自动包含在文档中。
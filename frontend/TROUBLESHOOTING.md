# 前端请求问题排查指南

## 问题：面试接口返回404和CORS错误

### 错误信息
```
已拦截跨源请求：同源策略禁止读取位于 http://localhost:8888/api/interview/start/stream 的远程资源。
（原因：CORS 头缺少 'Access-Control-Allow-Origin'）。状态码：404。
```

### 问题原因

404错误通常意味着：
1. 后端服务未启动
2. 后端路由未正确配置
3. JWT中间件拦截了请求（因为路径不在公共路由列表中）

### 解决方案

#### 方案1：检查后端服务（推荐先执行）

1. 确认后端服务已启动：
```bash
cd backend
go run main.go
```

2. 检查后端是否监听在8888端口：
```bash
# Windows
netstat -ano | findstr :8888

# Linux/Mac
lsof -i :8888
```

3. 使用前端诊断工具：
   - 访问社招面试页面
   - 点击右上角"后端服务诊断"按钮
   - 查看诊断结果

#### 方案2：修改后端路由配置（如果诊断显示404）

编辑 `backend/api/router/interview/middleware.go` 文件：

```go
var jwtPublicRoutes = map[string]struct{}{
	"/api/user/login":              {},
	"/api/user/register":           {},
	"/api/user/logout":             {},
	"/api/user/wechat/login":       {},
	"/api/user/wechat/callback":    {},
	"/api/interview/records/mock":  {},
	"/api/interview/titleBank":     {},
	"/api/interview/titleBank/:id": {},
	// 添加以下两行 👇
	"/api/interview/start/stream":  {},
	"/api/interview/submit/answer": {},
}
```

添加后重启后端服务。

#### 方案3：检查Token

确保浏览器localStorage中有有效的token：

1. 打开浏览器开发者工具（F12）
2. 切换到Application标签
3. 查看Local Storage
4. 确认有`token`键值对

如果没有token或token过期，请先登录。

### 前端调试

前端已添加详细的日志输出，打开浏览器控制台可以看到：

```
[检测] 测试后端服务连接...
[检测] 后端服务连接正常
[面试启动] 尝试方案1: 使用Authorization header
[面试启动] 请求URL: http://localhost:8888/api/interview/start/stream
[面试启动] 收到响应: 404 Not Found
```

根据日志输出可以判断问题所在。

### 常见问题

**Q: 后端服务已启动，但仍然404**
A: 检查后端路由配置，确保 `/api/interview/start/stream` 在公共路由列表中

**Q: CORS错误**
A: 404会导致后端不返回CORS头，先解决404问题

**Q: 登录已过期**
A: 重新登录获取新的token

**Q: 无法连接到后端服务**
A: 检查后端是否运行在 http://localhost:8888

### 技术细节

前端使用了以下技术处理SSE流式响应：
- `fetch` API
- `ReadableStream`
- 自动重试机制（Authorization header失败时尝试URL参数）
- 详细的错误日志

后端使用SSE（Server-Sent Events）推送数据，格式为：
```
data: {"type":"start","message":"面试已开始","session_id":"xxx"}

data: {"type":"question","index":1,"data":{"question_text":"问题内容"}}
```

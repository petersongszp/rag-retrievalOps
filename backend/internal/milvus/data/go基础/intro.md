# Go 基础入门

本文档涵盖 Go 基础语法、类型系统、控制结构、函数与方法、包与模块等内容。

## 目录
- 基础语法
- 类型与变量
- 控制结构
- 函数与方法
- 包与模块管理

## 基础语法
Go 使用静态类型、编译型，内置并发原语和垃圾回收。

- 程序入口为 `main` 包下的 `main` 函数
- 大写标识符导出，小写为包内可见
- 使用 `go fmt` 统一格式，`go vet` 做静态检查

示例：

```go
package main

import "fmt"

func main() {
	fmt.Println("Hello, Go")
}
```

## 类型与变量
内置基础类型：`bool`、数值（`int`/`int64`/`float64` 等）、`string`，以及复合类型：`array`、`slice`、`map`、`struct`、`pointer`、`function`、`interface`、`channel`。

- 变量声明：`var x int`、短变量声明：`x := 10`
- 常量：`const Pi = 3.14159`
- 切片：`make([]int, 0, 10)`；追加 `append`
- 映射：`m := map[string]int{"a": 1}`
- 结构体：

```go
type User struct {
	ID   int64
	Name string
	Tags []string
}
```

## 控制结构
- `for` 支持三段式、`while` 风格与 `range`
- `if` 支持初始化语句：`if v := f(); v > 0 { ... }`
- `switch` 自动 `break`；支持表达式与类型开关
- `defer` 延迟调用（LIFO），常用于资源释放

```go
for i := 0; i < 3; i++ {}
for i < n {}
for k, v := range m { _ = k; _ = v }
```

## 函数与方法
- 多返回值与命名返回值：`func f() (int, error)`
- 可变参数：`func sum(nums ...int) int`
- 方法接收者可为值或指针，指针接收者可修改状态

```go
type Counter int
func (c *Counter) Inc() { *c++ }
```

## 包与模块管理
使用 Go Modules 进行依赖管理。

- 初始化：`go mod init example.com/project`
- 引用：`go get` 增加依赖；`go mod tidy` 清理
- 构建与运行：`go build`、`go run`、`go test`

## 接口与错误处理
- 接口隐式实现；倾向小接口与组合
- 错误标准类型 `error`；使用 `fmt.Errorf("...: %w", err)` 包装
- `errors.Is/As` 做错误判定与解包

```go
type Reader interface{ Read(p []byte) (int, error) }
```

## 并发基础
- `goroutine`：`go f()` 启动协程
- `channel`：通信与同步；有/无缓冲
- `context`：取消与超时；跨 API 传递
- 同步：`sync.WaitGroup`、`sync.Mutex`、`atomic`

```go
ch := make(chan int, 1)
go func() { ch <- 1 }()
v := <-ch
_ = v
```

## 常用工具与项目结构
- 工具：`go fmt`, `go vet`, `golangci-lint`, `staticcheck`
- 测试：`testing` 子测试/基准测试（`go test -bench=.`）
- 结构建议：`cmd/`, `internal/`, `pkg/`, `configs/`, `scripts/`

## 最佳实践清单
- 用组合代替继承，接口最小化
- 错误显式处理并包裹根因，不要忽略错误
- 使用 `context` 管理取消/超时；避免全局可变状态
- 基于基准测试优化，避免过早优化



# Go 专项：并发与性能优化

本专题深入探讨 Go 并发模型、调度器、内存模型，以及常见性能优化手段。

## 目录
- Goroutine 与调度
- Channel 模式
- sync 包与原子操作
- 内存模型与逃逸分析
- 性能优化实践

## Goroutine 与调度
GMP 调度模型支撑了高并发能力，需要结合实际场景进行合理使用。

### GMP 简述
- G：goroutine，可执行的协程单元
- M：machine，系统线程
- P：processor，调度器的本地运行队列，数量由 `GOMAXPROCS` 决定
- 窃取调度：当本地队列空时从其他 P 窃取

### 常见注意点
- 阻塞系统调用会占用 M，合理设置 `GOMAXPROCS`
- 避免创建过量 goroutine，配合 worker pool/限流
- 谨慎在 `init()` 或包级变量中启动 goroutine

## Channel 模式
### 典型模式
- Pipeline：分段处理，提升吞吐
- Fan-out/Fan-in：并行处理-汇聚结果
- Worker Pool：控制并发度
- Producer-Consumer：缓冲区解耦

```go
func worker(id int, jobs <-chan int, results chan<- int) {
	for j := range jobs {
		results <- j * 2
	}
}
```

### 关闭与广播
- 仅发送方关闭 channel；读取方通过 `v, ok := <-ch` 判断
- 广播可用 `close(ch)` 或 `context` 取消

## sync 包与原子操作
- `sync.WaitGroup`：等待 goroutine 完成
- `sync.Mutex/RWMutex`：互斥与读写锁；读多写少用 RWMutex
- `sync.Cond`：条件变量场景
- `sync.Map`：读多写少，避免频繁分配对象
- `atomic`：无锁原子操作；在热点路径上替代互斥

## 内存模型与逃逸分析
- `go build -gcflags='all=-d=checkptr=0 -m=2'` 查看逃逸
- 指针从栈逃逸到堆会增加 GC 压力；减少不必要的堆分配
- `sync.Pool` 复用临时对象，注意生命周期不可跨请求

## 性能优化实践
- 基准测试：`go test -bench=. -benchmem`
- 分析工具：`pprof`（CPU/内存/阻塞/互斥）、`trace`
- 字符串构建：`strings.Builder`/`bytes.Buffer`
- JSON 编解码优化：复用 buffer、避免反射（如 `easyjson`）
- GC 相关：减少小对象分配、合并分配、复用切片容量

### 常见陷阱
- 在 `range` 循环中捕获迭代变量
- 忘记关闭闲置 `http.Response.Body`
- map 并发写导致竞态；用锁或分片 map
- `time.After` 泄漏：用 `time.NewTimer` 并 `Stop`



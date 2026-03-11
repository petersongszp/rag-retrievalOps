# Java 专项：JVM 调优与故障诊断

本专题聚焦 JVM 运行时、内存区域、GC 算法、性能调优与排障实践。

## 目录
- JVM 内存结构
- 垃圾回收器
- GC 日志分析
- 常见性能问题定位
- 调优案例

## JVM 内存结构
掌握堆、栈、方法区等内存区域的特征与常见问题。

### 运行时内存
- 线程私有：程序计数器、虚拟机栈、本地方法栈
- 线程共享：堆（Young/Old）、方法区（Metaspace）
- 对象生命周期：新生代（Eden→Survivor）晋升老年代

## 垃圾回收器
- 串行/并行：Serial/Parallel（吞吐优先）
- CMS（低停顿，已废弃）、G1（服务端默认，分区化，GC 目标可控）
- ZGC、Shenandoah（超低停顿，区域化/并发压缩）
- 选择建议：延迟敏感→G1/ZGC；吞吐敏感→Parallel

## GC 日志分析
- 启用：`-Xlog:gc*:file=gc.log:tags,uptime,time,level`（JDK9+）
- 关注：停顿时间、晋升失败、频繁 Full GC、混合回收比例
- 工具：GC Easy、GCViewer、JDK Mission Control

## 常见性能问题定位
- CPU 高：采样火焰图（async-profiler）找热点
- 内存泄漏：`jmap -histo`、`jcmd GC.heap_dump` + MAT 分析
- 线程阻塞：`jstack`、JFR 事件看锁竞争与 safepoint
- GC 频繁：检查对象分配速率、晋升速率、TLAB 与大对象

## 调优步骤与参数示例
1) 明确 SLO：吞吐/延迟/内存上限/峰值
2) 采集基线数据（CPU/内存/GC/响应时间）
3) 定位瓶颈并假设→实验→回归验证

参数模板（G1）：
```
-XX:+UseG1GC
-XX:MaxGCPauseMillis=200
-XX:InitiatingHeapOccupancyPercent=30
-XX:MaxTenuringThreshold=8
-XX:+ParallelRefProcEnabled
-Xms4g -Xmx4g
```

## 调优案例提示
- 瞬时 QPS 峰值导致 YGC 频繁：扩大 Eden 或平滑流量
- 大对象直入老年代触发 Full GC：增大 `-XX:G1HeapRegionSize`/合理分配
- 对象逃逸严重：减少装箱与临时对象、复用缓冲、优化热路径

## 工具箱
- JFR/JMC、async-profiler、Arthas、BTrace
- Prometheus + Grafana（导出 JVM 指标）
- Chaos 工具注入延迟/故障验证韧性



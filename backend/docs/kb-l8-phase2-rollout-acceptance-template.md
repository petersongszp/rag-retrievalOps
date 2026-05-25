# Phase 2 L8 灰度发布与验收模板

## 1. 发布信息

- 发布日期：
- 发布负责人：
- 当前阶段：`internal / small_flow / batch / full`
- 运行策略摘要：
- 回滚负责人：

## 2. 灰度过程

1. 内部全量验证
   - 时间窗口：
   - 结论：
2. 小流量灰度
   - 百分比：
   - 结论：
3. 分批扩量
   - 百分比：
   - 结论：

## 3. 关键指标

- Retrieve P95：
- Recall / MRR / nDCG / Citation Accuracy：
- Empty-After-Filter 占比：
- Rewrite Hit Rate：
- Rerank P95：
- Dense / Sparse 路由贡献：

## 4. 风险与告警

- 是否触发 L8 告警：
- 告警详情：
- 是否需要回滚：
- 结论：

## 5. 回滚演练

1. `/api/admin/kb/release/rollback`
2. 验证 Phase 1 策略恢复
3. 验证关键指标回稳

- 演练结果：
- 恢复耗时：

## 6. 验收结论

- 是否通过：
- 是否进入 Phase 3：
- 遗留问题：

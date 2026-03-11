# 自动化评估集构建与测试指南

## 1. 概述
为了量化面试智能体 (Interview Agent) 的回答质量和准确率，我们构建了一套自动化评估流程。该流程包含“真值数据集” (Ground Truth Dataset) 和自动化测试脚本。

## 2. 数据集结构
数据集位于 `backend/test/evaluation/dataset.json`，采用 JSON 格式存储。

### 字段说明
- `id`: 用例唯一标识
- `domain`: 领域 (Go, Java, MySQL 等)
- `question`: 面试问题
- `reference_answer`: 标准参考答案
- `difficulty`: 难度等级

### 示例
```json
{
  "id": "go-basic-1",
  "domain": "Go",
  "question": "Go 语言中的 slice 和 array 有什么区别？",
  "reference_answer": "Array 是固定长度的，是值类型；Slice 是动态长度的...",
  "difficulty": "Easy"
}
```

## 3. 评估方法
目前的评估脚本位于 `backend/test/evaluation/evaluate_test.go`。

### 3.1 运行流程
1. **加载数据**: 读取 `dataset.json`。
2. **执行 Agent**: 模拟或实际调用 Agent 接口，输入问题，获取 `Agent Answer`。
3. **评分 (Evaluation)**: 将 `Agent Answer` 与 `Reference Answer` 进行对比。
   - **当前实现**: 简单的关键词匹配模拟。
   - **进阶规划**: 使用 LLM-as-a-Judge 模式，调用 GPT-4 或 Claude 对一致性进行打分 (0-1分)。

### 3.2 运行命令
```bash
cd backend/test/evaluation
go test -v evaluate_test.go
```

## 4. 扩展计划
1. **接入真实 Agent**: 修改 `runAgent` 函数，集成 `interview_agent_service`。
2. **LLM 评分**: 引入 `Judgement Agent`，根据准确性、完整性、清晰度三个维度打分。
3. **CI/CD 集成**: 将评估测试加入 GitLab CI/GitHub Actions，每次代码合并前自动运行回归测试。

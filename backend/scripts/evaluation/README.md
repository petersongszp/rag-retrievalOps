# 12.3 评估与测试：自动化评估集与 Ragas/TruLens 维度

本目录实现书中 **12.3.2 自动化评估集构建（引入 Ragas/TruLens 评估维度）**。

## 评估集格式 (dataset.json)

每条记录包含：
- `question`: 面试/问答题目
- `context`: 背景知识或检索上下文
- `ground_truth`: 参考答案或评估标准

可选：`answer` 若已预先采集 Agent 回答，可直接用于评估，否则由脚本调用 API 或使用 Mock。

## 评估维度说明

| 框架 | 维度 | 含义 |
|------|------|------|
| **Ragas** | faithfulness | 回答与给定 context 的忠实程度 |
| **Ragas** | answer_relevancy | 回答与问题的相关度 |
| **Ragas** | context_recall | 回答对 ground_truth 的召回程度 |
| **TruLens** | answer_relevancy | 回答与问题的相关性 (RAG Triad 之一) |
| **TruLens** | context_relevance | 检索上下文与问题的相关性 |
| **TruLens** | groundedness | 回答是否基于上下文、有无幻觉 |
| **TruLens** | ground_truth_agreement | 与参考答案的语义一致性 |

## 环境准备

1. 安装依赖：
   ```bash
   pip install -r requirements.txt
   ```

2. 设置 OpenAI API Key（Ragas / TruLens 均会用到）：
   ```bash
   export OPENAI_API_KEY="your-api-key"
   ```

3. 若需对接真实 Agent，请先启动 Go 服务（如 `cd ../.. && go run cmd/server/main.go`）。

## 使用方式

### Ragas 评估 (evaluate.py)

```bash
python evaluate.py           # 调用 Go API 获取回答后评估（需服务运行）
python evaluate.py --no-api  # 使用 Mock 回答，仅跑通流程
```

报告输出：`evaluation_report.json`。

### TruLens 评估维度 (evaluate_trulens.py)

```bash
python evaluate_trulens.py           # 使用 OpenAI 计算 answer_relevancy / groundedness 等
python evaluate_trulens.py --no-api  # 仅生成维度占位报告（可不设 OPENAI_API_KEY）
```

报告输出：`trulens_report.json`。

## 文件说明

- **dataset.json**: 评估集；可增删条目以扩展自动化评估集。
- **evaluate.py**: Ragas 评估主脚本。
- **evaluate_trulens.py**: TruLens 评估维度示例（Answer Relevance、Groundedness 等）。
- **evaluation_report.json** / **trulens_report.json**: 运行后生成的报告。

## 扩展评估集

在 `dataset.json` 中追加条目即可，格式：

```json
{
  "question": "题目文本",
  "context": "背景或检索到的上下文",
  "ground_truth": "参考答案或评分要点"
}
```

## 故障排查

- **Ragas 报错**: 确认 `OPENAI_API_KEY` 已设置；`ragas` 版本过新时若 API 变更可参考 [Ragas 文档](https://docs.ragas.io/) 调整 `evaluate.py`。
- **TruLens 报错**: 确认已安装 `trulens` 与 `trulens-providers-openai`；Provider 方法名随版本可能不同，可查阅 [TruLens 文档](https://www.trulens.org/)。
- **Go API 连接失败**: 确保后端服务运行在 `API_BASE_URL`（默认 `http://localhost:8899`）。

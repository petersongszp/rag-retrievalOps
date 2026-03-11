#!/usr/bin/env python3
"""
12.3.2 TruLens 评估维度示例

对 dataset.json 中的 (question, context, answer, ground_truth) 使用 TruLens 常用维度：
- Answer Relevance：回答与问题的相关性（Direct Provider Call）
- Groundedness：回答与上下文的依据程度（可选）
- Ground Truth Agreement：与参考答案的语义一致性（可选，需 GroundTruthAgreement）

使用方式:
  python evaluate_trulens.py           # 需要 OPENAI_API_KEY，answer 用 mock
  python evaluate_trulens.py --no-api  # 仅 mock，用于检查脚本与数据集
"""

import json
import os
import sys
from typing import List, Dict, Any

DATASET_FILE = "dataset.json"
REPORT_FILE = "trulens_report.json"


def load_dataset(file_path: str) -> List[Dict]:
    with open(file_path, "r", encoding="utf-8") as f:
        return json.load(f)


def run_trulens_dimensions(dataset: List[Dict], use_mock_answer: bool) -> Dict[str, Any]:
    """
    使用 TruLens Provider 直接对 (prompt, response) 做 Answer Relevance 等维度。
    """
    try:
        from trulens.providers.openai import OpenAI as fOpenAI
    except ImportError:
        print("未安装 trulens，仅输出维度说明。安装: pip install trulens trulens-providers-openai")
        return {
            "dimensions": [
                "answer_relevancy",
                "context_relevance",
                "groundedness",
                "ground_truth_agreement",
            ],
            "note": "安装 trulens 并设置 OPENAI_API_KEY 后可得到真实分数",
            "num_items": len(dataset),
        }

    if not os.environ.get("OPENAI_API_KEY"):
        print("未设置 OPENAI_API_KEY，将只生成维度占位报告")
        return {
            "dimensions": [
                "answer_relevancy",
                "context_relevance",
                "groundedness",
                "ground_truth_agreement",
            ],
            "note": "设置 OPENAI_API_KEY 后可得到真实分数",
            "num_items": len(dataset),
        }

    provider = fOpenAI()
    results = []
    for item in dataset:
        question = item["question"]
        context = item["context"]
        ground_truth = item["ground_truth"]
        answer = (
            item.get("answer")
            if not use_mock_answer
            else f"[Mock] 针对「{question[:40]}...」的模拟回答"
        )

        row = {"question": question[:80]}
        try:
            # Answer Relevance: 回答与问题的相关性 (0-1)
            rel = provider.relevance(question, answer)
            if rel is not None:
                row["answer_relevancy"] = float(rel) if hasattr(rel, "__float__") else rel
        except Exception as e:
            row["answer_relevancy"] = None
            row["error_relevancy"] = str(e)

        try:
            # Groundedness: 回答是否基于 context（可选）
            g = provider.groundedness_measure(context, answer)
            if g is not None:
                row["groundedness"] = float(g) if hasattr(g, "__float__") else g
        except Exception as e:
            row["groundedness"] = None
            row["error_groundedness"] = str(e)

        results.append(row)

    # 聚合
    agg = {"num_items": len(results)}
    relevancies = [r.get("answer_relevancy") for r in results if r.get("answer_relevancy") is not None]
    grounded = [r.get("groundedness") for r in results if r.get("groundedness") is not None]
    if relevancies:
        agg["answer_relevancy_mean"] = sum(relevancies) / len(relevancies)
    if grounded:
        agg["groundedness_mean"] = sum(grounded) / len(grounded)
    agg["per_item"] = results
    return agg


def main():
    print("TruLens 评估维度 (12.3.2)")
    print("=" * 60)

    if not os.path.exists(DATASET_FILE):
        print(f"错误: 未找到 {DATASET_FILE}")
        sys.exit(1)

    data = load_dataset(DATASET_FILE)
    print(f"加载 {len(data)} 条数据")
    use_mock = "--no-api" in sys.argv
    if use_mock:
        print("使用 Mock 回答 (--no-api)")

    report = run_trulens_dimensions(data, use_mock_answer=use_mock)
    print("\n结果摘要:")
    for k, v in report.items():
        if k == "per_item":
            continue
        if isinstance(v, float):
            print(f"  {k}: {v:.4f}")
        else:
            print(f"  {k}: {v}")

    with open(REPORT_FILE, "w", encoding="utf-8") as f:
        json.dump(report, f, indent=2, ensure_ascii=False)
    print(f"\n报告已写入: {REPORT_FILE}")


if __name__ == "__main__":
    main()

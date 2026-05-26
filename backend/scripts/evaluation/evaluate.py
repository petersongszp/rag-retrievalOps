#!/usr/bin/env python3
"""
Evaluation entrypoint for both retrieval regression and legacy Ragas answer evaluation.

Examples:
  python evaluate.py
  python evaluate.py --mode retrieval --candidate "parent_child+advanced_rewrite"
  python evaluate.py --mode ragas --no-api
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Dict, List

import requests
from datasets import Dataset
from ragas import evaluate
from ragas.metrics import answer_relevancy, context_recall, faithfulness


SCRIPT_DIR = Path(__file__).resolve().parent
BACKEND_DIR = SCRIPT_DIR.parent.parent
API_BASE_URL = os.getenv("API_BASE_URL", "http://localhost:8899")
DATASET_FILE = SCRIPT_DIR / "dataset.json"
RAGAS_REPORT_FILE = SCRIPT_DIR / "evaluation_report.json"
RETRIEVAL_OUTPUT_PREFIX = BACKEND_DIR / "docs" / "retrieval-regression-report"


def load_dataset(file_path: Path) -> List[Dict]:
    with file_path.open("r", encoding="utf-8") as file:
        payload = json.load(file)
    if isinstance(payload, dict):
        return payload.get("cases", [])
    return payload


def call_agent_api(question: str) -> str:
    try:
        url = f"{API_BASE_URL}/api/mianshi/stream/start"
        payload = {
            "question": question,
            "session_id": "eval_session",
        }
        response = requests.post(url, json=payload, timeout=30)
        response.raise_for_status()
        data = response.json()
        return data.get("answer", "")
    except Exception as exc:
        print(f"Error calling API: {exc}")
        return f"[API Error: {exc}]"


def prepare_ragas_dataset(dataset: List[Dict], include_api_calls: bool = True) -> Dataset:
    questions, contexts, answers, ground_truths = [], [], [], []

    for item in dataset:
        question = item.get("question") or item.get("query")
        context = item.get("context", "")
        ground_truth = item.get("ground_truth", "")
        if not question or not ground_truth:
            continue

        if include_api_calls:
            print(f"Calling API for question: {question[:50]}...")
            answer = call_agent_api(question)
        else:
            answer = f"Mock answer for: {question}"

        questions.append(question)
        contexts.append([context])
        answers.append(answer)
        ground_truths.append(ground_truth)

    return Dataset.from_dict(
        {
            "question": questions,
            "contexts": contexts,
            "answer": answers,
            "ground_truth": ground_truths,
        }
    )


def run_ragas_evaluation(dataset: Dataset) -> Dict:
    metrics = [faithfulness, answer_relevancy, context_recall]
    return evaluate(dataset, metrics=metrics)


def save_json_report(results: Dict, file_path: Path) -> None:
    with file_path.open("w", encoding="utf-8") as file:
        json.dump(results, file, indent=2, ensure_ascii=False)


def run_retrieval_mode(args: argparse.Namespace) -> int:
    command = [
        "go",
        "run",
        "./cmd/retrieval-eval",
        "-config",
        args.config,
        "-dataset",
        args.dataset,
        "-output",
        args.output,
    ]
    if args.profiles:
        command.extend(["-profiles", args.profiles])
    if args.gates:
        command.extend(["-gates", args.gates])
    if args.baseline:
        command.extend(["-baseline", args.baseline])
    if args.candidate:
        command.extend(["-candidate", args.candidate])
    if args.collection:
        command.extend(["-collection", args.collection])

    print("Running retrieval regression:")
    print(" ".join(command))
    completed = subprocess.run(command, cwd=BACKEND_DIR, check=False)
    if completed.returncode == 0:
        print(f"\nRetrieval regression completed. Report prefix: {args.output}")
    elif completed.returncode == 2:
        print(f"\nRetrieval regression completed but gate failed. Report prefix: {args.output}")
    return completed.returncode


def run_ragas_mode(args: argparse.Namespace) -> int:
    dataset_path = Path(args.dataset)
    if not dataset_path.exists():
        print(f"Error: Dataset file '{dataset_path}' not found!")
        return 1

    raw_dataset = load_dataset(dataset_path)
    include_api_calls = not args.no_api
    ragas_dataset = prepare_ragas_dataset(raw_dataset, include_api_calls)

    try:
        results = run_ragas_evaluation(ragas_dataset)
        results_dict = dict(results)
        save_json_report(results_dict, Path(args.ragas_report))
        print(json.dumps(results_dict, indent=2, ensure_ascii=False))
        print(f"\nRagas report saved to: {args.ragas_report}")
        return 0
    except Exception as exc:
        print(f"\nRagas evaluation failed: {exc}")
        return 1


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Evaluation entrypoint")
    parser.add_argument("--mode", choices=["retrieval", "ragas"], default="retrieval")
    parser.add_argument("--config", default="config.yaml")
    parser.add_argument("--dataset", default=str(DATASET_FILE))
    parser.add_argument("--profiles", default=str(SCRIPT_DIR / "retrieval_strategy_profiles.example.json"))
    parser.add_argument("--gates", default=str(SCRIPT_DIR / "retrieval_gate_thresholds.example.json"))
    parser.add_argument("--output", default=str(RETRIEVAL_OUTPUT_PREFIX))
    parser.add_argument("--baseline", default="")
    parser.add_argument("--candidate", default="")
    parser.add_argument("--collection", default="")
    parser.add_argument("--no-api", action="store_true", help="Only used in ragas mode")
    parser.add_argument("--ragas-report", default=str(RAGAS_REPORT_FILE))
    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    if args.mode == "retrieval":
        return run_retrieval_mode(args)
    return run_ragas_mode(args)


if __name__ == "__main__":
    sys.exit(main())

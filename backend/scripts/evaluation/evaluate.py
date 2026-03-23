#!/usr/bin/env python3
"""
Agent Evaluation Script using Ragas

This script evaluates the interview agent's responses using Ragas metrics.
It loads questions from dataset.json, calls the Go API to get responses,
and evaluates them using Ragas.
"""

import json
import os
import sys
from typing import List, Dict
import requests
from datasets import Dataset
from ragas import evaluate
from ragas.metrics import faithfulness, answer_relevancy, context_recall

# Configuration
API_BASE_URL = os.getenv("API_BASE_URL", "http://localhost:8899")
DATASET_FILE = "dataset.json"
REPORT_FILE = "evaluation_report.json"


def load_dataset(file_path: str) -> List[Dict]:
    """Load evaluation dataset from JSON file."""
    with open(file_path, 'r', encoding='utf-8') as f:
        return json.load(f)


def call_agent_api(question: str) -> str:
    """
    Call the Go API to get agent response.
    
    Note: This is a simplified example. You may need to adjust the endpoint
    and request format based on your actual API structure.
    """
    try:
        # Example endpoint - adjust based on your actual API
        url = f"{API_BASE_URL}/api/mianshi/stream/start"
        
        payload = {
            "question": question,
            "session_id": "eval_session",
            # Add other required fields based on your API
        }
        
        response = requests.post(url, json=payload, timeout=30)
        response.raise_for_status()
        
        # Extract answer from response
        # Adjust based on your actual response structure
        data = response.json()
        return data.get("answer", "")
        
    except Exception as e:
        print(f"Error calling API: {e}")
        return f"[API Error: {e}]"


def prepare_ragas_dataset(dataset: List[Dict], include_api_calls: bool = True) -> Dataset:
    """
    Prepare dataset in Ragas format.
    
    Args:
        dataset: List of evaluation items
        include_api_calls: If True, call API to get actual responses
    
    Returns:
        Dataset object for Ragas evaluation
    """
    questions = []
    contexts = []
    answers = []
    ground_truths = []
    
    for item in dataset:
        question = item["question"]
        context = item["context"]
        ground_truth = item["ground_truth"]
        
        # Get agent response
        if include_api_calls:
            print(f"Calling API for question: {question[:50]}...")
            answer = call_agent_api(question)
        else:
            # For testing without API
            answer = f"Mock answer for: {question}"
        
        questions.append(question)
        contexts.append([context])  # Ragas expects list of contexts
        answers.append(answer)
        ground_truths.append(ground_truth)
    
    # Create Ragas dataset
    data = {
        "question": questions,
        "contexts": contexts,
        "answer": answers,
        "ground_truth": ground_truths
    }
    
    return Dataset.from_dict(data)


def run_evaluation(dataset: Dataset) -> Dict:
    """
    Run Ragas evaluation on the dataset.
    
    Returns:
        Evaluation results dictionary
    """
    print("\nRunning Ragas evaluation...")
    
    # Define metrics to evaluate
    metrics = [
        faithfulness,        # How faithful is the answer to the context
        answer_relevancy,    # How relevant is the answer to the question
        context_recall,      # How well does the answer recall the ground truth
    ]
    
    # Run evaluation
    results = evaluate(dataset, metrics=metrics)
    
    return results


def save_report(results: Dict, file_path: str):
    """Save evaluation report to JSON file."""
    with open(file_path, 'w', encoding='utf-8') as f:
        json.dump(results, f, indent=2, ensure_ascii=False)
    print(f"\nReport saved to: {file_path}")


def print_results(results: Dict):
    """Print evaluation results to console."""
    print("\n" + "="*60)
    print("EVALUATION RESULTS")
    print("="*60)
    
    for metric, score in results.items():
        if isinstance(score, (int, float)):
            print(f"{metric:.<40} {score:.4f}")
    
    print("="*60)


def main():
    """Main evaluation workflow."""
    print("Agent Evaluation Script")
    print("="*60)
    
    # Check if dataset exists
    if not os.path.exists(DATASET_FILE):
        print(f"Error: Dataset file '{DATASET_FILE}' not found!")
        sys.exit(1)
    
    # Load dataset
    print(f"Loading dataset from {DATASET_FILE}...")
    raw_dataset = load_dataset(DATASET_FILE)
    print(f"Loaded {len(raw_dataset)} evaluation items")
    
    # Prepare Ragas dataset
    # Set include_api_calls=False for testing without running server
    include_api_calls = "--no-api" not in sys.argv
    
    if not include_api_calls:
        print("\nRunning in NO-API mode (using mock responses)")
    
    ragas_dataset = prepare_ragas_dataset(raw_dataset, include_api_calls)
    
    # Run evaluation
    try:
        results = run_evaluation(ragas_dataset)
        
        # Print results
        print_results(results)
        
        # Save report
        save_report(dict(results), REPORT_FILE)
        
        print("\n✅ Evaluation completed successfully!")
        
    except Exception as e:
        print(f"\n❌ Evaluation failed: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()

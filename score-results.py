from openai import OpenAI
import os
import sys
import json
import re
import yaml
from datetime import datetime
from pathlib import Path


def load_config() -> dict:
    """Load configuration from YAML file."""
    config_file = Path("scoring_config.yaml")

    if not config_file.exists():
        raise FileNotFoundError(
            f"Configuration file not found: {config_file}\n"
            f"Please create a scoring_config.yaml file."
        )

    with open(config_file, "r") as f:
        config = yaml.safe_load(f)

    return config


def create_client(config: dict):
    """Create API client for scoring based on configuration."""
    if config.get("api") == "anthropic":
        try:
            from anthropic import Anthropic

            key = os.environ.get("ANTHROPIC_API_KEY", "")
            if key == "":
                raise ValueError("ANTHROPIC_API_KEY environment variable must be set.")

            return Anthropic(api_key=key)
        except ImportError:
            raise ImportError(
                "anthropic package not installed. Install with: uv add anthropic"
            )
    elif config.get("api") == "local":
        return OpenAI(
            api_key="not-needed",
            base_url=config.get("endpoint"),
        )
    else:
        raise ValueError(
            f"Unsupported scoring API: {config.get('api')}. Supported: 'local', 'anthropic'"
        )


EVALUATION_PROMPT = """You are a research assistant, evaluating the responses to some exam questions on Kubernetes.

The user submits questions and answers, both the expected answers as well as the actual answers provided by a candidate.

Your task is to evaluate whether the actual answer is correct or not, and then count the number of correct answers. Any single answer may only be correct or incorrect.

Correct means that the answer contains the necessary information. A correct answer is not necessarily identical to the expected answer.

Example input:

---
NO. 2 - Setup & Aliases
QUESTION: How do you enable bash autocompletion for the 'k' alias?
EXPECTED ANSWER: complete -F __start_kubectl k
ACTUAL ANSWER: ```bash
source <(kubectl completion bash)
alias k=kubectl
complete -F __start_kubectl k
```

Example output:

58 out of 100 answers are correct.
"""


def evaluate_with_anthropic(
    config: dict, client, content: str, run_number: int, total_runs: int, verbose: bool = True
) -> str:
    """Evaluate using Anthropic API with streaming."""
    if verbose:
        print(f"\n{'=' * 80}")
        print(f"Evaluation Run {run_number}/{total_runs}")
        print(f"{'=' * 80}")
        print("Sending results to Anthropic API for evaluation...\n")

    try:
        # Use streaming with Anthropic
        full_response = ""
        if verbose:
            print("LLM Response: ", end="", flush=True)

        with client.messages.stream(
            model=config.get("model"),
            max_tokens=4096,
            system=EVALUATION_PROMPT,
            messages=[{"role": "user", "content": content}],
        ) as stream:
            for text in stream.text_stream:
                full_response += text
                if verbose:
                    print(text, end="", flush=True)

        if verbose:
            print("\n")

        return full_response

    except Exception as e:
        # Fallback to non-streaming
        if verbose:
            print(f"Streaming failed ({e}), using standard mode...\n")

        response = client.messages.create(
            model=config.get("model"),
            max_tokens=4096,
            system=EVALUATION_PROMPT,
            messages=[{"role": "user", "content": content}],
        )

        result = response.content[0].text
        if verbose:
            print(f"LLM Response: {result}\n")

        return result


def evaluate_with_local(
    config: dict, client, content: str, run_number: int, total_runs: int, verbose: bool = True
) -> str:
    """Evaluate using local OpenAI-compatible API with streaming."""
    if verbose:
        print(f"\n{'=' * 80}")
        print(f"Evaluation Run {run_number}/{total_runs}")
        print(f"{'=' * 80}")
        print("Sending results to local LLM for evaluation...\n")

    try:
        # Try streaming with chat completions API
        response = client.chat.completions.create(
            model=config.get("model"),
            messages=[
                {"role": "system", "content": EVALUATION_PROMPT},
                {"role": "user", "content": content},
            ],
            stream=True,
        )

        # Stream the response
        full_response = ""
        if verbose:
            print("LLM Response: ", end="", flush=True)

        for chunk in response:
            if chunk.choices[0].delta.content:
                content_chunk = chunk.choices[0].delta.content
                full_response += content_chunk
                if verbose:
                    print(content_chunk, end="", flush=True)

        if verbose:
            print("\n")

        return full_response

    except Exception:
        # Fallback to non-streaming if streaming fails
        if verbose:
            print("Streaming not available, using standard mode...\n")

        response = client.chat.completions.create(
            model=config.get("model"),
            messages=[
                {"role": "system", "content": EVALUATION_PROMPT},
                {"role": "user", "content": content},
            ],
        )

        result = response.choices[0].message.content
        if verbose:
            print(f"LLM Response: {result}\n")

        return result


def evaluate_results(
    config: dict, client, content: str, run_number: int, total_runs: int, verbose: bool = True
) -> str:
    """Send results to LLM for evaluation with streaming output."""
    if config.get("api") == "anthropic":
        return evaluate_with_anthropic(config, client, content, run_number, total_runs, verbose)
    else:
        return evaluate_with_local(config, client, content, run_number, total_runs, verbose)


def parse_score(result_text: str) -> dict:
    """Parse the score from the LLM output."""
    # Try to extract "X out of Y" pattern
    match = re.search(r"(\d+)\s+out\s+of\s+(\d+)", result_text)
    if match:
        correct = int(match.group(1))
        total = int(match.group(2))
        percentage = (correct / total * 100) if total > 0 else 0
        return {
            "correct": correct,
            "total": total,
            "percentage": round(percentage, 2),
            "raw_output": result_text.strip(),
        }
    else:
        return {
            "correct": None,
            "total": None,
            "percentage": None,
            "raw_output": result_text.strip(),
            "parse_error": "Could not parse score from output",
        }


def calculate_statistics(runs: list) -> dict:
    """Calculate summary statistics from multiple runs."""
    correct_scores = [r["correct"] for r in runs if r["correct"] is not None]
    percentages = [r["percentage"] for r in runs if r["percentage"] is not None]

    if not correct_scores:
        return {
            "mean_correct": None,
            "mean_percentage": None,
            "min_correct": None,
            "max_correct": None,
            "all_runs_parsed": False,
        }

    return {
        "mean_correct": round(sum(correct_scores) / len(correct_scores), 2),
        "mean_percentage": round(sum(percentages) / len(percentages), 2),
        "min_correct": min(correct_scores),
        "max_correct": max(correct_scores),
        "all_runs_parsed": len(correct_scores) == len(runs),
    }


def score_results_file(config: dict, results_file: str):
    """Score a results file and output structured JSON."""
    results_path = Path(results_file)

    if not results_path.exists():
        raise FileNotFoundError(f"Results file not found: {results_file}")

    # Determine scoring model name based on API
    scoring_model = config.get("model")
    scoring_api = config.get("api")
    repetitions = config.get("repetitions", 3)

    print(f"\n{'=' * 80}")
    print("SCORING RESULTS")
    print(f"{'=' * 80}")
    print(f"Results file: {results_file}")
    print(f"Scoring API: {scoring_api}")
    print(f"Scoring model: {scoring_model}")
    print(f"Repetitions: {repetitions}")
    print(f"{'=' * 80}")

    # Read results file
    with open(results_path, "r") as f:
        content = f.read()

    # Count questions in file for progress info
    question_count = content.count("---")
    print(f"\nResults file contains {question_count} questions\n")

    # Create OpenAI client
    client = create_client(config)

    # Run evaluations
    runs = []
    for n in range(repetitions):
        result_text = evaluate_results(config, client, content, n + 1, repetitions)
        parsed = parse_score(result_text)
        runs.append(parsed)

        # Show immediate result
        if parsed["correct"] is not None:
            print(
                f"✓ Parsed score: {parsed['correct']}/{parsed['total']} ({parsed['percentage']}%)"
            )
        else:
            print("⚠ Could not parse score from output")

    print(f"\n{'=' * 80}")
    print(f"Completed {repetitions} evaluation runs")
    print(f"{'=' * 80}")

    # Calculate statistics
    stats = calculate_statistics(runs)

    # Build output structure
    output = {
        "metadata": {
            "timestamp": datetime.now().isoformat(),
            "results_file": str(results_path),
            "scoring_api": scoring_api,
            "scoring_model": scoring_model,
            "repetitions": repetitions,
        },
        "runs": runs,
        "summary": stats,
    }

    # Generate output filename
    output_filename = results_path.stem + "_scores.json"
    output_path = results_path.parent / output_filename

    # Write JSON output
    with open(output_path, "w") as f:
        json.dump(output, f, indent=2)

    print(f"\n📊 Scores written to: {output_path}")

    # Display summary
    print(f"\n{'=' * 80}")
    print("SUMMARY")
    print(f"{'=' * 80}")
    if stats["mean_correct"] is not None:
        print(
            f"Mean Score:     {stats['mean_correct']}/{runs[0]['total']} ({stats['mean_percentage']}%)"
        )
        print(
            f"Score Range:    {stats['min_correct']}-{stats['max_correct']} correct answers"
        )
        print(f"Variance:       ±{stats['max_correct'] - stats['min_correct']} answers")
        print(f"All runs valid: {'Yes' if stats['all_runs_parsed'] else 'No'}")
    else:
        print("⚠ Unable to parse scores from LLM output")
    print(f"{'=' * 80}\n")

    return output_path


def main():
    """Main entry point."""
    if len(sys.argv) < 2:
        print("Usage: uv run score-results.py <results-file>")
        print(
            "\nExample: uv run score-results.py results/kubernetes-cka/qwen2.5-coder-7b-instruct-mlx@8bit_20260101235730.txt"
        )
        sys.exit(1)

    results_file = sys.argv[1]

    config = load_config()

    try:
        score_results_file(config, results_file)
    except KeyboardInterrupt:
        print("\n\nInterrupted by user. Exiting gracefully...", file=sys.stderr)
        sys.exit(130)  # Standard exit code for Ctrl+C
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()

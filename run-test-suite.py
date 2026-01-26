import json
from openai import OpenAI
import os
import csv
import yaml
import sys
from datetime import datetime
from pathlib import Path


def load_config(test_suite_path: str) -> dict:
    """Load configuration from YAML file."""
    config_file = Path(test_suite_path) / "config.yaml"

    if not config_file.exists():
        raise FileNotFoundError(
            f"Configuration file not found: {config_file}\n"
            f"Please create a config.yaml file in the test suite directory."
        )

    with open(config_file, "r") as f:
        config = yaml.safe_load(f)

    return config


def create_client(config: dict) -> OpenAI:
    """Create OpenAI client from configuration."""
    api_config = config.get("api", {})

    # Get API key from environment or use default
    api_key_env = api_config.get("api_key_env", "OPENAI_API_KEY")
    api_key_default = api_config.get("api_key_default", "not-needed")
    api_key = os.environ.get(api_key_env, api_key_default)

    # Get base URL
    base_url = api_config.get("base_url", "http://localhost:1234/v1")

    return OpenAI(api_key=api_key, base_url=base_url)


def prompt(
    client: OpenAI, model_config: dict, prompt_config: dict, question: str
) -> str:
    """Send a question to the LLM and get a response."""
    model_name = model_config.get("name", "gpt-3.5-turbo")
    system_message = prompt_config.get("system_message", "You are a helpful assistant.")
    temperature = model_config.get("temperature", 0.0)

    response = client.chat.completions.create(
        model=model_name,
        messages=[
            {"role": "system", "content": system_message},
            {"role": "user", "content": question},
        ],
        temperature=temperature,
    )

    return response.choices[0].message.content


def run_test_suite(test_suite: str):
    """Run a test suite and generate results for all configured models."""
    test_suite_path = Path("test_suites") / test_suite

    if not test_suite_path.exists():
        raise FileNotFoundError(
            f"Test suite not found: {test_suite_path}\n"
            f"Available test suites should be in the 'test_suites/' directory."
        )

    # Create results directory for this test suite run
    timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
    results_path = Path("results") / (test_suite + '_' + timestamp)
    results_path.mkdir(parents=True, exist_ok=True)

    # Load configuration
    print(f"Loading configuration for test suite: {test_suite}")
    config = load_config(test_suite_path)

    # Display test suite info
    print(f"Test Suite: {config.get('name', test_suite)}")
    print(f"Description: {config.get('description', 'N/A')}")
    print(f"API Base URL: {config['api']['base_url']}")

    # Create manifest for all test run related metadata
    test_run_metadata = {
        "test_suite_config": config,
        "models": [],
        "full_duration": None,
    }

    # Get models list
    models = config.get("models", [])
    if not models:
        raise ValueError(
            "No models configured in config.yaml. Please add at least one model to the 'models' list."
        )

    print(f"Models to test: {len(models)}")
    for i, model in enumerate(models, 1):
        print(f"  {i}. {model['name']}")
    print()

    # Create OpenAI client
    client = create_client(config)

    # Get questions file path
    questions_file = config.get("questions_file", "questions.csv")
    questions_path = test_suite_path / questions_file

    if not questions_path.exists():
        raise FileNotFoundError(f"Questions file not found: {questions_path}")

    # Get prompt configuration (shared across all models)
    prompt_config = config.get("prompt", {})

    # Track generated files
    generated_files = []

    start_time = datetime.now()

    # Process each model
    for model_idx, model_config in enumerate(models, 1):
        iteration_start_time = datetime.now()

        model_name = model_config.get("name", "unknown")

        print(f"\n{'=' * 80}")
        print(f"Model {model_idx}/{len(models)}: {model_name}")
        print(f"Temperature: {model_config.get('temperature', 0.7)}")
        print(f"{'=' * 80}\n")

        # Process questions
        print(f"Processing questions from: {questions_file}")
        output = ""
        question_count = 0

        with open(questions_path, newline="") as csvfile:
            reader = csv.DictReader(csvfile)

            for row in reader:
                qid = row["ID"]
                section = row["Section"]
                question = row["Question"]
                expected_answer = row["ExpectedAnswer"]

                print(f"Processing question {qid}...", end="\r")
                result = prompt(client, model_config, prompt_config, question)
                question_count += 1

                output += (
                    f"---\n"
                    f"NO. {qid} - {section}\n"
                    f"QUESTION: {question}\n"
                    f"EXPECTED ANSWER: {expected_answer}\n"
                    f"ACTUAL ANSWER: {result}\n"
                )

        print(f"\nProcessed {question_count} questions.")

        # Generate output filename
        filename_pattern = config.get("output", {}).get(
            "filename_pattern", "results_{model}.txt"
        )
        filename = filename_pattern.format(model=model_name, timestamp=timestamp)
        output_path = results_path / filename

        iteration_duration = datetime.now() - iteration_start_time

        # Write output
        with open(output_path, "w") as f:
            f.write(output)
        
        # Update manifest
        test_run_metadata["models"].append({
            "model_name": model_name,
            "duration": iteration_duration.total_seconds(),
            "results_file": str(output_path),
        })
    
    # Update manifest
    duration = datetime.now() - start_time
    test_run_metadata["full_duration"] = duration.total_seconds()

    # Write manifest
    with open(results_path / "resultset.json", "w") as f:
        json.dump(test_run_metadata, f, indent=4)

    print(f"Results written to: {output_path}")
    generated_files.append(output_path)

    # Summary
    print(f"\n{'=' * 80}")
    print("Test suite completed!")
    print(f"Tested {len(models)} model(s) with {question_count} questions each")
    print("\nGenerated results files:")
    for file_path in generated_files:
        print(f"  - {file_path}")
    print(f"{'=' * 80}")


def main():
    """Main entry point."""
    if len(sys.argv) < 2:
        print("Usage: uv run run-test-suite.py <test-suite-name>")
        print("\nExample: uv run run-test-suite.py kubernetes-cka")
        print("\nAvailable test suites:")
        test_suites_dir = Path("test_suites")
        if test_suites_dir.exists():
            for suite in sorted(test_suites_dir.iterdir()):
                if suite.is_dir():
                    print(f"  - {suite.name}")
        sys.exit(1)

    test_suite = sys.argv[1]

    try:
        run_test_suite(test_suite)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        raise e
        sys.exit(1)


if __name__ == "__main__":
    main()

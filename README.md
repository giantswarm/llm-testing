# LLM Performance Evaluation Framework

A framework for evaluating Large Language Models (LLMs), implemented in Python.

Some questions this framework helps answer:

- For a given set of questions, how well do certain models perform?
- How does performance for answering questions change if I modify my system prompt?
- How stable is the performance of a certain model when answering certain questions?

## How it works

1. You create a test suite, which is a set of questions, or use the provided one.
2. Run the test suite against your model(s) of choice, generating a set of answers/solutions.
3. Rate the answers, which involves a another LLM use.

For both testing and rating, you can choose to use a local LLM server (e.g., LM Studio) or a cloud-based API (e.g., Anthropic. OpenAI).

## Test Suites

So far, a single test suite is provided:

- **kubernetes-cka** - 100 questions covering Kubernetes Certified Administrator (CKA) exam topics

## Prerequisites

- Python 3.10+
- [uv](https://docs.astral.sh/uv/) - Fast Python package installer and resolver
- For local inference: a running local LLM server compatible with OpenAI API (e.g., [LM Studio](https://lmstudio.ai/)) and a model loaded.

## Installation

1. Install uv if you haven't already
2. Clone the repository and navigate to it:
3. Install dependencies using uv:

   ```bash
   uv sync --no-install-project
   ```

   This will create a virtual environment and install all dependencies automatically.

4. Configure environment variables. Set `OPENAI_API_KEY` or `ANTHROPIC_API_KEY` as needed. Not required when running locally.

## Usage

### Step 1: Configure Your Test Suite

Each test suite has its own `config.yaml` file that you also have to edit to define what model to test it against. Check `test_suites/kubernetes-cka-v2/config.yaml` for an example.

**Multiple Models**: The test suite will automatically run once for each model listed in the `models` array. This allows you to easily compare different models, quantization levels, or temperature settings in a single run.

**Model Configuration Options**:

- `name`: The model identifier (required)
- `temperature`: Controls randomness (0.0-2.0, default: 0.7)

Edit the `config.yaml` file in your test suite folder to customize these settings.

### Step 2: Run the test suite

Run the test suite runner with your chosen test suite:

```bash
uv run run-test-suite.py kubernetes-cka-v2
```

This will:

- Load configuration from `test_suites/kubernetes-cka-v2/config.yaml`
- Read questions from the configured questions file
- Create `results/kubernetes-cka-v2_<timestamp>/` directory if it doesn't exist
- **For each model** in the `models` list:
  - Send each question to your local LLM with the configured prompt
  - Generate a separate results file: `results/kubernetes-cka-v2_<timestamp>/<model>.txt`
- Display a summary of all generated results files

To see available test suites:

```bash
uv run run-test-suite.py
```

### Step 3: Score Results

Evaluate the generated results using either a local LLM or the Anthropic API:

Configure evaluation in `scoring_config.yaml`.

Then execute the scoring script with the path to your results file:

```bash
uv run score-results.py results/kubernetes-cka-v2_20260125/qwen2.5-coder-7b-instruct-mlx@8bit.txt
```

This will:

- Read the results file from the results folder
- Use an LLM to evaluate correctness (run 5 times for consistency)
- Generate a JSON file with structured scores: `results/kubernetes-cka-v2_20260125/qwen2.5-coder-7b-instruct-mlx@8bit_scores.json`
- Display a summary in the console

## Project Structure

```
.
├── test_suites/                         # Test suite configurations
│   └── kubernetes-cka-v2/               # Kubernetes CKA test suite
│       ├── config.yaml                  # Test suite configuration
│       └── questions.csv                # 100 CKA exam questions
├── results/                             # Generated results (gitignored)
│   └── kubernetes-cka-v2/               # Results for each test suite
│       ├── model_timestamp.txt          # Model output files
│       ├── model_timestamp_scores.json  # Evaluation scores (generated in scoring)
│       └── resultset.json               # Test run metadata
├── run-test-suite.py                    # Test suite runner script
└── score-results.py                     # Evaluates result correctness
```

## Managing Dependencies

To add a new dependency:
```bash
uv add package-name
```

To add a development dependency:
```bash
uv add --dev package-name
```

To update dependencies:
```bash
uv sync
```

## Example results

These are some results from evaluating different models on the `kubernetes-cka-v2` test suite:

| Model | Average Score | Score spread |
|-|-:|:-:|
| `claude-opus-4-5-20251101`| 95.0% | 94 - 97 |
| `claude-sonnet-4-5-20250929`| 91.0% | 90 - 92 |
| `ministral-3-8b-instruct-2512@4bit` | 55.3% | 51 - 58 |
| `ministral-3-8b-instruct-2512@5bit` | 63.0% | 58 - 68 |
| `ministral-3-8b-instruct-2512@6bit` | 65.7% | 58 - 70 |
| `qwen2.5-coder-7b-instruct-mlx@4bit` | 74.3% | 73 - 77 |
| `qwen2.5-coder-7b-instruct-mlx@8bit` | 71.7% | 71 - 73 |

All evaluations were performed using the `claude-sonnet-4-5-20250929` model for scoring.

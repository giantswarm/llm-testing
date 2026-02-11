# llm-testing

LLM evaluation testing framework with MCP server, KServe integration, and OAuth 2.1 authentication.

[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/llm-testing/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/llm-testing/tree/main)
[![Go Reference](https://pkg.go.dev/badge/github.com/giantswarm/llm-testing.svg)](https://pkg.go.dev/github.com/giantswarm/llm-testing)

## Overview

`llm-testing` evaluates LLM performance on domain-specific test suites. It:

- Manages model serving via **KServe** `InferenceService` CRDs (vLLM runtime)
- Runs Q&A evaluation test suites against model endpoints
- Scores results using **LLM-as-judge** with statistical confidence
- Exposes all functionality via an **MCP server** with OAuth 2.1

## Prerequisites

- **KServe** installed in the cluster with a vLLM `ServingRuntime` (managed via GitOps)
- Go 1.25+ (for building from source)
- Helm 3 (for Kubernetes deployment)

## Installation

### From release

Download the latest binary from [GitHub Releases](https://github.com/giantswarm/llm-testing/releases).

### From source

```bash
go install github.com/giantswarm/llm-testing@latest
```

### Helm chart

```bash
helm install llm-testing helm/llm-testing/ \
  --namespace llm-testing \
  --create-namespace
```

## Usage

### CLI Commands

**List available test suites:**

```bash
llm-testing list
```

**Run a test suite:**

```bash
llm-testing run kubernetes-cka-v2 \
  --model mistral-7b \
  --endpoint http://localhost:8000/v1
```

**Score results:**

```bash
llm-testing score results/Kubernetes_CKA_20260210-120000/mistral-7b.txt \
  --scoring-model claude-sonnet-4-5-20250929 \
  --scoring-endpoint https://api.anthropic.com/v1 \
  --api-key $ANTHROPIC_API_KEY \
  --repetitions 3
```

### MCP Server

**Start with stdio transport (for IDE integration):**

```bash
llm-testing serve --transport stdio
```

**Start with HTTP transport:**

```bash
llm-testing serve \
  --transport streamable-http \
  --http-addr :8080 \
  --in-cluster
```

**With OAuth enabled:**

```bash
llm-testing serve \
  --transport streamable-http \
  --enable-oauth \
  --oauth-base-url https://llm-testing.example.com \
  --dex-issuer-url https://dex.example.com \
  --dex-client-id llm-testing \
  --dex-client-secret $DEX_CLIENT_SECRET
```

### MCP Tools

| Tool | Description |
|------|-------------|
| `list_test_suites` | List available test suites with metadata |
| `run_test_suite` | Execute a test suite against models |
| `score_results` | Score results using LLM-as-judge |
| `get_results` | Retrieve past results and scores |
| `deploy_model` | Create a KServe InferenceService |
| `teardown_model` | Delete a KServe InferenceService |
| `list_models` | List managed InferenceService resources |

## Architecture

```
llm-testing/
├── cmd/                  # Cobra CLI commands
├── internal/
│   ├── kserve/           # KServe InferenceService lifecycle
│   ├── llm/              # OpenAI-compatible client abstraction
│   ├── mcp/              # MCP tool definitions and handlers
│   ├── runner/           # Test execution engine (strategy pattern)
│   ├── scorer/           # LLM-as-judge scoring engine
│   ├── server/           # Server context and configuration
│   └── testsuite/        # Test suite types, loader, embedded suites
│       └── testdata/     # Bundled test suite definitions (embedded via go:embed)
└── helm/llm-testing/     # Helm chart
```

## Test Suites

Test suites are defined as a directory containing:
- `config.yaml` -- suite metadata, models, prompt configuration
- `questions.csv` -- questions with ID, Section, Question, ExpectedAnswer

Default test suites are embedded in the binary from `internal/testsuite/testdata/`. Additional suites can be loaded from an external directory via `--suites-dir`.

### Bundled Suites

- **kubernetes-cka-v2** -- 100 Kubernetes CKA exam questions

## Development

```bash
make build      # Build binary
make test       # Run tests
make lint       # Run linter
make helm-lint  # Lint Helm chart
```

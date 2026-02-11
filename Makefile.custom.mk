##@ Custom

.PHONY: build
build: ## Build the binary.
	go build -trimpath -ldflags "-s -w -X main.version=dev -X main.commit=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.date=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o llm-testing .

.PHONY: test
test: ## Run tests with race detection.
	go test -race ./...

.PHONY: lint
lint: ## Run golangci-lint.
	golangci-lint run --timeout 15m

.PHONY: fmt
fmt: ## Format Go code.
	goimports -local github.com/giantswarm/llm-testing -w .
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: clean
clean: ## Remove build artifacts.
	rm -f llm-testing

.PHONY: helm-lint
helm-lint: ## Lint the Helm chart.
	helm lint helm/llm-testing/

.PHONY: helm-template
helm-template: ## Template the Helm chart for validation.
	helm template llm-testing helm/llm-testing/

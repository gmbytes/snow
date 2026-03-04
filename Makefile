PKG=./...
BIN_DIR=bin
GO=go
LINTER=golangci-lint

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make tools              Install required tools"
	@echo "  make lint              Run golangci-lint"
	@echo "  make test              Run tests"
	@echo "  make test-race         Run tests with -race"
	@echo "  make bench             Run benchmarks"
	@echo "  make coverage          Run tests and generate coverage.out"
	@echo "  make cover-html       Generate HTML coverage report"
	@echo "  make vet              Run go vet"
	@echo "  make fmt              Run go fmt"
	@echo "  make tidy             Run go mod tidy"
	@echo "  make build            Build all packages"
	@echo "  make build-examples   Build all examples into bin/"
	@echo "  make run-example     Run one example (EX=example_path)"
	@echo "  make ci               Run lint, tests and race detector"
	@echo "  make init-hooks       Install git pre-commit hook"
	@echo "  make init-workspace   Generate go.work at slgame/ root (IDE cross-module support)"
	@echo "  make clean            Clean build artifacts"

.PHONY: tools
tools:
	@command -v $(LINTER) >/dev/null 2>&1 || $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.8.0

.PHONY: lint
lint:
	$(LINTER) run -c .golangci.yml $(PKG)

.PHONY: test
test:
	$(GO) test -count=1 -shuffle=on $(PKG)

.PHONY: test-race
test-race:
	$(GO) test -race -count=1 -shuffle=on -timeout 3m $(PKG)

.PHONY: bench
bench:
	$(GO) test -run=^$$ -bench=. $(PKG)

.PHONY: coverage
coverage:
	$(GO) test -covermode=atomic -coverprofile=coverage.out -coverpkg=./... $(PKG)
	@echo ""
	@echo "=== uncovered functions (< 100%) ==="
	@$(GO) tool cover -func=coverage.out | grep -v '100.0%' | grep -v '^total:' | sort -t$$'\t' -k3 -n
	@echo ""
	@echo "=== coverage summary ==="
	@$(GO) tool cover -func=coverage.out | grep -E '^total:'

.PHONY: cover-html
cover-html: coverage
	$(GO) tool cover -html=coverage.out -o coverage.html

.PHONY: vet
vet:
	$(GO) vet $(PKG)

.PHONY: fmt
fmt:
	$(GO) fmt $(PKG)

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: build
build:
	$(GO) build $(PKG)

.PHONY: build-examples
build-examples:
	@mkdir -p $(BIN_DIR)
	@for d in examples/*; do if [ -d $$d ]; then n=$$(basename $$d); $(GO) build -o $(BIN_DIR)/$$n ./$$d; fi; done

.PHONY: run-example
run-example:
	@if [ -z "$(EX)" ]; then echo "Usage: make run-example EX=examples/pingpong"; exit 1; fi
	$(GO) run ./$(EX)

.PHONY: ci
ci: lint test test-race

.PHONY: init-hooks
init-hooks:
	git config core.hooksPath githooks
	@echo "Git hooks installed (githooks/)"

.PHONY: init-workspace
init-workspace:
	@cd .. && go work init ./snow ./snow/examples/minimal ./snow/examples/pingpong ./snow/examples/discovery ./server 2>/dev/null; \
	echo "go.work created at $$(cd .. && pwd)"

.PHONY: clean
clean:
	@rm -rf $(BIN_DIR) coverage.out coverage.html
.PHONY: default help build build-release test test-race coverage go-lint go-vet govulncheck security go-update go-clean install

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS_VERSION = -X github.com/julienhmmt/helmdownloader/pkg/version.Version=$(VERSION)

default: help

help:
	@echo "Available targets:"
	@echo "  build          Build the helmdownloader binary"
	@echo "  build-release  Build optimized release binary"
	@echo "  test           Run all tests"
	@echo "  test-race      Run tests with race detector"
	@echo "  coverage       Run tests with coverage profile (coverage.out)"
	@echo "  go-lint        Run golangci-lint on the codebase"
	@echo "  go-vet         Run go vet"
	@echo "  govulncheck    Scan for known vulnerabilities (CVEs)"
	@echo "  security       Run vet + lint + govulncheck"
	@echo "  go-update      Update Go module dependencies"
	@echo "  go-clean       Remove build artifacts and caches"
	@echo "  install        Install the binary to \$$GOPATH/bin"

build:
	go build -ldflags "$(LDFLAGS_VERSION)" -o helmdownloader .

build-release:
	go build -ldflags="-s -w $(LDFLAGS_VERSION)" -trimpath -o helmdownloader .

test:
	go test ./... -count=1

test-race:
	go test ./... -race -count=1

coverage:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -n 1

go-lint:
	golangci-lint run ./...

go-vet:
	go vet ./...

govulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

security: go-vet go-lint govulncheck

go-update:
	go get -u ./...
	go mod tidy
	go mod verify

go-clean:
	rm -f helmdownloader
	go clean -cache

install:
	go install .

# ---------------------------------------------------------------------------
# SonarQube (shared local server, see $(SONAR_HOME)/README.md)
# ---------------------------------------------------------------------------
SONAR_HOME ?= $(HOME)/git/gh/sonarqube

.PHONY: sonar sonar-up sonar-down sonar-coverage sonar-scan sonar-query

sonar: sonar-scan ## Generate coverage and scan into the shared SonarQube

sonar-up: ## Start the shared SonarQube server (http://localhost:9000)
	@$(MAKE) -C $(SONAR_HOME) up

sonar-down: ## Stop the shared SonarQube server
	@$(MAKE) -C $(SONAR_HOME) down

sonar-coverage: ## Generate Go coverage reports for SonarQube
	@chmod +x tools/sonar-coverage.sh
	@GO_TEST_FLAGS="$(GO_TEST_FLAGS)" SERVER_DIR="$(SERVER_DIR)" tools/sonar-coverage.sh
	@echo "Coverage reports ready in .sonar/"

sonar-scan: sonar-coverage ## Scan into the shared SonarQube (starts it if needed)
	@$(SONAR_HOME)/scan.sh

sonar-query: ## Query SonarQube results (usage: make sonar-query CMD="summary")
	@python3 $(SONAR_HOME)/query.py $(CMD)

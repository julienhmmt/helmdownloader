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
# SonarQube (local container, single Go project: helmdownloader)
# ---------------------------------------------------------------------------
COMPOSE_SONAR := docker compose -f docker-compose.sonarqube.yml

.PHONY: sonar-up sonar-down sonar-logs sonar-status sonar-bootstrap sonar-coverage sonar-scan sonar-scan-server sonar-query sonar-clean

sonar-up: ## Start the SonarQube server (http://localhost:9000)
	$(COMPOSE_SONAR) up -d sonarqube

sonar-down: ## Stop the SonarQube server
	$(COMPOSE_SONAR) down

sonar-logs: ## Follow SonarQube logs
	$(COMPOSE_SONAR) logs -f sonarqube

sonar-status: ## Check SonarQube server status
	@docker ps --filter "name=helmdownloader-sonarqube" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

sonar-bootstrap: ## Provision or rotate the SonarQube analysis token
	@chmod +x tools/sonar-bootstrap.sh
	@tools/sonar-bootstrap.sh
	@echo "SonarQube token ready in .sonar/token"

sonar-coverage: ## Generate Go coverage reports for SonarQube
	@chmod +x tools/sonar-coverage.sh
	@GO_TEST_FLAGS="$(GO_TEST_FLAGS)" SERVER_DIR="$(SERVER_DIR)" tools/sonar-coverage.sh
	@echo "Coverage reports ready in .sonar/"

sonar-scan: sonar-coverage ## Run SonarScanner for the Go project (requires sonar-up + sonar-bootstrap)
	@chmod +x tools/sonar-scan.sh
	@tools/sonar-scan.sh

sonar-scan-server: ## Scan only the Go project (server short name)
	@chmod +x tools/sonar-scan.sh
	@tools/sonar-scan.sh server

sonar-query: ## Query SonarQube results (usage: make sonar-query CMD="summary")
	@python3 tools/sonar-query.py $(CMD)

sonar-clean: sonar-down ## Stop SonarQube and remove its local data and token
	rm -rf .sonar

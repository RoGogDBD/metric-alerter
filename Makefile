SERVER_DIR=cmd/server
AGENT_DIR=cmd/agent
RESET_DIR=cmd/reset

PROFILES_DIR=profiles
COVERAGE_SERVER=$(PROFILES_DIR)/coverage-server.out
COVERAGE_AGENT=$(PROFILES_DIR)/coverage-agent.out

# Инфа по сборке
VERSION?=dev
BUILD_DATE=$(shell date -u '+%Y-%m-%d_%H:%M:%S') # UTC
BUILD_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown") # Короткий хеш коммита
LDFLAGS=-ldflags "-X main.buildVersion=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.buildCommit=$(BUILD_COMMIT)" # Флаги для передачи информации о сборке

.PHONY: all test build clean test-server test-agent cover generate build-with-version

all: test build

test: test-server test-agent

test-server:
	@echo "--- Running tests in $(SERVER_DIR) ---"
	@mkdir -p $(PROFILES_DIR)
	@go test -v -coverprofile=$(COVERAGE_SERVER) ./$(SERVER_DIR)
	@go tool cover -html=$(COVERAGE_SERVER) -o $(PROFILES_DIR)/server_coverage.html
	@if [ "${OPEN_BROWSER:-0}" = "1" ]; then xdg-open $(PROFILES_DIR)/server_coverage.html || true; fi
	@echo "--- Completed ---"

test-agent:
	@echo "--- Running tests in $(AGENT_DIR) ---"
	@mkdir -p $(PROFILES_DIR)
	@go test -v -coverprofile=$(COVERAGE_AGENT) -covermode=atomic ./$(AGENT_DIR)
	@go tool cover -html=$(COVERAGE_AGENT) -o $(PROFILES_DIR)/agent_coverage.html
	@if [ "${OPEN_BROWSER:-0}" = "1" ]; then xdg-open $(PROFILES_DIR)/agent_coverage.html || true; fi
	@echo "--- Completed ---"

build:
	@echo "--- Building the server and agent ---"
	@mkdir -p bin/server
	@mkdir -p bin/agent
	@go build -o bin/server/server ./$(SERVER_DIR)
	@go build -o bin/agent/agent ./$(AGENT_DIR)
	@echo "--- Completed ---"

clean:
	@echo "--- Cleaning build artifacts ---"
	@rm -f $(PROFILES_DIR)/coverage*.out $(PROFILES_DIR)/*.html
	@rm -rf bin
	@echo "--- Completed ---"

generate:
	@echo "--- Generating Reset() methods ---"
	@go run ./$(RESET_DIR)/main.go
	@echo "--- Completed ---"

build-with-version:
	@echo "--- Building with version info ---"
	@echo "Version: $(VERSION)"
	@echo "Build date: $(BUILD_DATE)"
	@echo "Build commit: $(BUILD_COMMIT)"
	@mkdir -p bin/server
	@mkdir -p bin/agent
	@go build $(LDFLAGS) -o bin/server/server ./$(SERVER_DIR)
	@go build $(LDFLAGS) -o bin/agent/agent ./$(AGENT_DIR)
	@echo "--- Completed ---"

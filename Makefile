SERVER_DIR=cmd/server
AGENT_DIR=cmd/agent

COVERAGE_SERVER=coverage-server.out
COVERAGE_AGENT=coverage-agent.out

.PHONY: all test build clean test-server test-agent cover

all: test build

test: test-server test-agent

test-server:
	@echo "--- Running tests in $(SERVER_DIR) ---"
	@go test -v -coverprofile=$(COVERAGE_SERVER) ./$(SERVER_DIR)
	@go tool cover -html=$(COVERAGE_SERVER) -o server_coverage.html
	@xdg-open server_coverage.html
	@echo "--- Completed ---"

test-agent:
	@echo "--- Running tests in $(AGENT_DIR) ---"
	@go test -v -coverprofile=$(COVERAGE_AGENT) -covermode=atomic ./$(AGENT_DIR)
	@go tool cover -html=$(COVERAGE_AGENT) -o agent_coverage.html
	@xdg-open agent_coverage.html
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
	@rm -f coverage*.out *.html
	@rm -rf bin
	@echo "--- Completed ---"

.PHONY: all test clean build
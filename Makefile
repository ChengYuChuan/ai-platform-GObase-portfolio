# AI Platform Portfolio - Root Makefile
# Unified command interface for all services

.PHONY: all help up down logs test-all build-all clean

# Default target
all: help

## ============================================================================
## HELP
## ============================================================================

## help: Show this help message
help:
	@echo "╔══════════════════════════════════════════════════════════════════╗"
	@echo "║           AI Platform Portfolio - Command Reference              ║"
	@echo "╚══════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "─────────────────────────────────────────────────────────────────────"
	@echo "  DOCKER COMPOSE (Full Stack)"
	@echo "─────────────────────────────────────────────────────────────────────"
	@echo "  up              Start all services"
	@echo "  up-monitoring   Start with Prometheus/Grafana/Jaeger"
	@echo "  down            Stop all services"
	@echo "  logs            View all logs (follow mode)"
	@echo "  ps              Show running services"
	@echo "  restart         Restart all services"
	@echo ""
	@echo "─────────────────────────────────────────────────────────────────────"
	@echo "  INDIVIDUAL SERVICES"
	@echo "─────────────────────────────────────────────────────────────────────"
	@echo "  gateway         Run Go gateway locally"
	@echo "  rag             Run Python RAG service locally"
	@echo "  dashboard       Run TypeScript dashboard locally"
	@echo ""
	@echo "─────────────────────────────────────────────────────────────────────"
	@echo "  BUILD & TEST"
	@echo "─────────────────────────────────────────────────────────────────────"
	@echo "  build-all       Build all Docker images"
	@echo "  test-all        Run tests for all projects"
	@echo "  lint-all        Run linters for all projects"
	@echo "  clean           Clean all build artifacts"
	@echo ""
	@echo "─────────────────────────────────────────────────────────────────────"
	@echo "  BENCHMARKS"
	@echo "─────────────────────────────────────────────────────────────────────"
	@echo "  benchmark       Run performance benchmarks"
	@echo "  benchmark-report Generate benchmark report"
	@echo ""

## ============================================================================
## DOCKER COMPOSE
## ============================================================================

## up: Start all services
up:
	@echo "🚀 Starting all services..."
	docker compose up -d
	@echo ""
	@echo "✅ Services started!"
	@echo ""
	@echo "Access points:"
	@echo "  • Dashboard:    http://localhost:3000"
	@echo "  • Gateway API:  http://localhost:8080"
	@echo "  • RAG Service:  http://localhost:8000"
	@echo ""
	@echo "Run 'make logs' to view logs"

## up-monitoring: Start with monitoring stack (Prometheus, Grafana, Jaeger)
up-monitoring:
	@echo "🚀 Starting all services with monitoring..."
	docker compose --profile monitoring up -d
	@echo ""
	@echo "✅ Services started with monitoring!"
	@echo ""
	@echo "Access points:"
	@echo "  • Dashboard:    http://localhost:3000"
	@echo "  • Gateway API:  http://localhost:8080"
	@echo "  • RAG Service:  http://localhost:8000"
	@echo "  • Prometheus:   http://localhost:9090"
	@echo "  • Grafana:      http://localhost:3001 (admin/admin)"
	@echo "  • Jaeger:       http://localhost:16686"

## down: Stop all services
down:
	@echo "🛑 Stopping all services..."
	docker compose --profile monitoring down
	@echo "✅ All services stopped"

## logs: View all logs
logs:
	docker compose logs -f

## logs-gateway: View gateway logs only
logs-gateway:
	docker compose logs -f gateway

## logs-rag: View RAG service logs only
logs-rag:
	docker compose logs -f rag-service

## ps: Show running services
ps:
	docker compose ps

## restart: Restart all services
restart: down up

## ============================================================================
## INDIVIDUAL SERVICES (Local Development)
## ============================================================================

## gateway: Run Go gateway locally
gateway:
	@echo "🚀 Starting Go Gateway..."
	cd llm-gateway && make run

## rag: Run Python RAG service locally
rag:
	@echo "🚀 Starting Python RAG Service..."
	cd rag-agent && uv run uvicorn src.main:app --reload --port 8000

## dashboard: Run TypeScript dashboard locally
dashboard:
	@echo "🚀 Starting TypeScript Dashboard..."
	cd agent-dashboard && pnpm dev

## ============================================================================
## BUILD
## ============================================================================

## build-all: Build all Docker images
build-all:
	@echo "🔨 Building all Docker images..."
	docker compose build
	@echo "✅ All images built"

## build-gateway: Build gateway image only
build-gateway:
	@echo "🔨 Building Gateway image..."
	docker compose build gateway

## build-rag: Build RAG service image only
build-rag:
	@echo "🔨 Building RAG Service image..."
	docker compose build rag-service

## build-dashboard: Build dashboard image only
build-dashboard:
	@echo "🔨 Building Dashboard image..."
	docker compose build dashboard

## ============================================================================
## TEST
## ============================================================================

## test-all: Run tests for all projects
test-all: test-gateway test-rag test-dashboard
	@echo ""
	@echo "✅ All tests completed!"

## test-gateway: Run Go gateway tests
test-gateway:
	@echo "🧪 Testing Go Gateway..."
	cd llm-gateway && make test

## test-rag: Run Python RAG tests
test-rag:
	@echo "🧪 Testing Python RAG Service..."
	cd rag-agent && uv run pytest

## test-dashboard: Run TypeScript dashboard tests
test-dashboard:
	@echo "🧪 Testing TypeScript Dashboard..."
	cd agent-dashboard && pnpm test

## ============================================================================
## LINT
## ============================================================================

## lint-all: Run linters for all projects
lint-all: lint-gateway lint-rag lint-dashboard
	@echo ""
	@echo "✅ All linting completed!"

## lint-gateway: Lint Go code
lint-gateway:
	@echo "🔍 Linting Go Gateway..."
	cd llm-gateway && make lint

## lint-rag: Lint Python code
lint-rag:
	@echo "🔍 Linting Python RAG Service..."
	cd rag-agent && uv run ruff check .

## lint-dashboard: Lint TypeScript code
lint-dashboard:
	@echo "🔍 Linting TypeScript Dashboard..."
	cd agent-dashboard && pnpm lint

## ============================================================================
## BENCHMARKS
## ============================================================================

## benchmark: Run performance benchmarks
benchmark:
	@echo "📊 Running benchmarks..."
	@echo "Make sure the gateway is running first: make up"
	@echo ""
	@if command -v hey >/dev/null 2>&1; then \
		echo "Running throughput test (10s, 100 connections)..."; \
		hey -z 10s -c 100 http://localhost:8080/health; \
	else \
		echo "❌ 'hey' not installed. Install with: go install github.com/rakyll/hey@latest"; \
	fi

## benchmark-report: Generate benchmark report
benchmark-report:
	@echo "📊 Generating benchmark report..."
	@echo "TODO: Implement benchmark automation"

## ============================================================================
## CLEAN
## ============================================================================

## clean: Clean all build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	cd llm-gateway && make clean 2>/dev/null || true
	cd rag-agent && rm -rf .venv __pycache__ .pytest_cache .ruff_cache 2>/dev/null || true
	cd agent-dashboard && rm -rf node_modules .next 2>/dev/null || true
	docker compose down -v --remove-orphans 2>/dev/null || true
	@echo "✅ Clean complete"

## ============================================================================
## SETUP (First-time setup)
## ============================================================================

## setup: First-time project setup
setup:
	@echo "🔧 Setting up development environment..."
	@echo ""
	@echo "Checking prerequisites..."
	@command -v docker >/dev/null 2>&1 || (echo "❌ Docker not found" && exit 1)
	@command -v go >/dev/null 2>&1 || echo "⚠️  Go not found (needed for local gateway development)"
	@command -v uv >/dev/null 2>&1 || echo "⚠️  uv not found (needed for local RAG development)"
	@command -v pnpm >/dev/null 2>&1 || echo "⚠️  pnpm not found (needed for local dashboard development)"
	@echo ""
	@echo "Creating .env file..."
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "✅ Created .env file - please add your API keys"; \
	else \
		echo "ℹ️  .env file already exists"; \
	fi
	@echo ""
	@echo "Setup complete! Next steps:"
	@echo "  1. Add your API keys to .env"
	@echo "  2. Run 'make up' to start all services"

## install-tools: Install development tools
install-tools:
	@echo "🔧 Installing development tools..."
	@echo ""
	@echo "Go tools:"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/rakyll/hey@latest
	go install github.com/cosmtrek/air@latest
	@echo ""
	@echo "Python tools (via uv):"
	@command -v uv >/dev/null 2>&1 || curl -LsSf https://astral.sh/uv/install.sh | sh
	@echo ""
	@echo "Node.js tools:"
	@command -v pnpm >/dev/null 2>&1 || npm install -g pnpm
	@echo ""
	@echo "✅ Tools installed"

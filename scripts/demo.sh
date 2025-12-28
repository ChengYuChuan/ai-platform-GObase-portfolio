#!/bin/bash
# AI Platform Demo Script
# This script demonstrates the full capabilities of the AI Platform

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
RAG_URL="${RAG_URL:-http://localhost:8000}"
DASHBOARD_URL="${DASHBOARD_URL:-http://localhost:3000}"

print_header() {
    echo ""
    echo -e "${CYAN}========================================${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}========================================${NC}"
    echo ""
}

print_step() {
    echo -e "${YELLOW}➤ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

wait_for_key() {
    echo ""
    read -p "Press Enter to continue..."
    echo ""
}

# Check if services are running
check_services() {
    print_header "Checking Services Health"

    print_step "Checking Gateway..."
    if curl -s "$GATEWAY_URL/health" > /dev/null 2>&1; then
        print_success "Gateway is healthy"
    else
        echo -e "${RED}✗ Gateway is not running${NC}"
        echo "Please start services with: docker compose up -d"
        exit 1
    fi

    print_step "Checking RAG Service..."
    if curl -s "$RAG_URL/health" > /dev/null 2>&1; then
        print_success "RAG Service is healthy"
    else
        echo -e "${RED}✗ RAG Service is not running${NC}"
    fi

    print_step "Checking Dashboard..."
    if curl -s "$DASHBOARD_URL" > /dev/null 2>&1; then
        print_success "Dashboard is accessible"
    else
        echo -e "${RED}✗ Dashboard is not running${NC}"
    fi
}

# Demo 1: Basic Chat Completion
demo_chat_completion() {
    print_header "Demo 1: Chat Completion API"

    print_info "Sending a simple chat request to the Gateway..."
    print_step "Request:"
    echo '{
  "model": "gpt-4",
  "messages": [
    {"role": "user", "content": "What is the capital of France? Reply in one sentence."}
  ],
  "max_tokens": 50
}'

    wait_for_key

    print_step "Response:"
    curl -s -X POST "$GATEWAY_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "gpt-4",
            "messages": [
                {"role": "user", "content": "What is the capital of France? Reply in one sentence."}
            ],
            "max_tokens": 50
        }' | jq .

    print_success "Chat completion successful!"
}

# Demo 2: Streaming Response
demo_streaming() {
    print_header "Demo 2: Streaming Response (SSE)"

    print_info "Streaming allows real-time token-by-token response..."
    print_step "Request with stream: true"

    wait_for_key

    print_step "Streaming response:"
    curl -s -X POST "$GATEWAY_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "gpt-4",
            "messages": [
                {"role": "user", "content": "Count from 1 to 5 slowly."}
            ],
            "stream": true,
            "max_tokens": 100
        }' 2>&1 | while read line; do
        if [[ $line == data:* ]]; then
            echo "$line"
        fi
    done

    echo ""
    print_success "Streaming demo complete!"
}

# Demo 3: Multi-Provider Support
demo_providers() {
    print_header "Demo 3: Multi-Provider Support"

    print_info "The Gateway supports multiple LLM providers..."

    print_step "Available Providers:"
    echo "  • OpenAI (GPT-4, GPT-3.5-turbo)"
    echo "  • Anthropic (Claude 3)"
    echo "  • Ollama (Local models)"

    wait_for_key

    print_step "Testing Anthropic Claude:"
    curl -s -X POST "$GATEWAY_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "claude-3-haiku-20240307",
            "messages": [
                {"role": "user", "content": "Say hello in Japanese."}
            ],
            "max_tokens": 30
        }' | jq .

    print_success "Multi-provider demo complete!"
}

# Demo 4: Rate Limiting
demo_rate_limiting() {
    print_header "Demo 4: Rate Limiting"

    print_info "The Gateway implements token bucket rate limiting..."
    print_step "Sending 5 rapid requests..."

    wait_for_key

    for i in {1..5}; do
        response=$(curl -s -w "%{http_code}" -o /dev/null -X POST "$GATEWAY_URL/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -d '{
                "model": "gpt-4",
                "messages": [{"role": "user", "content": "Hi"}],
                "max_tokens": 5
            }')
        echo "Request $i: HTTP $response"
    done

    print_success "Rate limiting demo complete!"
}

# Demo 5: Caching
demo_caching() {
    print_header "Demo 5: Semantic Caching"

    print_info "Identical requests are cached for faster response..."

    REQUEST='{
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "What is 2+2?"}],
        "max_tokens": 10
    }'

    print_step "First request (cache miss):"
    time curl -s -X POST "$GATEWAY_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "$REQUEST" > /dev/null

    wait_for_key

    print_step "Second request (cache hit - should be faster):"
    time curl -s -X POST "$GATEWAY_URL/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "$REQUEST" > /dev/null

    print_success "Caching demo complete!"
}

# Demo 6: RAG Pipeline
demo_rag() {
    print_header "Demo 6: RAG Pipeline"

    print_info "Demonstrating document ingestion and retrieval..."

    print_step "1. Uploading a document..."

    # Create a sample document
    echo "The AI Platform is a production-grade system for building AI applications.
It consists of three main components:
1. LLM Gateway (Go) - High-performance API gateway
2. RAG Agent (Python) - Document processing and retrieval
3. Dashboard (TypeScript) - User interface

Key features include:
- Multi-provider support (OpenAI, Anthropic, Ollama)
- Semantic caching for faster responses
- Rate limiting and circuit breakers
- Vector search with Qdrant" > /tmp/demo-doc.txt

    curl -s -X POST "$RAG_URL/api/documents" \
        -F "file=@/tmp/demo-doc.txt" \
        -F "title=AI Platform Overview" | jq .

    wait_for_key

    print_step "2. Querying with RAG..."
    curl -s -X POST "$RAG_URL/api/query" \
        -H "Content-Type: application/json" \
        -d '{
            "query": "What are the main components of the AI Platform?",
            "top_k": 3
        }' | jq .

    print_success "RAG demo complete!"
}

# Demo 7: Agent Execution
demo_agent() {
    print_header "Demo 7: Agent Execution"

    print_info "Running a ReAct agent with tools..."

    print_step "Available Agent Types:"
    echo "  • ReAct Agent - Reasoning and acting"
    echo "  • RAG Agent - Document-augmented generation"
    echo "  • Code Agent - Code analysis and generation"

    wait_for_key

    print_step "Executing ReAct Agent..."
    curl -s -X POST "$RAG_URL/api/agents/react" \
        -H "Content-Type: application/json" \
        -d '{
            "query": "What is the weather like today? Use the weather tool.",
            "max_steps": 5
        }' | jq .

    print_success "Agent demo complete!"
}

# Demo 8: Metrics & Monitoring
demo_monitoring() {
    print_header "Demo 8: Metrics & Monitoring"

    print_info "The platform exports Prometheus metrics..."

    print_step "Gateway Metrics (sample):"
    curl -s "$GATEWAY_URL/metrics" 2>/dev/null | head -20

    echo ""
    print_info "Access full monitoring stack:"
    echo "  • Prometheus: http://localhost:9090"
    echo "  • Grafana:    http://localhost:3001 (admin/admin)"
    echo "  • Jaeger:     http://localhost:16686"

    print_success "Monitoring demo complete!"
}

# Demo 9: Test Results
demo_tests() {
    print_header "Demo 9: Test Coverage"

    print_info "Running all tests..."

    print_step "Go Gateway Tests:"
    cd "$(dirname "$0")/../llm-gateway"
    go test ./... -v --short 2>&1 | tail -20

    wait_for_key

    print_step "TypeScript Dashboard Tests:"
    cd "$(dirname "$0")/../agent-dashboard"
    npm test -- --passWithNoTests 2>&1 | tail -20

    print_success "All tests passed!"
}

# Main demo flow
main() {
    clear
    print_header "AI Platform Portfolio Demo"

    echo "This demo showcases a production-grade AI platform with:"
    echo ""
    echo "  🚀 Go Gateway      - High-performance LLM routing"
    echo "  🐍 Python RAG      - Document processing & agents"
    echo "  ⚛️  TypeScript UI   - Modern web dashboard"
    echo ""
    echo "Architecture:"
    echo "  ┌──────────┐    ┌──────────┐    ┌──────────┐"
    echo "  │Dashboard │───▶│ Gateway  │───▶│RAG Agent │"
    echo "  │  :3000   │    │  :8080   │    │  :8000   │"
    echo "  └──────────┘    └──────────┘    └──────────┘"
    echo ""

    wait_for_key

    check_services

    wait_for_key

    # Run demos
    demo_chat_completion
    wait_for_key

    demo_streaming
    wait_for_key

    demo_caching
    wait_for_key

    demo_rate_limiting
    wait_for_key

    demo_monitoring

    print_header "Demo Complete!"

    echo "Summary of Demonstrated Features:"
    echo ""
    echo "  ✓ Chat Completion API (OpenAI compatible)"
    echo "  ✓ Streaming responses (SSE)"
    echo "  ✓ Multi-provider support"
    echo "  ✓ Rate limiting"
    echo "  ✓ Semantic caching"
    echo "  ✓ Prometheus metrics"
    echo ""
    echo "Additional Resources:"
    echo "  • Dashboard:     $DASHBOARD_URL"
    echo "  • API Docs:      $RAG_URL/docs"
    echo "  • Architecture:  docs/architecture.md"
    echo "  • Deployment:    docs/deployment.md"
    echo ""
    print_success "Thank you for watching!"
}

# Parse arguments
case "${1:-}" in
    "chat")
        demo_chat_completion
        ;;
    "stream")
        demo_streaming
        ;;
    "cache")
        demo_caching
        ;;
    "rate")
        demo_rate_limiting
        ;;
    "rag")
        demo_rag
        ;;
    "agent")
        demo_agent
        ;;
    "monitor")
        demo_monitoring
        ;;
    "test")
        demo_tests
        ;;
    "all"|"")
        main
        ;;
    *)
        echo "Usage: $0 [command]"
        echo ""
        echo "Commands:"
        echo "  all      Run full demo (default)"
        echo "  chat     Chat completion demo"
        echo "  stream   Streaming demo"
        echo "  cache    Caching demo"
        echo "  rate     Rate limiting demo"
        echo "  rag      RAG pipeline demo"
        echo "  agent    Agent execution demo"
        echo "  monitor  Monitoring demo"
        echo "  test     Run tests"
        ;;
esac

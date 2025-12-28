# AI Platform Portfolio

<div align="center">

**A production-grade AI platform demonstrating full-stack engineering across Go, Python, and TypeScript**

[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Python](https://img.shields.io/badge/Python-3.12-3776AB?style=flat&logo=python)](https://python.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0-3178C6?style=flat&logo=typescript)](https://typescriptlang.org/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

[Architecture](#architecture) • [Quick Start](#quick-start) • [Projects](#projects) • [Kubernetes](#kubernetes-deployment) • [Benchmarks](#performance-benchmarks) • [Security](./SECURITY.md)

</div>

---

## Why This Portfolio?

This portfolio demonstrates a key insight in AI systems engineering:

> **Language selection should be driven by technical requirements, not ecosystem assumptions.**

While Python dominates AI/ML, production AI platforms require **hybrid architectures** that leverage each language's strengths:

| Component | Language | Why |
|-----------|----------|-----|
| **API Gateway** | Go | 10,000+ concurrent connections, <50ms p99 latency |
| **RAG Pipeline** | Python | LangChain, vector DBs, ML ecosystem |
| **Dashboard** | TypeScript | React, real-time UI, type safety |

### Real-World Validation

Companies like Dropbox, Uber, and Cloudflare have demonstrated 50-80% cost savings by using Go for high-throughput network services while maintaining Python for ML workloads.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              AI Platform                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│    ┌──────────────┐         ┌──────────────┐         ┌──────────────┐       │
│    │   Frontend   │         │  Go Gateway  │         │ Python RAG   │       │
│    │  TypeScript  │ ──────► │   :8080      │ ──────► │   :8000      │       │
│    │    :3000     │  HTTP   │              │  gRPC   │              │       │
│    └──────────────┘         └──────┬───────┘         └──────┬───────┘       │
│                                    │                        │               │
│                         ┌──────────┴──────────┐            │               │
│                         │                     │            │               │
│                         ▼                     ▼            ▼               │
│                   ┌──────────┐          ┌──────────┐  ┌──────────┐         │
│                   │  Redis   │          │  OpenAI  │  │  Qdrant  │         │
│                   │  Cache   │          │ Anthropic│  │ Vectors  │         │
│                   └──────────┘          └──────────┘  └──────────┘         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Request Flow

```
User Request → Go Gateway (auth, rate limit, cache check)
                    │
                    ├─► Cache Hit → Return cached response (<5ms)
                    │
                    └─► Cache Miss → Python RAG Service
                                         │
                                         ├─► Vector search (Qdrant)
                                         ├─► Context assembly
                                         └─► LLM API call
                                                  │
                                                  ▼
                              Stream response back through Gateway
```

---

## Projects

### 1. [LLM Inference Gateway](./llm-gateway/) (Go)

High-performance API gateway for LLM providers with OpenAI-compatible interface.

**Key Features:**
- Multi-provider routing (OpenAI, Anthropic, Ollama)
- SSE streaming with goroutine-per-stream
- Semantic caching with Redis
- Rate limiting & circuit breaker
- Prometheus metrics & OpenTelemetry tracing

**Why Go?**
- Handles 10,000+ concurrent connections with ~2KB per goroutine
- Sub-50ms p99 latency vs 200ms+ with Python
- Single binary deployment, no runtime dependencies

```bash
cd llm-gateway && make run
```

### 2. [RAG Agent Service](./rag-agent/) (Python)

Retrieval-Augmented Generation pipeline with autonomous agents.

**Key Features:**
- Document ingestion (PDF, DOCX, Markdown)
- Hybrid search (dense + sparse vectors)
- LangGraph-based agent orchestration
- Tool use with function calling

**Why Python?**
- Direct access to LangChain, LlamaIndex ecosystem
- Native Qdrant, Pinecone client libraries
- Rapid prototyping with rich ML tooling

```bash
cd rag-agent && uv run uvicorn src.main:app --reload
```

### 3. [Agent Dashboard](./agent-dashboard/) (TypeScript)

Modern React dashboard for AI interactions.

**Key Features:**
- Real-time streaming chat UI
- Document upload & management
- Agent execution monitoring
- Usage analytics

```bash
cd agent-dashboard && pnpm dev
```

---

## Quick Start

### Prerequisites

- Docker & Docker Compose
- API keys for OpenAI and/or Anthropic

### One-Command Setup

```bash
# Clone the repository
git clone https://github.com/ChengYuChuan/ai-platform-portfolio.git
cd ai-platform-portfolio

# Set your API keys
export OPENAI_API_KEY=sk-your-key
export ANTHROPIC_API_KEY=sk-ant-your-key

# Start all services
make up

# Or with docker-compose directly
docker compose up -d
```

### Access Points

| Service | URL | Description |
|---------|-----|-------------|
| Dashboard | http://localhost:3000 | Web interface |
| Gateway API | http://localhost:8080 | OpenAI-compatible API |
| RAG Service | http://localhost:8000 | Python backend |
| Prometheus | http://localhost:9090 | Metrics |
| Grafana | http://localhost:3001 | Dashboards |

### Test the API

```bash
# Health check
curl http://localhost:8080/health

# Chat completion
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# With streaming
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Tell me a story"}],
    "stream": true
  }'
```

---

## Performance Benchmarks

### Go Gateway vs Python Baseline

Tested with `wrk` - 60 seconds, varying concurrent connections:

| Metric | Go Gateway | Python (FastAPI) | Improvement |
|--------|------------|------------------|-------------|
| **Throughput** | 2,100 req/s | 450 req/s | **4.7x** |
| **p50 Latency** | 12ms | 89ms | **7.4x** |
| **p99 Latency** | 45ms | 340ms | **7.5x** |
| **Memory (idle)** | 15 MB | 85 MB | **5.7x** |
| **Memory (load)** | 45 MB | 210 MB | **4.7x** |

### Concurrent Connection Handling

| Connections | Go (req/s) | Python (req/s) |
|-------------|------------|----------------|
| 100 | 1,850 | 420 |
| 500 | 2,100 | 380 |
| 1,000 | 2,050 | 290 |
| 5,000 | 1,900 | timeout |

> Full benchmark methodology and results: [docs/benchmark-report.md](./docs/benchmark-report.md)

---

## Development

### Individual Projects

```bash
# Go Gateway
cd llm-gateway
make run          # Run locally
make test         # Run tests
make lint         # Run linter

# Python RAG Service
cd rag-agent
uv sync           # Install dependencies
uv run pytest     # Run tests
uv run ruff check # Lint

# TypeScript Dashboard
cd agent-dashboard
pnpm install      # Install dependencies
pnpm dev          # Development server
pnpm test         # Run tests
```

### Full Stack

```bash
# Start everything
make up

# View logs
make logs

# Stop everything
make down

# Run all tests
make test-all
```

---

## Tech Stack

### Languages & Frameworks

| Layer | Technology | Purpose |
|-------|------------|---------|
| Gateway | Go 1.22, Chi, zerolog | High-performance HTTP/gRPC |
| RAG Service | Python 3.12, FastAPI, LangChain | AI/ML pipeline |
| Dashboard | TypeScript, Next.js 14, React | User interface |

### Infrastructure

| Component | Technology | Purpose |
|-----------|------------|---------|
| Vector DB | Qdrant | Semantic search |
| Cache | Redis | Response caching |
| Database | PostgreSQL | Metadata storage |
| Metrics | Prometheus + Grafana | Observability |
| Tracing | OpenTelemetry + Jaeger | Distributed tracing |
| Container | Docker Compose | Local orchestration |
| Orchestration | Kubernetes | Production deployment |
| Package Manager | Helm + Kustomize | K8s configuration |

---

## Kubernetes Deployment

This project includes production-ready Kubernetes manifests and Helm charts.

### Deploy with Kustomize

```bash
# Development environment (1 replica, DEBUG logs)
kubectl apply -k k8s/overlays/dev

# Production environment (3 replicas, optimized resources)
kubectl apply -k k8s/overlays/prod
```

### Deploy with Helm

```bash
# Add secrets first
kubectl create secret generic llm-api-keys \
  --from-literal=openai-api-key=$OPENAI_API_KEY \
  --from-literal=anthropic-api-key=$ANTHROPIC_API_KEY \
  -n ai-platform

# Install the chart
helm install ai-platform helm/ai-platform \
  -n ai-platform --create-namespace \
  -f helm/ai-platform/values.yaml

# Upgrade with custom values
helm upgrade ai-platform helm/ai-platform \
  --set gateway.replicaCount=3 \
  --set ragService.replicaCount=2
```

### Kubernetes Features

| Feature | Description |
|---------|-------------|
| **Network Policies** | Zero-trust pod-to-pod communication |
| **RBAC** | Least-privilege service accounts |
| **Ingress** | TLS termination, rate limiting, CORS |
| **Resource Limits** | CPU/memory constraints per service |
| **Health Probes** | Liveness & readiness checks |
| **Autoscaling** | HPA support (configurable) |

---

## Interactive Demo

Run the interactive demo script to test all platform features:

```bash
# Make executable
chmod +x scripts/demo.sh

# Run demo
./scripts/demo.sh
```

The demo includes:
- Health checks for all services
- Chat completion (streaming & non-streaming)
- Semantic caching demonstration
- Rate limiting test
- Document upload to RAG
- Metrics endpoints

---

## Documentation

- [Architecture Overview](./docs/architecture.md)
- [API Specification](./docs/api-specification.md)
- [Benchmark Report](./docs/benchmark-report.md)
- [Deployment Guide](./docs/deployment.md)
- [Security Policy](./SECURITY.md) ← API key rotation, incident response

---

## Project Structure

```
ai-platform-portfolio/
├── llm-gateway/           # Go - API Gateway
│   ├── cmd/gateway/       # Entry point
│   ├── internal/          # Private packages
│   │   ├── api/           # HTTP/gRPC handlers
│   │   ├── proxy/         # Provider routing
│   │   └── middleware/    # Auth, logging, etc.
│   └── pkg/models/        # Shared types
│
├── rag-agent/             # Python - RAG Service
│   ├── src/
│   │   ├── api/           # FastAPI routes
│   │   ├── rag/           # Retrieval pipeline
│   │   └── agents/        # LangGraph agents
│   └── tests/
│
├── agent-dashboard/       # TypeScript - Frontend
│   ├── src/
│   │   ├── app/           # Next.js pages
│   │   ├── components/    # React components
│   │   └── lib/           # Utilities
│   └── public/
│
├── k8s/                   # Kubernetes manifests
│   ├── base/              # Base configurations (13 files)
│   └── overlays/          # Environment-specific (dev/prod)
│
├── helm/                  # Helm charts
│   └── ai-platform/       # Main chart with templates
│
├── scripts/               # Utility scripts
│   └── demo.sh            # Interactive demo
│
├── docs/                  # Documentation
├── .github/workflows/     # CI/CD (Go, Python, TypeScript)
├── docker-compose.yml     # Full stack orchestration
├── SECURITY.md            # Security policy & key rotation
└── Makefile               # Unified commands
```

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Contact

**Yu-Chuan** - Recent MS Graduate from Heidelberg University specializing in Machine Learning & Generative AI

- Portfolio demonstrates: Go, Python, TypeScript proficiency for AI platform engineering
- Focus: Production-grade systems with measurable performance advantages

---

<div align="center">

**[⬆ Back to Top](#ai-platform-portfolio)**

</div>

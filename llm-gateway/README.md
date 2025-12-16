# LLM Inference Gateway

A high-performance, multi-provider LLM API gateway written in Go. This gateway provides a unified OpenAI-compatible API interface while routing requests to multiple LLM providers (OpenAI, Anthropic, Ollama).

## Features

- **Multi-Provider Support**: Route requests to OpenAI, Anthropic, or local Ollama models
- **OpenAI-Compatible API**: Drop-in replacement for OpenAI API clients
- **Streaming Support**: Server-Sent Events (SSE) for token-by-token streaming
- **High Performance**: Built with Go for low latency and high throughput
- **Automatic Routing**: Intelligent model-based routing to appropriate providers
- **Observability**: Structured logging with zerolog, Prometheus metrics ready
- **Production Ready**: Health checks, graceful shutdown, Docker support

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose (optional)
- API keys for OpenAI and/or Anthropic

### Local Development

1. **Clone and setup**
```bash
cd llm-gateway
cp .env.example .env
# Edit .env with your API keys
```

2. **Run directly**
```bash
# Install dependencies
go mod download

# Run the gateway
make run
```

3. **Or use Docker**
```bash
# Set your API keys
export OPENAI_API_KEY=sk-your-key
export ANTHROPIC_API_KEY=sk-ant-your-key

# Start the stack
docker compose up -d
```

### Test the API

```bash
# Health check
curl http://localhost:8080/health

# List available models
curl http://localhost:8080/v1/models

# Chat completion (OpenAI)
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Chat completion (Anthropic model via OpenAI-compatible API)
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-haiku-20240307",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Streaming
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Tell me a joke"}],
    "stream": true
  }'
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      LLM Gateway                            │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Router    │──│  Middleware │──│      Handlers       │ │
│  │   (Chi)     │  │  (Auth,Log) │  │  (Chat, Embed...)   │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│                           │                                 │
│                    ┌──────┴──────┐                         │
│                    │ Proxy Router │                         │
│                    └──────┬──────┘                         │
│         ┌─────────────────┼─────────────────┐              │
│         ▼                 ▼                 ▼              │
│  ┌────────────┐   ┌────────────┐   ┌────────────┐         │
│  │   OpenAI   │   │  Anthropic │   │   Ollama   │         │
│  │  Provider  │   │  Provider  │   │  Provider  │         │
│  └────────────┘   └────────────┘   └────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

## Configuration

Configuration can be done via `config.yaml` or environment variables:

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `LLM_GATEWAY_SERVER_PORT` | Server port | 8080 |
| `LLM_GATEWAY_LOG_LEVEL` | Log level (debug/info/warn/error) | info |
| `LLM_GATEWAY_PROVIDERS_OPENAI_API_KEY` | OpenAI API key | - |
| `LLM_GATEWAY_PROVIDERS_ANTHROPIC_API_KEY` | Anthropic API key | - |
| `LLM_GATEWAY_PROVIDERS_DEFAULT` | Default provider | openai |

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/ready` | GET | Readiness check (includes provider status) |
| `/metrics` | GET | Prometheus metrics |
| `/v1/chat/completions` | POST | Chat completion (OpenAI-compatible) |
| `/v1/completions` | POST | Legacy completion |
| `/v1/embeddings` | POST | Generate embeddings |
| `/v1/models` | GET | List available models |
| `/v1/messages` | POST | Anthropic-style messages API |

## Model Routing

The gateway automatically routes requests based on model name prefixes:

| Prefix | Provider |
|--------|----------|
| `gpt-*` | OpenAI |
| `claude-*` | Anthropic |
| `text-embedding-*` | OpenAI |
| Other | Default provider |

## Development

```bash
# Run tests
make test

# Run linter
make lint

# Format code
make fmt

# Build binary
make build

# Build Docker image
make docker-build
```

## Project Structure

```
llm-gateway/
├── cmd/gateway/          # Application entry point
├── internal/
│   ├── api/rest/         # HTTP handlers and router
│   ├── config/           # Configuration management
│   ├── middleware/       # HTTP middleware (auth, logging)
│   ├── proxy/            # Provider routing
│   │   └── providers/    # LLM provider implementations
│   ├── cache/            # Semantic caching (TODO)
│   ├── queue/            # Request queuing (TODO)
│   └── circuitbreaker/   # Circuit breaker (TODO)
├── pkg/models/           # Request/Response DTOs
├── proto/                # gRPC definitions (TODO)
├── config.yaml           # Default configuration
├── Dockerfile            # Multi-stage build
├── docker-compose.yml    # Local development stack
└── Makefile              # Build automation
```

## Roadmap

### Phase 1: Foundation ✅
- [x] Project scaffolding
- [x] Configuration management
- [x] Basic REST API
- [x] OpenAI provider
- [x] Anthropic provider
- [x] Request/response logging

### Phase 2: Streaming & Multi-Provider
- [x] SSE streaming support
- [x] Model-based routing
- [ ] Ollama provider
- [ ] Load balancing

### Phase 3: Reliability
- [ ] Rate limiting
- [ ] Circuit breaker
- [ ] Retry with backoff
- [ ] Request timeout handling

### Phase 4: Performance
- [ ] Connection pooling
- [ ] Request queuing
- [ ] Semantic caching (Redis)

### Phase 5: Observability
- [ ] Prometheus metrics
- [ ] OpenTelemetry tracing
- [ ] gRPC API

## License

MIT

## Contributing

Contributions are welcome! Please read the contributing guidelines first.

# System Architecture

## Overview

This document describes the architecture of the AI Platform, a production-grade system demonstrating full-stack AI engineering across Go, Python, and TypeScript.

## Design Principles

### 1. Language Selection by Technical Requirements

| Requirement | Language | Rationale |
|-------------|----------|-----------|
| High-throughput networking | Go | Goroutines, efficient net/http, single binary |
| AI/ML ecosystem access | Python | LangChain, vector DBs, model libraries |
| Modern web UI | TypeScript | React, type safety, real-time updates |

### 2. Separation of Concerns

```
┌─────────────────────────────────────────────────────────────────┐
│                         Presentation Layer                       │
│                    (TypeScript / Next.js / React)                │
│                                                                   │
│  • User interface                                                 │
│  • Real-time streaming display                                   │
│  • State management                                               │
└───────────────────────────────┬───────────────────────────────────┘
                                │ HTTP/SSE
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Gateway Layer                           │
│                         (Go / Chi / gRPC)                        │
│                                                                   │
│  • Request routing                                                │
│  • Authentication & rate limiting                                │
│  • Response caching                                               │
│  • Load balancing                                                 │
│  • Circuit breaking                                               │
└───────────────────────────────┬───────────────────────────────────┘
                                │ gRPC/HTTP
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Application Layer                        │
│                    (Python / FastAPI / LangChain)                │
│                                                                   │
│  • RAG pipeline                                                   │
│  • Agent orchestration                                            │
│  • Document processing                                            │
│  • Vector search                                                  │
└───────────────────────────────┬───────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                           Data Layer                             │
│                                                                   │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐  │
│  │PostgreSQL│    │  Qdrant  │    │  Redis   │    │  LLM API │  │
│  │ Metadata │    │ Vectors  │    │  Cache   │    │ Providers│  │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 3. Stateless Services

All services are designed to be stateless and horizontally scalable:

- **Gateway**: Can run multiple instances behind a load balancer
- **RAG Service**: Stateless processing, all state in databases
- **Dashboard**: Static assets + API calls

## Component Details

### LLM Gateway (Go)

**Purpose**: High-performance entry point for all LLM requests

**Key Responsibilities**:
1. Accept HTTP/gRPC requests from clients
2. Authenticate and authorize requests
3. Apply rate limiting per API key
4. Check semantic cache for similar prompts
5. Route to appropriate LLM provider
6. Stream responses back to clients
7. Collect metrics and traces

**Technology Choices**:

| Component | Choice | Rationale |
|-----------|--------|-----------|
| HTTP Router | Chi | stdlib compatible, middleware support |
| Config | Viper | Industry standard, env var support |
| Logging | zerolog | Zero allocation, structured JSON |
| Metrics | Prometheus | Industry standard, Go native client |
| Tracing | OpenTelemetry | Vendor neutral, wide adoption |

**Concurrency Model**:

```go
// Each request gets its own goroutine (handled by net/http)
// Streaming uses goroutine-per-stream pattern

func (h *Handler) ChatCompletionStream(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Get stream from provider
    stream, err := provider.ChatCompletionStream(ctx, req)
    
    // Forward stream to client
    for {
        select {
        case <-ctx.Done():
            return
        case chunk := <-stream:
            w.Write(formatSSE(chunk))
            flusher.Flush()
        }
    }
}
```

### RAG Agent Service (Python)

**Purpose**: Document processing, retrieval, and LLM orchestration

**Key Responsibilities**:
1. Ingest and chunk documents
2. Generate and store embeddings
3. Perform hybrid search (dense + sparse)
4. Assemble context for LLM
5. Execute agent workflows
6. Handle tool calls

**RAG Pipeline**:

```
Document → Loader → Chunker → Embedder → Vector Store
                                              │
Query → Embedder → Retriever → Reranker → Context
                                              │
                                              ▼
                                    LLM (with context)
                                              │
                                              ▼
                                         Response
```

**Agent Architecture (LangGraph)**:

```python
from langgraph.graph import StateGraph

# Define agent state
class AgentState(TypedDict):
    messages: list[BaseMessage]
    tools_called: list[str]
    final_answer: str | None

# Build graph
graph = StateGraph(AgentState)
graph.add_node("reason", reasoning_node)
graph.add_node("act", action_node)
graph.add_node("observe", observation_node)

graph.add_edge("reason", "act")
graph.add_edge("act", "observe")
graph.add_conditional_edges("observe", should_continue)
```

### Agent Dashboard (TypeScript)

**Purpose**: User interface for AI interactions

**Key Responsibilities**:
1. Render streaming chat interface
2. Handle file uploads
3. Display agent execution logs
4. Show analytics and metrics

**Streaming Implementation**:

```typescript
// SSE streaming with React hooks
function useChat() {
  const [messages, setMessages] = useState<Message[]>([]);
  
  const sendMessage = async (content: string) => {
    const response = await fetch('/v1/chat/completions', {
      method: 'POST',
      body: JSON.stringify({ messages: [...messages, { role: 'user', content }], stream: true }),
    });
    
    const reader = response.body?.getReader();
    const decoder = new TextDecoder();
    
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      
      const chunk = decoder.decode(value);
      // Parse SSE and update state
      setMessages(prev => updateWithChunk(prev, chunk));
    }
  };
  
  return { messages, sendMessage };
}
```

## Communication Patterns

### Synchronous (HTTP/gRPC)

Used for: Simple request/response, real-time queries

```
Client → Gateway → RAG Service → LLM Provider
   ↑                                    │
   └────────────────────────────────────┘
```

### Streaming (SSE)

Used for: Token-by-token LLM responses

```
Client ←─SSE─← Gateway ←─SSE─← LLM Provider
         │
         └─ Each token forwarded immediately
```

### Async (Message Queue)

Future enhancement for: Long-running tasks, batch processing

```
Client → Gateway → Message Queue → Worker → Result Store
   ↑                                              │
   └──────────── Poll/Webhook ────────────────────┘
```

## Data Flow

### Chat Completion Request

```
1. User sends POST /v1/chat/completions
   │
2. Gateway receives request
   ├─ Validate API key
   ├─ Check rate limit
   ├─ Compute semantic hash
   └─ Check cache
       │
       ├─ CACHE HIT → Return cached response (< 5ms)
       │
       └─ CACHE MISS → Continue
           │
3. Gateway routes to RAG Service (if RAG enabled)
   │
4. RAG Service processes
   ├─ Embed query
   ├─ Search vector store
   ├─ Rerank results
   └─ Construct augmented prompt
       │
5. RAG Service calls LLM
   │
6. Response streams back
   ├─ Gateway caches (async)
   ├─ Gateway logs metrics
   └─ Client receives tokens
```

## Scalability Considerations

### Horizontal Scaling

| Service | Scaling Strategy |
|---------|------------------|
| Gateway | Add instances behind load balancer |
| RAG Service | Add instances, share vector DB |
| Vector DB | Qdrant clustering |
| Cache | Redis cluster |

### Bottleneck Analysis

| Component | Potential Bottleneck | Mitigation |
|-----------|---------------------|------------|
| LLM API | Rate limits, latency | Multiple providers, queue |
| Vector search | Query time at scale | Approximate search, caching |
| Gateway | Connection limits | Horizontal scaling |

## Security Architecture

### Authentication Flow

```
Client → [API Key in Header] → Gateway → Validate → Route
                                 │
                                 └─ Invalid → 401 Unauthorized
```

### Security Measures

1. **API Key Authentication**: Required for all /v1/* endpoints
2. **Rate Limiting**: Per-key limits prevent abuse
3. **Input Validation**: Strict request schema validation
4. **TLS**: All external communication encrypted
5. **Secret Management**: API keys via environment variables

## Observability

### Three Pillars

| Pillar | Implementation | Storage |
|--------|---------------|---------|
| Metrics | Prometheus client | Prometheus |
| Logging | zerolog (Go), structlog (Python) | stdout → aggregator |
| Tracing | OpenTelemetry | Jaeger |

### Key Metrics

**Gateway**:
- `llm_gateway_requests_total` - Total requests by endpoint
- `llm_gateway_request_duration_seconds` - Latency histogram
- `llm_gateway_provider_errors_total` - Provider errors by type
- `llm_gateway_cache_hits_total` - Cache hit rate

**RAG Service**:
- `rag_documents_processed_total` - Ingestion count
- `rag_retrieval_duration_seconds` - Search latency
- `rag_agent_steps_total` - Agent execution steps

## Deployment

### Container Architecture

```yaml
services:
  gateway:
    replicas: 2
    resources:
      limits:
        memory: 256Mi
        cpu: 500m
    
  rag-service:
    replicas: 2
    resources:
      limits:
        memory: 1Gi
        cpu: 1000m
```

### Health Checks

| Service | Endpoint | Checks |
|---------|----------|--------|
| Gateway | /health | HTTP listener |
| Gateway | /ready | Provider connections |
| RAG Service | /health | DB connections |

## Future Considerations

1. **Kubernetes Deployment**: Helm charts for production
2. **Multi-Region**: Geographic distribution for latency
3. **Model Serving**: Self-hosted models via vLLM
4. **A/B Testing**: Provider performance comparison
5. **Cost Optimization**: Smart routing based on cost/performance

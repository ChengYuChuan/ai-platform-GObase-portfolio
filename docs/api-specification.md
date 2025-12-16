# API Specification

## Overview

The AI Platform exposes an **OpenAI-compatible API** through the Go Gateway. This allows any OpenAI SDK or client to work with the platform without modification.

## Base URL

```
http://localhost:8080
```

## Authentication

All `/v1/*` endpoints require authentication via API key:

```bash
# Option 1: Authorization header (recommended)
Authorization: Bearer sk-your-api-key

# Option 2: X-API-Key header
X-API-Key: sk-your-api-key
```

## Endpoints

### Health & Status

#### GET /health

Basic health check.

**Response**:
```json
{
  "status": "healthy",
  "service": "llm-gateway"
}
```

#### GET /ready

Readiness check with provider status.

**Response**:
```json
{
  "status": "ready",
  "providers": ["openai", "anthropic"]
}
```

#### GET /metrics

Prometheus metrics endpoint.

---

### Chat Completions

#### POST /v1/chat/completions

Create a chat completion (OpenAI-compatible).

**Request**:
```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "temperature": 0.7,
  "max_tokens": 1000,
  "stream": false
}
```

**Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| model | string | Yes | Model identifier (e.g., "gpt-4o-mini", "claude-3-haiku-20240307") |
| messages | array | Yes | Array of message objects |
| temperature | number | No | Sampling temperature (0-2). Default: 1 |
| max_tokens | integer | No | Maximum tokens to generate |
| stream | boolean | No | Enable SSE streaming. Default: false |
| top_p | number | No | Nucleus sampling parameter |
| stop | array | No | Stop sequences |
| presence_penalty | number | No | Presence penalty (-2 to 2) |
| frequency_penalty | number | No | Frequency penalty (-2 to 2) |

**Message Object**:
```json
{
  "role": "user|assistant|system",
  "content": "Message content"
}
```

**Response** (non-streaming):
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1702000000,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 10,
    "total_tokens": 30
  }
}
```

**Response** (streaming):

When `stream: true`, the response is Server-Sent Events (SSE):

```
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1702000000,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1702000000,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1702000000,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1702000000,"model":"gpt-4o-mini","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

---

### Legacy Completions

#### POST /v1/completions

Create a text completion (legacy endpoint).

**Request**:
```json
{
  "model": "gpt-3.5-turbo-instruct",
  "prompt": "Once upon a time",
  "max_tokens": 100,
  "temperature": 0.7
}
```

**Response**:
```json
{
  "id": "cmpl-abc123",
  "object": "text_completion",
  "created": 1702000000,
  "model": "gpt-3.5-turbo-instruct",
  "choices": [
    {
      "text": ", there was a small village...",
      "index": 0,
      "finish_reason": "length"
    }
  ],
  "usage": {
    "prompt_tokens": 5,
    "completion_tokens": 100,
    "total_tokens": 105
  }
}
```

---

### Embeddings

#### POST /v1/embeddings

Generate embeddings for text.

**Request**:
```json
{
  "model": "text-embedding-3-small",
  "input": "The quick brown fox jumps over the lazy dog."
}
```

**Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| model | string | Yes | Embedding model identifier |
| input | string or array | Yes | Text to embed (string or array of strings) |
| encoding_format | string | No | "float" or "base64". Default: "float" |
| dimensions | integer | No | Desired embedding dimensions |

**Response**:
```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "embedding": [0.0023, -0.0094, 0.0152, ...],
      "index": 0
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 10,
    "total_tokens": 10
  }
}
```

---

### Models

#### GET /v1/models

List available models.

**Response**:
```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "owned_by": "openai",
      "provider": "openai"
    },
    {
      "id": "gpt-4o-mini",
      "object": "model",
      "owned_by": "openai",
      "provider": "openai"
    },
    {
      "id": "claude-3-5-sonnet-20241022",
      "object": "model",
      "owned_by": "anthropic",
      "provider": "anthropic"
    },
    {
      "id": "claude-3-haiku-20240307",
      "object": "model",
      "owned_by": "anthropic",
      "provider": "anthropic"
    }
  ]
}
```

---

### Anthropic Messages API (Optional)

#### POST /v1/messages

Anthropic-style messages endpoint.

**Request**:
```json
{
  "model": "claude-3-haiku-20240307",
  "max_tokens": 1024,
  "system": "You are a helpful assistant.",
  "messages": [
    {"role": "user", "content": "Hello!"}
  ]
}
```

**Response**:
Converted to OpenAI-compatible format internally.

---

## Model Routing

The gateway automatically routes requests based on model name:

| Model Pattern | Provider |
|---------------|----------|
| `gpt-*` | OpenAI |
| `text-embedding-*` | OpenAI |
| `claude-*` | Anthropic |
| Other | Default provider (configurable) |

---

## Error Responses

All errors follow this format:

```json
{
  "error": {
    "type": "error_type",
    "message": "Human-readable error message",
    "param": "parameter_name",
    "code": "error_code"
  }
}
```

### Common Error Types

| HTTP Status | Type | Description |
|-------------|------|-------------|
| 400 | invalid_request | Malformed request |
| 401 | invalid_api_key | Invalid or missing API key |
| 403 | permission_denied | Not authorized for this operation |
| 404 | not_found | Resource not found |
| 429 | rate_limit_exceeded | Too many requests |
| 500 | internal_error | Server error |
| 503 | service_unavailable | Provider unavailable |

### Example Error Response

```json
{
  "error": {
    "type": "invalid_request",
    "message": "model is required",
    "param": "model",
    "code": "missing_parameter"
  }
}
```

---

## Rate Limiting

When rate limiting is enabled:

| Header | Description |
|--------|-------------|
| X-RateLimit-Limit | Maximum requests per minute |
| X-RateLimit-Remaining | Remaining requests |
| X-RateLimit-Reset | Unix timestamp when limit resets |

When exceeded:
```json
{
  "error": {
    "type": "rate_limit_exceeded",
    "message": "Rate limit exceeded. Please retry after 60 seconds."
  }
}
```

---

## SDK Compatibility

The API is compatible with official OpenAI SDKs:

### Python

```python
from openai import OpenAI

client = OpenAI(
    api_key="your-api-key",
    base_url="http://localhost:8080/v1"
)

response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

### Node.js

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: 'your-api-key',
  baseURL: 'http://localhost:8080/v1',
});

const response = await client.chat.completions.create({
  model: 'gpt-4o-mini',
  messages: [{ role: 'user', content: 'Hello!' }],
});
console.log(response.choices[0].message.content);
```

### cURL

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

---

## WebSocket API (Future)

Reserved for future real-time bidirectional communication:

```
ws://localhost:8080/v1/realtime
```

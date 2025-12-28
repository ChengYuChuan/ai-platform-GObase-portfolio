# Deployment Guide

This document provides comprehensive instructions for deploying the AI Platform in various environments.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start (Local Development)](#quick-start-local-development)
3. [Docker Compose Deployment](#docker-compose-deployment)
4. [Production Deployment](#production-deployment)
5. [Environment Configuration](#environment-configuration)
6. [Health Checks & Monitoring](#health-checks--monitoring)
7. [Scaling Guidelines](#scaling-guidelines)
8. [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software

| Software | Version | Purpose |
|----------|---------|---------|
| Docker | 24.0+ | Container runtime |
| Docker Compose | 2.20+ | Multi-container orchestration |
| Git | 2.40+ | Source control |

### Optional (for local development)

| Software | Version | Purpose |
|----------|---------|---------|
| Go | 1.21+ | Gateway development |
| Python | 3.11+ | RAG service development |
| Node.js | 20+ | Dashboard development |
| uv | 0.5+ | Python package management |
| pnpm | 9+ | Node.js package management |

### Required API Keys

At minimum, you need one of the following:

- **OpenAI API Key**: For GPT-4, GPT-3.5-turbo models
- **Anthropic API Key**: For Claude models

## Quick Start (Local Development)

### 1. Clone the Repository

```bash
git clone https://github.com/your-username/ai-platform-portfolio.git
cd ai-platform-portfolio
```

### 2. Configure Environment

```bash
cp .env.example .env
```

Edit `.env` and add your API keys:

```bash
OPENAI_API_KEY=sk-your-openai-key-here
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key-here
```

### 3. Start All Services

```bash
docker compose up -d
```

### 4. Verify Deployment

```bash
# Check all services are running
docker compose ps

# Test Gateway health
curl http://localhost:8080/health

# Test RAG service health
curl http://localhost:8000/health

# Access Dashboard
open http://localhost:3000
```

## Docker Compose Deployment

### Service Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Docker Compose                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐              │
│  │ Dashboard│    │  Gateway │    │RAG Service│              │
│  │  :3000   │───▶│  :8080   │───▶│  :8000   │              │
│  └──────────┘    └──────────┘    └──────────┘              │
│                        │               │                     │
│                        ▼               ▼                     │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐              │
│  │  Redis   │    │PostgreSQL│    │  Qdrant  │              │
│  │  :6379   │    │  :5432   │    │  :6333   │              │
│  └──────────┘    └──────────┘    └──────────┘              │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Available Profiles

**Default Services** (always started):
- `gateway` - LLM Gateway (Go)
- `rag-service` - RAG Agent (Python)
- `dashboard` - Web UI (TypeScript)
- `redis` - Caching
- `postgres` - Metadata storage
- `qdrant` - Vector database

**Monitoring Profile** (optional):
```bash
docker compose --profile monitoring up -d
```

This adds:
- `prometheus` - Metrics collection (:9090)
- `grafana` - Dashboards (:3001)
- `jaeger` - Distributed tracing (:16686)

### Docker Compose Commands

```bash
# Start all services
docker compose up -d

# Start with monitoring
docker compose --profile monitoring up -d

# View logs
docker compose logs -f gateway
docker compose logs -f rag-service

# Restart a service
docker compose restart gateway

# Stop all services
docker compose down

# Stop and remove volumes (clean start)
docker compose down -v
```

## Production Deployment

### Pre-Deployment Checklist

- [ ] API keys stored in secure secrets manager
- [ ] TLS certificates configured
- [ ] Database passwords changed from defaults
- [ ] Rate limiting configured appropriately
- [ ] Monitoring and alerting set up
- [ ] Backup strategy for databases
- [ ] Log aggregation configured

### Kubernetes Deployment (Recommended)

#### 1. Create Namespace

```bash
kubectl create namespace ai-platform
```

#### 2. Create Secrets

```bash
kubectl create secret generic llm-api-keys \
  --namespace ai-platform \
  --from-literal=openai-api-key=$OPENAI_API_KEY \
  --from-literal=anthropic-api-key=$ANTHROPIC_API_KEY
```

#### 3. Deploy Services

```yaml
# gateway-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
  namespace: ai-platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      labels:
        app: gateway
    spec:
      containers:
      - name: gateway
        image: ai-platform-gateway:latest
        ports:
        - containerPort: 8080
        env:
        - name: LLM_GATEWAY_PROVIDERS_OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: llm-api-keys
              key: openai-api-key
        resources:
          requests:
            memory: "128Mi"
            cpu: "250m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
```

#### 4. Expose Services

```yaml
# gateway-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: gateway
  namespace: ai-platform
spec:
  selector:
    app: gateway
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gateway-ingress
  namespace: ai-platform
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - api.your-domain.com
    secretName: gateway-tls
  rules:
  - host: api.your-domain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: gateway
            port:
              number: 8080
```

### Cloud Provider Deployments

#### AWS (ECS/Fargate)

1. Push images to ECR:
```bash
aws ecr get-login-password | docker login --username AWS --password-stdin $ECR_REGISTRY
docker tag ai-platform-gateway:latest $ECR_REGISTRY/gateway:latest
docker push $ECR_REGISTRY/gateway:latest
```

2. Create ECS task definition with secrets from AWS Secrets Manager

#### Google Cloud (Cloud Run)

```bash
# Deploy Gateway
gcloud run deploy gateway \
  --image gcr.io/$PROJECT_ID/gateway:latest \
  --platform managed \
  --region us-central1 \
  --set-secrets OPENAI_API_KEY=openai-key:latest

# Deploy RAG Service
gcloud run deploy rag-service \
  --image gcr.io/$PROJECT_ID/rag-service:latest \
  --platform managed \
  --region us-central1 \
  --memory 1Gi
```

## Environment Configuration

### Gateway Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_GATEWAY_SERVER_PORT` | 8080 | HTTP server port |
| `LLM_GATEWAY_LOG_LEVEL` | info | Logging level (debug, info, warn, error) |
| `LLM_GATEWAY_LOG_FORMAT` | json | Log format (json, pretty) |
| `LLM_GATEWAY_PROVIDERS_OPENAI_API_KEY` | - | OpenAI API key |
| `LLM_GATEWAY_PROVIDERS_ANTHROPIC_API_KEY` | - | Anthropic API key |
| `LLM_GATEWAY_PROVIDERS_DEFAULT` | openai | Default provider |
| `LLM_GATEWAY_CACHE_ENABLED` | true | Enable response caching |
| `LLM_GATEWAY_CACHE_REDIS_ADDRESS` | redis:6379 | Redis connection |
| `LLM_GATEWAY_RATELIMIT_ENABLED` | true | Enable rate limiting |
| `LLM_GATEWAY_RATELIMIT_REQUESTS_PER_MIN` | 60 | Requests per minute |

### RAG Service Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENAI_API_KEY` | - | OpenAI API key |
| `ANTHROPIC_API_KEY` | - | Anthropic API key |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `QDRANT_URL` | http://qdrant:6333 | Qdrant vector DB URL |
| `REDIS_URL` | redis://redis:6379 | Redis connection |
| `LOG_LEVEL` | info | Logging level |
| `CHUNK_SIZE` | 1000 | Document chunk size |
| `CHUNK_OVERLAP` | 200 | Chunk overlap |

### Dashboard Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXT_PUBLIC_GATEWAY_URL` | http://localhost:8080 | Public Gateway URL (client-side) |
| `GATEWAY_URL` | http://gateway:8080 | Internal Gateway URL (server-side) |

## Health Checks & Monitoring

### Health Endpoints

| Service | Endpoint | Purpose |
|---------|----------|---------|
| Gateway | `GET /health` | Basic liveness |
| Gateway | `GET /ready` | Readiness with dependencies |
| Gateway | `GET /metrics` | Prometheus metrics |
| RAG Service | `GET /health` | Basic liveness |
| RAG Service | `GET /docs` | API documentation |

### Monitoring Setup

#### Prometheus Configuration

The platform exports metrics in Prometheus format:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'gateway'
    static_configs:
      - targets: ['gateway:8080']
    metrics_path: /metrics

  - job_name: 'rag-service'
    static_configs:
      - targets: ['rag-service:8000']
    metrics_path: /metrics
```

#### Grafana Dashboards

Pre-configured dashboards are available in `monitoring/grafana/dashboards/`:
- `gateway-dashboard.json` - Gateway metrics
- `rag-dashboard.json` - RAG service metrics

#### Alerting Examples

```yaml
# alerting-rules.yml
groups:
- name: ai-platform
  rules:
  - alert: HighErrorRate
    expr: rate(llm_gateway_requests_total{status=~"5.."}[5m]) > 0.1
    for: 5m
    annotations:
      summary: High error rate detected

  - alert: HighLatency
    expr: histogram_quantile(0.95, rate(llm_gateway_request_duration_seconds_bucket[5m])) > 5
    for: 5m
    annotations:
      summary: P95 latency exceeds 5 seconds
```

## Scaling Guidelines

### Horizontal Scaling

| Service | Scaling Trigger | Recommendation |
|---------|----------------|----------------|
| Gateway | CPU > 70%, RPS > 100/instance | Add instances |
| RAG Service | Memory > 80%, queue depth | Add instances |
| Redis | Memory > 80% | Scale vertically or cluster |
| Qdrant | Query latency > 100ms | Add replicas |

### Resource Recommendations

**Development**:
```yaml
gateway:
  memory: 256Mi
  cpu: 250m

rag-service:
  memory: 512Mi
  cpu: 500m
```

**Production**:
```yaml
gateway:
  memory: 512Mi
  cpu: 1000m
  replicas: 2-4

rag-service:
  memory: 2Gi
  cpu: 2000m
  replicas: 2-4
```

## Troubleshooting

### Common Issues

#### 1. Services Not Starting

```bash
# Check container logs
docker compose logs gateway
docker compose logs rag-service

# Check if ports are in use
lsof -i :8080
lsof -i :8000
```

#### 2. Database Connection Failed

```bash
# Verify PostgreSQL is running
docker compose exec postgres pg_isready

# Check connection from RAG service
docker compose exec rag-service python -c "import asyncpg; print('OK')"
```

#### 3. API Key Issues

```bash
# Verify environment variables
docker compose exec gateway env | grep API_KEY

# Test OpenAI connectivity
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
  https://api.openai.com/v1/models
```

#### 4. High Memory Usage

```bash
# Check container memory
docker stats

# For Python service, check for memory leaks
docker compose exec rag-service python -c "import tracemalloc; tracemalloc.start()"
```

### Debug Mode

Enable debug logging:

```bash
# Set in .env
LOG_LEVEL=debug
LOG_FORMAT=pretty

# Restart services
docker compose restart
```

### Getting Help

1. Check service logs: `docker compose logs -f <service>`
2. Verify health endpoints: `curl http://localhost:8080/health`
3. Review metrics: Access Prometheus at http://localhost:9090
4. Check traces: Access Jaeger at http://localhost:16686

## Security Considerations

### Production Checklist

- [ ] Change all default passwords (PostgreSQL, Grafana)
- [ ] Enable TLS for all external endpoints
- [ ] Use secrets management (Vault, AWS Secrets Manager)
- [ ] Configure network policies to restrict inter-service communication
- [ ] Enable audit logging
- [ ] Set up WAF for public endpoints
- [ ] Regular security updates for base images

### API Key Rotation

1. Generate new API key in provider console
2. Update secret in secrets manager
3. Restart services to pick up new keys
4. Revoke old API key

```bash
# Kubernetes secret update
kubectl create secret generic llm-api-keys \
  --namespace ai-platform \
  --from-literal=openai-api-key=$NEW_OPENAI_KEY \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart deployments
kubectl rollout restart deployment/gateway -n ai-platform
```

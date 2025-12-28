# Security Policy

## Overview

This document outlines the security policies, procedures, and best practices for the AI Platform. Security is a top priority, and we take all security concerns seriously.

## Table of Contents

- [Supported Versions](#supported-versions)
- [Reporting a Vulnerability](#reporting-a-vulnerability)
- [Security Architecture](#security-architecture)
- [Authentication & Authorization](#authentication--authorization)
- [API Key Management](#api-key-management)
- [API Key Rotation Procedure](#api-key-rotation-procedure)
- [Data Protection](#data-protection)
- [Network Security](#network-security)
- [Container Security](#container-security)
- [Secrets Management](#secrets-management)
- [Audit Logging](#audit-logging)
- [Incident Response](#incident-response)

---

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

We provide security updates for the latest major version only.

---

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability, please send an email to:

📧 **security@ai-platform.example.com**

Include the following information:
- Type of vulnerability (e.g., SQL injection, XSS, authentication bypass)
- Location of the affected source code (file path, line numbers)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if available)
- Impact assessment

### Response Timeline

| Stage | Timeline |
|-------|----------|
| Initial Response | Within 48 hours |
| Vulnerability Assessment | Within 7 days |
| Fix Development | Within 30 days (critical: 7 days) |
| Public Disclosure | After fix is released |

---

## Security Architecture

```
                                    ┌─────────────────┐
                                    │   CloudFlare    │
                                    │   WAF + DDoS    │
                                    └────────┬────────┘
                                             │
                                    ┌────────▼────────┐
                                    │  Ingress NGINX  │
                                    │  (TLS + Rate    │
                                    │   Limiting)     │
                                    └────────┬────────┘
                                             │
              ┌──────────────────────────────┼──────────────────────────────┐
              │                              │                              │
    ┌─────────▼─────────┐        ┌───────────▼───────────┐      ┌──────────▼──────────┐
    │    Dashboard      │        │     API Gateway       │      │    RAG Service      │
    │  (Auth Required)  │───────▶│  (API Key Validation) │──────│  (Internal Only)    │
    └───────────────────┘        └───────────────────────┘      └─────────────────────┘
                                             │                              │
                                    ┌────────▼────────┐          ┌──────────▼──────────┐
                                    │     Redis       │          │   Vector DB/SQL     │
                                    │  (Rate Limits)  │          │  (Encrypted @Rest)  │
                                    └─────────────────┘          └─────────────────────┘
```

---

## Authentication & Authorization

### API Authentication

The API Gateway supports multiple authentication methods:

#### 1. API Key Authentication (Primary)

```http
Authorization: Bearer <api_key>
```

or

```http
X-API-Key: <api_key>
```

#### 2. JWT Token (Future)

```http
Authorization: Bearer <jwt_token>
```

### API Key Properties

| Property | Specification |
|----------|---------------|
| Format | Alphanumeric with prefix (e.g., `sk-ai-xxxx`) |
| Length | 64 characters minimum |
| Entropy | 256 bits |
| Hashing | SHA-256 (stored) |
| Transmission | HTTPS only |

### Authorization Scopes

| Scope | Description |
|-------|-------------|
| `chat:read` | Access chat completion API |
| `chat:write` | Create chat completions |
| `documents:read` | Read uploaded documents |
| `documents:write` | Upload and manage documents |
| `admin:*` | Full administrative access |

---

## API Key Management

### Key Generation

API keys are generated using cryptographically secure random number generators:

```go
// Go implementation example
func GenerateAPIKey() (string, error) {
    bytes := make([]byte, 32)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return "sk-ai-" + base64.URLEncoding.EncodeToString(bytes), nil
}
```

### Key Storage

- **Never** store API keys in plain text
- Store only the SHA-256 hash of the API key
- Use a separate secrets management system for production

### Key Lifecycle

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Created    │────▶│    Active    │────▶│   Expiring   │────▶│   Revoked    │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
                            │
                            │ (Emergency)
                            ▼
                     ┌──────────────┐
                     │   Revoked    │
                     └──────────────┘
```

---

## API Key Rotation Procedure

### Automated Rotation (Recommended)

#### Prerequisites

1. Access to secrets management system (Vault, AWS Secrets Manager, etc.)
2. Administrative access to AI Platform
3. Deployment pipeline access

#### Step-by-Step Procedure

##### Step 1: Generate New API Key

```bash
# Generate new key
NEW_KEY=$(openssl rand -base64 32 | tr -d '=' | tr '+/' '-_')
PREFIXED_KEY="sk-ai-${NEW_KEY}"
echo "New API Key: ${PREFIXED_KEY}"
```

##### Step 2: Store New Key in Secrets Manager

**HashiCorp Vault:**
```bash
vault kv put secret/ai-platform/api-keys \
  openai-api-key="${OPENAI_API_KEY}" \
  anthropic-api-key="${ANTHROPIC_API_KEY}" \
  internal-api-key="${PREFIXED_KEY}"
```

**AWS Secrets Manager:**
```bash
aws secretsmanager update-secret \
  --secret-id ai-platform/api-keys \
  --secret-string "{\"openai-api-key\":\"${OPENAI_API_KEY}\"}"
```

**Kubernetes Secrets:**
```bash
kubectl create secret generic llm-api-keys \
  --from-literal=openai-api-key="${OPENAI_API_KEY}" \
  --from-literal=anthropic-api-key="${ANTHROPIC_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -
```

##### Step 3: Deploy Updated Configuration

```bash
# Trigger rolling restart to pick up new secrets
kubectl rollout restart deployment/gateway -n ai-platform
kubectl rollout restart deployment/rag-service -n ai-platform

# Wait for rollout completion
kubectl rollout status deployment/gateway -n ai-platform
kubectl rollout status deployment/rag-service -n ai-platform
```

##### Step 4: Verify New Key Works

```bash
# Test API with new key
curl -X POST https://api.ai-platform.example.com/v1/chat/completions \
  -H "Authorization: Bearer ${PREFIXED_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}'
```

##### Step 5: Revoke Old Key

```bash
# Mark old key as revoked in database
# This depends on your key management implementation
```

### Manual Rotation Checklist

- [ ] Generate new API key with sufficient entropy
- [ ] Store new key in secrets management system
- [ ] Update Kubernetes secrets
- [ ] Trigger rolling deployment
- [ ] Verify new key functionality
- [ ] Update client applications with new key
- [ ] Revoke old key
- [ ] Audit log the rotation event
- [ ] Update documentation if needed

### Emergency Key Rotation

In case of a suspected key compromise:

1. **Immediately revoke the compromised key**
   ```bash
   # Revoke immediately - this will cause brief downtime
   kubectl delete secret llm-api-keys -n ai-platform
   ```

2. **Generate and deploy new key**
   ```bash
   # Create new secret immediately
   kubectl create secret generic llm-api-keys \
     --from-literal=openai-api-key="${NEW_OPENAI_KEY}" \
     -n ai-platform

   # Force restart
   kubectl rollout restart deployment --all -n ai-platform
   ```

3. **Investigate the breach**
   - Review access logs
   - Check for unauthorized API usage
   - Identify the source of compromise

4. **Report and document**
   - File incident report
   - Update security procedures if needed

### LLM Provider Key Rotation

#### OpenAI API Key

1. Go to [OpenAI Platform](https://platform.openai.com/api-keys)
2. Create new API key
3. Update secrets in your infrastructure
4. Deploy changes
5. Delete old key from OpenAI dashboard

#### Anthropic API Key

1. Go to [Anthropic Console](https://console.anthropic.com/)
2. Generate new API key
3. Update secrets in your infrastructure
4. Deploy changes
5. Revoke old key from Anthropic console

---

## Data Protection

### Data Classification

| Level | Description | Examples |
|-------|-------------|----------|
| **Public** | Non-sensitive data | API documentation |
| **Internal** | Business-sensitive | Usage metrics |
| **Confidential** | Requires protection | User prompts, API keys |
| **Restricted** | Highly sensitive | PII, credentials |

### Encryption

#### At Rest

- Database encryption: AES-256
- File storage: AES-256
- Backup encryption: AES-256-GCM

#### In Transit

- TLS 1.3 required for all connections
- Minimum cipher suite: TLS_AES_256_GCM_SHA384
- Certificate: Let's Encrypt or enterprise CA

### Data Retention

| Data Type | Retention Period | Deletion Method |
|-----------|------------------|-----------------|
| API Logs | 90 days | Automatic purge |
| User Prompts | 30 days | Secure deletion |
| Uploaded Documents | User-defined | On-demand deletion |
| Metrics | 1 year | Aggregation + deletion |

---

## Network Security

### Network Policies

The platform implements strict network segmentation:

```yaml
# Default deny all ingress
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-ingress
spec:
  podSelector: {}
  policyTypes:
    - Ingress
```

### Allowed Traffic Flows

| Source | Destination | Port | Purpose |
|--------|-------------|------|---------|
| Ingress Controller | Gateway | 8080 | API traffic |
| Ingress Controller | Dashboard | 3000 | Web UI |
| Gateway | RAG Service | 8000 | Internal API |
| Gateway | Redis | 6379 | Caching |
| RAG Service | PostgreSQL | 5432 | Metadata |
| RAG Service | Qdrant | 6333 | Vectors |

### Rate Limiting

| Endpoint | Limit | Window |
|----------|-------|--------|
| `/v1/chat/completions` | 100 req | 1 minute |
| `/v1/embeddings` | 200 req | 1 minute |
| `/v1/documents` | 50 req | 1 minute |

---

## Container Security

### Security Contexts

All containers run with restricted security contexts:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
```

### Image Security

- Base images: Official distroless or slim variants
- Image scanning: Trivy/Clair in CI/CD
- No root user in containers
- Minimal attack surface

### Pod Security Standards

The platform enforces the "restricted" Pod Security Standard:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ai-platform
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

---

## Secrets Management

### Production Recommendations

1. **HashiCorp Vault** (Recommended)
   ```yaml
   annotations:
     vault.hashicorp.com/agent-inject: "true"
     vault.hashicorp.com/role: "ai-platform"
     vault.hashicorp.com/agent-inject-secret-api-keys: "secret/ai-platform/api-keys"
   ```

2. **AWS Secrets Manager + External Secrets Operator**
   ```yaml
   apiVersion: external-secrets.io/v1beta1
   kind: ExternalSecret
   metadata:
     name: llm-api-keys
   spec:
     refreshInterval: 1h
     secretStoreRef:
       name: aws-secrets-manager
       kind: ClusterSecretStore
     target:
       name: llm-api-keys
     data:
       - secretKey: openai-api-key
         remoteRef:
           key: ai-platform/api-keys
           property: openai-api-key
   ```

3. **GCP Secret Manager**
   ```yaml
   apiVersion: external-secrets.io/v1beta1
   kind: ExternalSecret
   spec:
     secretStoreRef:
       name: gcp-secret-manager
       kind: ClusterSecretStore
   ```

### Secret Rotation Schedule

| Secret Type | Rotation Frequency |
|-------------|-------------------|
| LLM API Keys | Every 90 days |
| Database Passwords | Every 30 days |
| TLS Certificates | Before expiry (auto) |
| Internal Service Keys | Every 60 days |

---

## Audit Logging

### Logged Events

| Event Category | Examples |
|----------------|----------|
| Authentication | Login, logout, API key usage |
| Authorization | Access denied, permission changes |
| Data Access | Document upload, retrieval |
| Administration | User management, config changes |
| Security | Rate limit hits, suspicious activity |

### Log Format

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "info",
  "event": "api_request",
  "user_id": "user_123",
  "api_key_prefix": "sk-ai-xxxx",
  "endpoint": "/v1/chat/completions",
  "method": "POST",
  "status_code": 200,
  "response_time_ms": 1523,
  "ip_address": "10.0.0.1",
  "user_agent": "python-requests/2.31.0"
}
```

### Log Retention

- **Hot storage:** 7 days (Elasticsearch/Loki)
- **Warm storage:** 30 days (compressed)
- **Cold storage:** 1 year (archived)

---

## Incident Response

### Severity Levels

| Level | Description | Response Time |
|-------|-------------|---------------|
| **P1 - Critical** | System down, data breach | 15 minutes |
| **P2 - High** | Major functionality impacted | 1 hour |
| **P3 - Medium** | Minor functionality impacted | 4 hours |
| **P4 - Low** | Cosmetic or minor issues | 24 hours |

### Response Procedure

1. **Detection & Triage**
   - Identify the incident
   - Assess severity
   - Notify incident response team

2. **Containment**
   - Isolate affected systems
   - Preserve evidence
   - Prevent further damage

3. **Eradication**
   - Remove threat
   - Patch vulnerabilities
   - Update security controls

4. **Recovery**
   - Restore services
   - Verify functionality
   - Monitor for recurrence

5. **Post-Incident**
   - Document lessons learned
   - Update procedures
   - Implement improvements

---

## Compliance

The AI Platform is designed to support compliance with:

- **SOC 2 Type II** - Security, Availability, Confidentiality
- **GDPR** - Data protection and privacy
- **HIPAA** - Healthcare data protection (with additional configuration)
- **PCI DSS** - Payment card data (if applicable)

---

## Security Checklist

### Pre-Deployment

- [ ] All secrets stored in secrets manager
- [ ] TLS certificates configured
- [ ] Network policies applied
- [ ] Pod security standards enforced
- [ ] RBAC configured with least privilege
- [ ] Image scanning completed
- [ ] Penetration testing performed

### Ongoing

- [ ] Regular secret rotation
- [ ] Dependency updates
- [ ] Security patch management
- [ ] Log monitoring and alerting
- [ ] Access review (quarterly)
- [ ] Incident response drills

---

## Changelog

| Date | Version | Changes |
|------|---------|---------|
| 2025-12-28 | 1.0.0 | Initial security policy |

---

*Last updated: 2025-12-28*
*Document owner: Security Team*

/**
 * API Client for RAG Agent Service
 */

import type {
  AgentRequest,
  AgentResponse,
  Agent,
  Document,
  ChatMessage,
  UsageMetrics,
  Model,
  AgentLog,
} from '@/types';
import { getApiBaseUrl, parseSSEData } from './utils';

const API_BASE_URL = getApiBaseUrl();

/**
 * Custom error for API requests
 */
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public details?: Record<string, unknown>
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

/**
 * Base fetch wrapper with error handling
 */
async function fetchApi<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...options.headers,
  };

  try {
    const response = await fetch(url, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      throw new ApiError(
        response.status,
        errorData.code || 'UNKNOWN_ERROR',
        errorData.detail || errorData.message || 'Request failed',
        errorData.details
      );
    }

    return response.json();
  } catch (error) {
    if (error instanceof ApiError) throw error;
    throw new ApiError(0, 'NETWORK_ERROR', 'Network request failed');
  }
}

// ============ Health API ============

export async function checkHealth(): Promise<{ status: string; version: string }> {
  return fetchApi('/health');
}

export async function checkReady(): Promise<{ status: string; checks: Record<string, boolean> }> {
  return fetchApi('/ready');
}

// ============ Documents API ============

export async function listDocuments(): Promise<{ documents: Document[] }> {
  return fetchApi('/api/v1/documents/');
}

export async function uploadDocument(file: File): Promise<Document> {
  const formData = new FormData();
  formData.append('file', file);

  const response = await fetch(`${API_BASE_URL}/api/v1/documents/upload`, {
    method: 'POST',
    body: formData,
  });

  if (!response.ok) {
    const errorData = await response.json().catch(() => ({}));
    throw new ApiError(
      response.status,
      errorData.code || 'UPLOAD_ERROR',
      errorData.detail || 'Failed to upload document'
    );
  }

  return response.json();
}

export async function deleteDocument(documentId: string): Promise<{ deleted_chunks: number }> {
  return fetchApi(`/api/v1/documents/${documentId}`, {
    method: 'DELETE',
  });
}

export async function getDocumentStatus(documentId: string): Promise<Document> {
  return fetchApi(`/api/v1/documents/${documentId}/status`);
}

// ============ Chat API ============

export async function sendChatMessage(
  message: string,
  conversationId?: string
): Promise<{ response: string; sources: ChatMessage['sources'] }> {
  return fetchApi('/api/v1/chat/', {
    method: 'POST',
    body: JSON.stringify({
      message,
      conversation_id: conversationId,
    }),
  });
}

export async function* streamChatMessage(
  message: string,
  conversationId?: string
): AsyncGenerator<{ content: string; done: boolean }> {
  const response = await fetch(`${API_BASE_URL}/api/v1/chat/stream`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      message,
      conversation_id: conversationId,
    }),
  });

  if (!response.ok) {
    throw new ApiError(
      response.status,
      'STREAM_ERROR',
      'Failed to start chat stream'
    );
  }

  const reader = response.body?.getReader();
  if (!reader) throw new ApiError(0, 'STREAM_ERROR', 'No response body');

  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();

    if (done) {
      yield { content: '', done: true };
      break;
    }

    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';

    for (const line of lines) {
      if (line.startsWith('data: ')) {
        const data = line.slice(6);
        if (data === '[DONE]') {
          yield { content: '', done: true };
          return;
        }
        const parsed = parseSSEData<{ content: string }>(data);
        if (parsed) {
          yield { content: parsed.content, done: false };
        }
      }
    }
  }
}

// ============ Agents API ============

export async function listAgentTypes(): Promise<{ agents: Agent[] }> {
  return fetchApi('/api/v1/agents/types');
}

export async function runAgent(request: AgentRequest): Promise<AgentResponse> {
  return fetchApi('/api/v1/agents/run', {
    method: 'POST',
    body: JSON.stringify({
      question: request.question,
      agent_type: request.agentType,
      model: request.model,
      temperature: request.temperature,
      max_iterations: request.maxIterations,
      stream: false,
    }),
  });
}

export async function* streamAgentExecution(
  request: AgentRequest
): AsyncGenerator<AgentLog | AgentResponse> {
  const response = await fetch(`${API_BASE_URL}/api/v1/agents/run`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      question: request.question,
      agent_type: request.agentType,
      model: request.model,
      temperature: request.temperature,
      max_iterations: request.maxIterations,
      stream: true,
    }),
  });

  if (!response.ok) {
    throw new ApiError(
      response.status,
      'AGENT_ERROR',
      'Failed to start agent execution'
    );
  }

  const reader = response.body?.getReader();
  if (!reader) throw new ApiError(0, 'STREAM_ERROR', 'No response body');

  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();

    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';

    for (const line of lines) {
      if (line.startsWith('data: ')) {
        const data = line.slice(6);
        if (data === '[DONE]') return;

        const parsed = parseSSEData<AgentLog | AgentResponse>(data);
        if (parsed) yield parsed;
      }
    }
  }
}

export async function runRagAgent(question: string): Promise<AgentResponse> {
  return fetchApi('/api/v1/agents/rag', {
    method: 'POST',
    body: JSON.stringify(question),
  });
}

export async function runResearchAgent(question: string): Promise<AgentResponse> {
  return fetchApi('/api/v1/agents/research', {
    method: 'POST',
    body: JSON.stringify(question),
  });
}

// ============ Analytics API ============

export async function getUsageMetrics(): Promise<UsageMetrics> {
  // Note: This would need to be implemented in the backend
  // For now, returning mock data
  return {
    totalRequests: 1250,
    totalTokens: 450000,
    requestsByAgent: {
      rag: 800,
      research: 250,
      data_entry: 100,
      support_triage: 50,
      report: 50,
    },
    requestsByDay: [
      { date: '2024-12-22', requests: 150, tokens: 55000, avgResponseTime: 1.2 },
      { date: '2024-12-23', requests: 180, tokens: 65000, avgResponseTime: 1.1 },
      { date: '2024-12-24', requests: 120, tokens: 45000, avgResponseTime: 1.3 },
      { date: '2024-12-25', requests: 90, tokens: 35000, avgResponseTime: 1.0 },
      { date: '2024-12-26', requests: 200, tokens: 75000, avgResponseTime: 1.4 },
      { date: '2024-12-27', requests: 250, tokens: 90000, avgResponseTime: 1.2 },
      { date: '2024-12-28', requests: 260, tokens: 95000, avgResponseTime: 1.1 },
    ],
    averageResponseTime: 1.2,
    successRate: 0.98,
  };
}

export async function getModelUsage(): Promise<Model[]> {
  // Note: This would need to be implemented in the backend
  return [
    { id: 'gpt-4o-mini', name: 'GPT-4o Mini', provider: 'OpenAI', contextLength: 128000, inputCost: 0.15, outputCost: 0.6 },
    { id: 'gpt-4o', name: 'GPT-4o', provider: 'OpenAI', contextLength: 128000, inputCost: 5.0, outputCost: 15.0 },
    { id: 'claude-3-5-sonnet', name: 'Claude 3.5 Sonnet', provider: 'Anthropic', contextLength: 200000, inputCost: 3.0, outputCost: 15.0 },
  ];
}

// ============ Gateway API ============

export async function checkGatewayHealth(): Promise<boolean> {
  try {
    const response = await checkHealth();
    return response.status === 'healthy';
  } catch {
    return false;
  }
}

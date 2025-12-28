/**
 * Type validation tests
 * These tests ensure our types are correctly defined and can be used properly
 */

import type {
  ChatMessage,
  ChatSession,
  Document,
  DocumentStatus,
  Agent,
  AgentStatus,
  AgentRequest,
  AgentResponse,
  AgentExecution,
  AgentLog,
  Source,
  UsageMetrics,
  DailyMetric,
  ModelUsage,
  ApiResponse,
  AppSettings,
  Model,
} from '../types';

describe('ChatMessage type', () => {
  it('should accept valid user message', () => {
    const message: ChatMessage = {
      id: '123',
      role: 'user',
      content: 'Hello',
      timestamp: new Date(),
    };
    expect(message.role).toBe('user');
    expect(message.content).toBe('Hello');
  });

  it('should accept valid assistant message with sources', () => {
    const message: ChatMessage = {
      id: '456',
      role: 'assistant',
      content: 'Response',
      timestamp: '2024-01-01T00:00:00Z',
      sources: [
        { content: 'Source content', metadata: { page: 1 } },
      ],
      isStreaming: true,
    };
    expect(message.role).toBe('assistant');
    expect(message.sources).toHaveLength(1);
    expect(message.isStreaming).toBe(true);
  });

  it('should accept system message', () => {
    const message: ChatMessage = {
      id: '789',
      role: 'system',
      content: 'System instruction',
      timestamp: new Date(),
    };
    expect(message.role).toBe('system');
  });
});

describe('ChatSession type', () => {
  it('should accept valid session', () => {
    const session: ChatSession = {
      id: 'session-1',
      title: 'Test Chat',
      messages: [],
      model: 'gpt-4o-mini',
      createdAt: new Date(),
      updatedAt: new Date(),
    };
    expect(session.id).toBe('session-1');
    expect(session.messages).toHaveLength(0);
  });
});

describe('Document type', () => {
  it('should accept valid document', () => {
    const doc: Document = {
      id: 'doc-1',
      name: 'Test Document',
      type: 'pdf',
      size: 1024,
      status: 'ready',
      uploadedAt: new Date(),
    };
    expect(doc.status).toBe('ready');
  });

  it('should accept all document statuses', () => {
    const statuses: DocumentStatus[] = ['pending', 'processing', 'ready', 'error'];
    statuses.forEach(status => {
      const doc: Document = {
        id: '1',
        name: 'Doc',
        type: 'pdf',
        size: 100,
        status,
        uploadedAt: new Date(),
      };
      expect(doc.status).toBe(status);
    });
  });
});

describe('Agent type', () => {
  it('should accept valid agent', () => {
    const agent: Agent = {
      id: 'agent-1',
      type: 'rag',
      name: 'RAG Agent',
      description: 'Retrieval augmented generation',
      capabilities: ['search', 'summarize'],
      status: 'active',
    };
    expect(agent.type).toBe('rag');
    expect(agent.capabilities).toContain('search');
  });

  it('should accept all agent statuses', () => {
    const statuses: AgentStatus[] = ['active', 'inactive', 'running', 'error'];
    statuses.forEach(status => {
      const agent: Agent = {
        id: '1',
        type: 'rag',
        name: 'Test',
        description: 'Test agent',
        status,
      };
      expect(agent.status).toBe(status);
    });
  });
});

describe('AgentRequest type', () => {
  it('should accept minimal request', () => {
    const request: AgentRequest = {
      question: 'What is the answer?',
    };
    expect(request.question).toBeTruthy();
  });

  it('should accept full request', () => {
    const request: AgentRequest = {
      question: 'What is the answer?',
      agentType: 'rag',
      model: 'gpt-4o-mini',
      temperature: 0.7,
      maxIterations: 5,
      stream: true,
    };
    expect(request.agentType).toBe('rag');
    expect(request.stream).toBe(true);
  });
});

describe('AgentResponse type', () => {
  it('should accept successful response', () => {
    const response: AgentResponse = {
      answer: 'The answer is 42',
      sources: [],
      iterations: 3,
      success: true,
    };
    expect(response.success).toBe(true);
    expect(response.iterations).toBe(3);
  });

  it('should accept failed response', () => {
    const response: AgentResponse = {
      answer: '',
      sources: [],
      iterations: 1,
      success: false,
      error: 'Something went wrong',
    };
    expect(response.success).toBe(false);
    expect(response.error).toBeTruthy();
  });
});

describe('AgentExecution type', () => {
  it('should accept execution with logs', () => {
    const log: AgentLog = {
      id: 'log-1',
      executionId: 'exec-1',
      timestamp: new Date(),
      level: 'info',
      message: 'Starting execution',
      step: 'init',
      iteration: 1,
    };

    const execution: AgentExecution = {
      id: 'exec-1',
      agentType: 'rag',
      status: 'running',
      startedAt: new Date(),
      logs: [log],
    };

    expect(execution.status).toBe('running');
    expect(execution.logs).toHaveLength(1);
  });
});

describe('UsageMetrics type', () => {
  it('should accept valid metrics', () => {
    const dailyMetric: DailyMetric = {
      date: '2024-01-01',
      requests: 100,
      tokens: 5000,
      avgResponseTime: 250,
    };

    const metrics: UsageMetrics = {
      totalRequests: 1000,
      totalTokens: 50000,
      requestsByAgent: { rag: 500, research: 500 },
      requestsByDay: [dailyMetric],
      averageResponseTime: 200,
      successRate: 0.95,
    };

    expect(metrics.successRate).toBe(0.95);
    expect(metrics.requestsByDay).toHaveLength(1);
  });
});

describe('ModelUsage type', () => {
  it('should accept valid model usage', () => {
    const usage: ModelUsage = {
      model: 'gpt-4o-mini',
      requests: 100,
      tokens: 10000,
      cost: 0.50,
    };
    expect(usage.cost).toBe(0.50);
  });
});

describe('ApiResponse type', () => {
  it('should accept successful response', () => {
    const response: ApiResponse<{ name: string }> = {
      data: { name: 'test' },
    };
    expect(response.data?.name).toBe('test');
  });

  it('should accept error response', () => {
    const response: ApiResponse<unknown> = {
      error: {
        code: 'NOT_FOUND',
        message: 'Resource not found',
      },
    };
    expect(response.error?.code).toBe('NOT_FOUND');
  });
});

describe('AppSettings type', () => {
  it('should accept valid settings', () => {
    const settings: AppSettings = {
      theme: 'dark',
      defaultModel: 'gpt-4o-mini',
      defaultTemperature: 0.7,
      streamingEnabled: true,
      apiUrl: 'http://localhost:8000',
    };
    expect(settings.theme).toBe('dark');
    expect(settings.streamingEnabled).toBe(true);
  });
});

describe('Model type', () => {
  it('should accept valid model', () => {
    const model: Model = {
      id: 'gpt-4o-mini',
      name: 'GPT-4 Mini',
      provider: 'openai',
      contextLength: 128000,
      inputCost: 0.15,
      outputCost: 0.60,
    };
    expect(model.provider).toBe('openai');
    expect(model.contextLength).toBe(128000);
  });
});

describe('Source type', () => {
  it('should accept source with score', () => {
    const source: Source = {
      content: 'This is the source content',
      metadata: { page: 1, section: 'intro' },
      score: 0.95,
    };
    expect(source.score).toBe(0.95);
    expect(source.metadata.page).toBe(1);
  });
});

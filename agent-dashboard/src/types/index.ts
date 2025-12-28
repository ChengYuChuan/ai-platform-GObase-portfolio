/**
 * Type definitions for Agent Dashboard
 */

// Chat Types
export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp: string | Date;
  sources?: Source[];
  isStreaming?: boolean;
  metadata?: Record<string, unknown>;
}

export interface Source {
  content: string;
  metadata: Record<string, unknown>;
  score?: number;
}

export interface ChatSession {
  id: string;
  title: string;
  messages: ChatMessage[];
  model: string;
  createdAt: string | Date;
  updatedAt: string | Date;
}

// Document Types
export interface Document {
  id: string;
  name: string;
  filename?: string;
  type: string;
  contentType?: string;
  size: number;
  chunkCount?: number;
  chunksCount?: number;
  status: DocumentStatus;
  uploadedAt: string | Date;
  processedAt?: string | Date;
  metadata?: Record<string, unknown>;
}

export type DocumentStatus = 'pending' | 'processing' | 'ready' | 'error';

export interface DocumentUploadProgress {
  documentId: string;
  progress: number;
  status: DocumentStatus;
  error?: string;
}

// Agent Types
export interface Agent {
  id: string;
  type: string;
  name: string;
  description: string;
  capabilities?: string[];
  defaultIterations?: number;
  status: AgentStatus;
  config?: Record<string, unknown>;
  lastRun?: string | Date;
}

export type AgentType = 'rag' | 'research' | 'code' | 'summarizer' | 'data_entry' | 'support_triage' | 'report';

export type AgentStatus = 'active' | 'inactive' | 'running' | 'error';

export interface AgentRequest {
  question: string;
  agentType?: string;
  model?: string;
  temperature?: number;
  maxIterations?: number;
  stream?: boolean;
}

export interface AgentResponse {
  id?: string;
  agentType?: string;
  answer: string;
  sources: Source[];
  iterations: number;
  success: boolean;
  error?: string;
  metadata?: Record<string, unknown>;
}

export interface AgentExecution {
  id: string;
  agentType: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  startedAt: string | Date;
  endedAt?: string | Date;
  input?: Record<string, unknown>;
  result?: AgentResponse;
  error?: string;
  logs?: AgentLog[];
}

export interface AgentLog {
  id: string;
  executionId: string;
  timestamp: string | Date;
  level: 'info' | 'warning' | 'error' | 'success';
  message: string;
  step?: string;
  iteration?: number;
  metadata?: Record<string, unknown>;
}

// Analytics Types
export interface UsageMetrics {
  totalRequests: number;
  totalTokens: number;
  requestsByAgent: Record<string, number>;
  requestsByDay: DailyMetric[];
  averageResponseTime: number;
  successRate: number;
}

export interface DailyMetric {
  date: string;
  requests: number;
  tokens: number;
  avgResponseTime: number;
}

export interface ModelUsage {
  model: string;
  requests: number;
  tokens: number;
  cost: number;
}

// API Response Types
export interface ApiResponse<T> {
  data?: T;
  error?: ApiError;
}

export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

// Settings Types
export interface AppSettings {
  theme: 'light' | 'dark' | 'system';
  defaultModel: string;
  defaultTemperature: number;
  streamingEnabled: boolean;
  apiUrl: string;
}

// Model Types
export interface Model {
  id: string;
  name: string;
  provider: string;
  contextLength: number;
  inputCost: number;
  outputCost: number;
}

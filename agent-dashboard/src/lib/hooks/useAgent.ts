'use client';

import { useState, useCallback, useRef } from 'react';
import { create } from 'zustand';
import type {
  Agent,
  AgentType,
  AgentRequest,
  AgentResponse,
  AgentExecution,
  AgentLog,
  AgentStatus,
} from '@/types';
import { listAgentTypes, runAgent, streamAgentExecution } from '@/lib/api';
import { generateId } from '@/lib/utils';

/**
 * Agent store for managing agent state
 */
interface AgentStore {
  agents: Agent[];
  executions: AgentExecution[];
  loadAgents: () => Promise<void>;
  addExecution: (execution: AgentExecution) => void;
  updateExecution: (id: string, updates: Partial<AgentExecution>) => void;
  addLog: (executionId: string, log: AgentLog) => void;
  clearExecutions: () => void;
}

export const useAgentStore = create<AgentStore>((set) => ({
  agents: [],
  executions: [],

  loadAgents: async () => {
    try {
      const response = await listAgentTypes();
      set({
        agents: response.agents.map((agent) => ({
          ...agent,
          status: 'idle' as AgentStatus,
        })),
      });
    } catch (error) {
      console.error('Failed to load agents:', error);
    }
  },

  addExecution: (execution) => {
    set((state) => ({
      executions: [execution, ...state.executions],
    }));
  },

  updateExecution: (id, updates) => {
    set((state) => ({
      executions: state.executions.map((exec) =>
        exec.id === id ? { ...exec, ...updates } : exec
      ),
    }));
  },

  addLog: (executionId, log) => {
    set((state) => ({
      executions: state.executions.map((exec) =>
        exec.id === executionId
          ? { ...exec, logs: [...(exec.logs || []), log] }
          : exec
      ),
    }));
  },

  clearExecutions: () => {
    set({ executions: [] });
  },
}));

/**
 * Hook for running agents
 */
export function useAgent() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  const store = useAgentStore();

  const executeAgent = useCallback(
    async (
      request: AgentRequest,
      streaming = true
    ): Promise<AgentResponse | null> => {
      setIsLoading(true);
      setError(null);

      const executionId = generateId();
      const execution: AgentExecution = {
        id: executionId,
        agentType: request.agentType || 'rag',
        status: 'running',
        startedAt: new Date().toISOString(),
        input: { question: request.question },
        logs: [],
      };
      store.addExecution(execution);

      try {
        if (streaming) {
          let result: AgentResponse | null = null;

          for await (const event of streamAgentExecution(request)) {
            // Check if it's a log event or final response
            if ('event' in event) {
              const log = event as AgentLog;
              store.addLog(executionId, log);
            } else {
              result = event as AgentResponse;
            }
          }

          store.updateExecution(executionId, {
            status: result?.success ? 'completed' : 'failed',
            endedAt: new Date().toISOString(),
            result: result || undefined,
          });

          setIsLoading(false);
          return result;
        } else {
          const result = await runAgent(request);

          store.updateExecution(executionId, {
            status: result.success ? 'completed' : 'failed',
            endedAt: new Date().toISOString(),
            result,
          });

          setIsLoading(false);
          return result;
        }
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Agent execution failed';
        setError(errorMessage);

        store.updateExecution(executionId, {
          status: 'failed',
          endedAt: new Date().toISOString(),
          error: errorMessage,
        });

        setIsLoading(false);
        return null;
      }
    },
    [store]
  );

  const stopExecution = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
  }, []);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  return {
    agents: store.agents,
    executions: store.executions,
    isLoading,
    error,
    executeAgent,
    stopExecution,
    clearError,
    loadAgents: store.loadAgents,
    clearExecutions: store.clearExecutions,
  };
}

/**
 * Hook for agent status polling
 */
export function useAgentStatus(agentType: AgentType) {
  const store = useAgentStore();
  const agent = store.agents.find((a) => a.type === agentType);
  const recentExecutions = store.executions.filter(
    (e) => e.agentType === agentType
  );

  return {
    agent,
    recentExecutions,
    isRunning: recentExecutions.some((e) => e.status === 'running'),
  };
}

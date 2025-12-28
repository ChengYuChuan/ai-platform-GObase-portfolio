'use client';

import { useState, useCallback, useRef } from 'react';
import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { ChatMessage, ChatSession } from '@/types';
import { streamChatMessage, sendChatMessage } from '@/lib/api';
import { generateId } from '@/lib/utils';

/**
 * Chat store for managing chat sessions
 */
interface ChatStore {
  sessions: ChatSession[];
  activeSessionId: string | null;
  createSession: (model?: string) => string;
  setActiveSession: (id: string) => void;
  addMessage: (sessionId: string, message: ChatMessage) => void;
  updateMessage: (sessionId: string, messageId: string, updates: Partial<ChatMessage>) => void;
  deleteSession: (id: string) => void;
  clearAllSessions: () => void;
}

export const useChatStore = create<ChatStore>()(
  persist(
    (set) => ({
      sessions: [],
      activeSessionId: null,

      createSession: (model = 'gpt-4o-mini') => {
        const id = generateId();
        const session: ChatSession = {
          id,
          title: 'New Chat',
          messages: [],
          model,
          createdAt: new Date(),
          updatedAt: new Date(),
        };
        set((state) => ({
          sessions: [session, ...state.sessions],
          activeSessionId: id,
        }));
        return id;
      },

      setActiveSession: (id) => {
        set({ activeSessionId: id });
      },

      addMessage: (sessionId, message) => {
        set((state) => ({
          sessions: state.sessions.map((session) => {
            if (session.id !== sessionId) return session;
            const messages = [...session.messages, message];
            // Update title based on first user message
            const title =
              session.messages.length === 0 && message.role === 'user'
                ? message.content.slice(0, 50) + (message.content.length > 50 ? '...' : '')
                : session.title;
            return {
              ...session,
              messages,
              title,
              updatedAt: new Date(),
            };
          }),
        }));
      },

      updateMessage: (sessionId, messageId, updates) => {
        set((state) => ({
          sessions: state.sessions.map((session) => {
            if (session.id !== sessionId) return session;
            return {
              ...session,
              messages: session.messages.map((msg) =>
                msg.id === messageId ? { ...msg, ...updates } : msg
              ),
              updatedAt: new Date(),
            };
          }),
        }));
      },

      deleteSession: (id) => {
        set((state) => {
          const sessions = state.sessions.filter((s) => s.id !== id);
          const activeSessionId =
            state.activeSessionId === id
              ? sessions.length > 0
                ? sessions[0].id
                : null
              : state.activeSessionId;
          return { sessions, activeSessionId };
        });
      },

      clearAllSessions: () => {
        set({ sessions: [], activeSessionId: null });
      },
    }),
    {
      name: 'chat-storage',
      partialize: (state) => ({
        sessions: state.sessions.map((s) => ({
          ...s,
          createdAt: typeof s.createdAt === 'string' ? s.createdAt : s.createdAt.toISOString(),
          updatedAt: typeof s.updatedAt === 'string' ? s.updatedAt : s.updatedAt.toISOString(),
          messages: s.messages.map((m) => ({
            ...m,
            timestamp: typeof m.timestamp === 'string' ? m.timestamp : m.timestamp.toISOString(),
          })),
        })),
        activeSessionId: state.activeSessionId,
      }),
    }
  )
);

/**
 * Hook for managing chat interactions
 */
export function useChat(sessionId?: string) {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  const store = useChatStore();
  const currentSessionId = sessionId || store.activeSessionId;
  const currentSession = store.sessions.find((s) => s.id === currentSessionId);

  const sendMessage = useCallback(
    async (content: string, streaming = true) => {
      const sessId = currentSessionId || store.createSession();

      setIsLoading(true);
      setError(null);

      // Add user message
      const userMessage: ChatMessage = {
        id: generateId(),
        role: 'user',
        content,
        timestamp: new Date(),
      };
      store.addMessage(sessId, userMessage);

      // Create placeholder for assistant message
      const assistantMessageId = generateId();
      const assistantMessage: ChatMessage = {
        id: assistantMessageId,
        role: 'assistant',
        content: '',
        timestamp: new Date(),
        isStreaming: streaming,
      };
      store.addMessage(sessId, assistantMessage);

      try {
        if (streaming) {
          let fullContent = '';
          for await (const chunk of streamChatMessage(content, sessId)) {
            if (chunk.done) break;
            fullContent += chunk.content;
            store.updateMessage(sessId, assistantMessageId, {
              content: fullContent,
            });
          }
          store.updateMessage(sessId, assistantMessageId, {
            isStreaming: false,
          });
        } else {
          const response = await sendChatMessage(content, sessId);
          store.updateMessage(sessId, assistantMessageId, {
            content: response.response,
            sources: response.sources,
            isStreaming: false,
          });
        }
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Failed to send message';
        setError(errorMessage);
        store.updateMessage(sessId, assistantMessageId, {
          content: `Error: ${errorMessage}`,
          isStreaming: false,
        });
      } finally {
        setIsLoading(false);
      }
    },
    [currentSessionId, store]
  );

  const stopGeneration = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
  }, []);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  return {
    messages: currentSession?.messages || [],
    isLoading,
    error,
    sendMessage,
    stopGeneration,
    clearError,
    createSession: store.createSession,
    setActiveSession: store.setActiveSession,
    deleteSession: store.deleteSession,
    sessions: store.sessions,
    activeSessionId: store.activeSessionId,
  };
}

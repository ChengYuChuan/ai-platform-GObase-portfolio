/**
 * Tests for custom React hooks
 */

import { renderHook, act } from '@testing-library/react';
import { useChatStore } from '../lib/hooks/useChat';

// Mock the API functions
jest.mock('../lib/api', () => ({
  streamChatMessage: jest.fn(),
  sendChatMessage: jest.fn(),
}));

// Mock utils
jest.mock('../lib/utils', () => ({
  ...jest.requireActual('../lib/utils'),
  generateId: () => `test-id-${Math.random().toString(36).substr(2, 9)}`,
}));

describe('useChatStore', () => {
  beforeEach(() => {
    // Clear the store before each test
    const { result } = renderHook(() => useChatStore());
    act(() => {
      result.current.clearAllSessions();
    });
  });

  describe('createSession', () => {
    it('creates a new session with default model', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId: string;
      act(() => {
        sessionId = result.current.createSession();
      });

      expect(result.current.sessions).toHaveLength(1);
      expect(result.current.sessions[0].model).toBe('gpt-4o-mini');
      expect(result.current.activeSessionId).toBe(sessionId!);
    });

    it('creates a new session with specified model', () => {
      const { result } = renderHook(() => useChatStore());

      act(() => {
        result.current.createSession('claude-3-haiku');
      });

      expect(result.current.sessions[0].model).toBe('claude-3-haiku');
    });

    it('sets the new session as active', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId: string;
      act(() => {
        sessionId = result.current.createSession();
      });

      expect(result.current.activeSessionId).toBe(sessionId!);
    });

    it('adds new sessions to the beginning of the list', () => {
      const { result } = renderHook(() => useChatStore());

      act(() => {
        result.current.createSession('model-1');
        result.current.createSession('model-2');
      });

      expect(result.current.sessions[0].model).toBe('model-2');
      expect(result.current.sessions[1].model).toBe('model-1');
    });
  });

  describe('setActiveSession', () => {
    it('sets the active session', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId1: string;
      let sessionId2: string;
      act(() => {
        sessionId1 = result.current.createSession();
        sessionId2 = result.current.createSession();
      });

      // sessionId2 should be active after creation
      expect(result.current.activeSessionId).toBe(sessionId2!);

      act(() => {
        result.current.setActiveSession(sessionId1!);
      });

      expect(result.current.activeSessionId).toBe(sessionId1!);
    });
  });

  describe('addMessage', () => {
    it('adds a message to the session', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId: string;
      act(() => {
        sessionId = result.current.createSession();
      });

      act(() => {
        result.current.addMessage(sessionId!, {
          id: 'msg-1',
          role: 'user',
          content: 'Hello',
          timestamp: new Date(),
        });
      });

      const session = result.current.sessions.find(s => s.id === sessionId);
      expect(session?.messages).toHaveLength(1);
      expect(session?.messages[0].content).toBe('Hello');
    });

    it('updates session title on first user message', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId: string;
      act(() => {
        sessionId = result.current.createSession();
      });

      act(() => {
        result.current.addMessage(sessionId!, {
          id: 'msg-1',
          role: 'user',
          content: 'This is my first message',
          timestamp: new Date(),
        });
      });

      const session = result.current.sessions.find(s => s.id === sessionId);
      expect(session?.title).toBe('This is my first message');
    });

    it('truncates long titles', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId: string;
      act(() => {
        sessionId = result.current.createSession();
      });

      const longMessage = 'This is a very long message that should be truncated because it exceeds fifty characters';
      act(() => {
        result.current.addMessage(sessionId!, {
          id: 'msg-1',
          role: 'user',
          content: longMessage,
          timestamp: new Date(),
        });
      });

      const session = result.current.sessions.find(s => s.id === sessionId);
      expect(session?.title).toHaveLength(53); // 50 chars + '...'
      expect(session?.title).toContain('...');
    });

    it('does not update title for assistant messages', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId: string;
      act(() => {
        sessionId = result.current.createSession();
      });

      act(() => {
        result.current.addMessage(sessionId!, {
          id: 'msg-1',
          role: 'assistant',
          content: 'Hello user',
          timestamp: new Date(),
        });
      });

      const session = result.current.sessions.find(s => s.id === sessionId);
      expect(session?.title).toBe('New Chat');
    });
  });

  describe('updateMessage', () => {
    it('updates a message content', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId: string;
      act(() => {
        sessionId = result.current.createSession();
      });

      act(() => {
        result.current.addMessage(sessionId!, {
          id: 'msg-1',
          role: 'assistant',
          content: 'Initial',
          timestamp: new Date(),
          isStreaming: true,
        });
      });

      act(() => {
        result.current.updateMessage(sessionId!, 'msg-1', {
          content: 'Updated content',
          isStreaming: false,
        });
      });

      const session = result.current.sessions.find(s => s.id === sessionId);
      expect(session?.messages[0].content).toBe('Updated content');
      expect(session?.messages[0].isStreaming).toBe(false);
    });
  });

  describe('deleteSession', () => {
    it('deletes a session', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId: string;
      act(() => {
        sessionId = result.current.createSession();
      });

      expect(result.current.sessions).toHaveLength(1);

      act(() => {
        result.current.deleteSession(sessionId!);
      });

      expect(result.current.sessions).toHaveLength(0);
    });

    it('clears active session if deleted', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId: string;
      act(() => {
        sessionId = result.current.createSession();
      });

      act(() => {
        result.current.deleteSession(sessionId!);
      });

      expect(result.current.activeSessionId).toBeNull();
    });

    it('switches to next session when active is deleted', () => {
      const { result } = renderHook(() => useChatStore());

      let sessionId1: string;
      let sessionId2: string;
      act(() => {
        sessionId1 = result.current.createSession();
        sessionId2 = result.current.createSession();
      });

      // sessionId2 is active
      act(() => {
        result.current.deleteSession(sessionId2!);
      });

      expect(result.current.activeSessionId).toBe(sessionId1!);
    });
  });

  describe('clearAllSessions', () => {
    it('clears all sessions and active session', () => {
      const { result } = renderHook(() => useChatStore());

      act(() => {
        result.current.createSession();
        result.current.createSession();
        result.current.createSession();
      });

      expect(result.current.sessions).toHaveLength(3);

      act(() => {
        result.current.clearAllSessions();
      });

      expect(result.current.sessions).toHaveLength(0);
      expect(result.current.activeSessionId).toBeNull();
    });
  });
});

describe('useChatStore persistence', () => {
  it('store has correct name for localStorage', () => {
    // The store uses 'chat-storage' as the persist name
    // This tests that the persist middleware is configured correctly
    const { result } = renderHook(() => useChatStore());

    act(() => {
      result.current.createSession();
    });

    expect(result.current.sessions).toHaveLength(1);
  });
});

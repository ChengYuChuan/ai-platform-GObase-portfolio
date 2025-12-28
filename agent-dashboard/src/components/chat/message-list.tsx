'use client';

import * as React from 'react';
import { MessageItem } from './message-item';
import { ScrollArea } from '@/components/ui/scroll-area';
import type { ChatMessage } from '@/types';

interface MessageListProps {
  messages: ChatMessage[];
  isLoading?: boolean;
  onRetry?: (messageId: string) => void;
}

export function MessageList({ messages, isLoading, onRetry }: MessageListProps) {
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const bottomRef = React.useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new messages arrive
  React.useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  if (messages.length === 0) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center p-8 text-center">
        <div className="rounded-full bg-primary/10 p-4">
          <svg
            className="h-8 w-8 text-primary"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
            />
          </svg>
        </div>
        <h3 className="mt-4 text-lg font-semibold">Start a conversation</h3>
        <p className="mt-2 max-w-sm text-sm text-muted-foreground">
          Ask a question or start a conversation with the AI assistant.
          You can also upload documents to get answers based on your content.
        </p>
      </div>
    );
  }

  return (
    <ScrollArea className="flex-1" ref={scrollRef}>
      <div className="flex flex-col py-4">
        {messages.map((message) => (
          <MessageItem
            key={message.id}
            message={message}
            onRetry={
              message.role === 'assistant' && onRetry
                ? () => onRetry(message.id)
                : undefined
            }
          />
        ))}

        {/* Typing indicator */}
        {isLoading && (
          <div className="flex gap-3 px-4 py-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-secondary">
              <svg
                className="h-4 w-4 text-secondary-foreground"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                />
              </svg>
            </div>
            <div className="flex items-center gap-1 rounded-2xl bg-muted px-4 py-2">
              <div className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground/50 [animation-delay:-0.3s]" />
              <div className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground/50 [animation-delay:-0.15s]" />
              <div className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground/50" />
            </div>
          </div>
        )}

        <div ref={bottomRef} />
      </div>
    </ScrollArea>
  );
}

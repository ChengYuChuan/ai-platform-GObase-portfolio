'use client';

import * as React from 'react';
import { MessageList } from './message-list';
import { ChatInput } from './chat-input';
import { useChat } from '@/lib/hooks/useChat';
import { Card } from '@/components/ui/card';

interface ChatWindowProps {
  sessionId?: string;
}

export function ChatWindow({ sessionId }: ChatWindowProps) {
  const {
    messages,
    isLoading,
    error,
    sendMessage,
    stopGeneration,
  } = useChat(sessionId);

  const [model, setModel] = React.useState('gpt-4');

  const handleSend = async (content: string) => {
    await sendMessage(content, true);
  };

  const handleRetry = async (messageId: string) => {
    // Find the user message before the assistant message
    const messageIndex = messages.findIndex((m) => m.id === messageId);
    if (messageIndex > 0) {
      const userMessage = messages[messageIndex - 1];
      if (userMessage.role === 'user') {
        await sendMessage(userMessage.content, true);
      }
    }
  };

  return (
    <Card className="flex h-full flex-col overflow-hidden">
      {/* Error banner */}
      {error && (
        <div className="border-b bg-destructive/10 px-4 py-2 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Messages */}
      <MessageList
        messages={messages}
        isLoading={isLoading}
        onRetry={handleRetry}
      />

      {/* Input */}
      <ChatInput
        onSend={handleSend}
        onStop={stopGeneration}
        isLoading={isLoading}
        model={model}
        onModelChange={setModel}
      />
    </Card>
  );
}

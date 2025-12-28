'use client';

import * as React from 'react';
import { Bot, User, Copy, Check, RotateCcw } from 'lucide-react';
import { cn, formatRelativeTime, copyToClipboard } from '@/lib/utils';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';
import type { ChatMessage } from '@/types';

interface MessageItemProps {
  message: ChatMessage;
  onRetry?: () => void;
}

export function MessageItem({ message, onRetry }: MessageItemProps) {
  const [copied, setCopied] = React.useState(false);
  const isUser = message.role === 'user';

  const handleCopy = async () => {
    const success = await copyToClipboard(message.content);
    if (success) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div
      className={cn(
        'group flex gap-3 px-4 py-3 transition-colors hover:bg-muted/50',
        isUser && 'flex-row-reverse'
      )}
    >
      {/* Avatar */}
      <Avatar className="h-8 w-8 shrink-0">
        <AvatarFallback className={cn(isUser ? 'bg-primary' : 'bg-secondary')}>
          {isUser ? (
            <User className="h-4 w-4 text-primary-foreground" />
          ) : (
            <Bot className="h-4 w-4 text-secondary-foreground" />
          )}
        </AvatarFallback>
      </Avatar>

      {/* Content */}
      <div className={cn('flex max-w-[80%] flex-col gap-1', isUser && 'items-end')}>
        {/* Message bubble */}
        <div
          className={cn(
            'rounded-2xl px-4 py-2',
            isUser
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-foreground'
          )}
        >
          <div className="prose-chat whitespace-pre-wrap break-words">
            {message.content}
          </div>
        </div>

        {/* Metadata and actions */}
        <div
          className={cn(
            'flex items-center gap-2 text-xs text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100',
            isUser && 'flex-row-reverse'
          )}
        >
          <span>{formatRelativeTime(message.timestamp)}</span>

          {!isUser && (
            <>
              <Button
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                onClick={handleCopy}
              >
                {copied ? (
                  <Check className="h-3 w-3 text-green-500" />
                ) : (
                  <Copy className="h-3 w-3" />
                )}
              </Button>

              {onRetry && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-6 w-6"
                  onClick={onRetry}
                >
                  <RotateCcw className="h-3 w-3" />
                </Button>
              )}
            </>
          )}

          {typeof message.metadata?.model === 'string' && (
            <span className="rounded bg-muted px-1.5 py-0.5">
              {message.metadata.model}
            </span>
          )}
        </div>

        {/* Sources */}
        {message.sources && message.sources.length > 0 && (
          <div className="mt-2 rounded-lg border bg-card p-3">
            <p className="mb-2 text-xs font-medium text-muted-foreground">
              Sources ({message.sources.length})
            </p>
            <div className="space-y-2">
              {message.sources.slice(0, 3).map((source, index) => (
                <div
                  key={index}
                  className="rounded bg-muted p-2 text-xs"
                >
                  <p className="line-clamp-2 text-foreground">
                    {source.content}
                  </p>
                  {typeof source.metadata?.title === 'string' && (
                    <p className="mt-1 text-muted-foreground">
                      {source.metadata.title}
                    </p>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

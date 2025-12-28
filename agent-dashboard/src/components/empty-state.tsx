'use client';

import * as React from 'react';
import { LucideIcon, FileQuestion, MessageSquare, Bot, FileText } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

interface EmptyStateProps {
  icon?: LucideIcon;
  title: string;
  description?: string;
  action?: {
    label: string;
    onClick: () => void;
  };
  className?: string;
}

export function EmptyState({
  icon: Icon = FileQuestion,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex min-h-[300px] flex-col items-center justify-center rounded-lg border border-dashed bg-muted/20 p-8 text-center',
        className
      )}
    >
      <div className="rounded-full bg-muted p-4">
        <Icon className="h-8 w-8 text-muted-foreground" />
      </div>
      <h3 className="mt-4 text-lg font-semibold">{title}</h3>
      {description && (
        <p className="mt-2 max-w-sm text-sm text-muted-foreground">{description}</p>
      )}
      {action && (
        <Button onClick={action.onClick} className="mt-4">
          {action.label}
        </Button>
      )}
    </div>
  );
}

// Preset empty states for common use cases
export function EmptyChat({ onNewChat }: { onNewChat?: () => void }) {
  return (
    <EmptyState
      icon={MessageSquare}
      title="No messages yet"
      description="Start a conversation with the AI assistant to get help with your questions."
      action={onNewChat ? { label: 'Start Chat', onClick: onNewChat } : undefined}
    />
  );
}

export function EmptyDocuments({ onUpload }: { onUpload?: () => void }) {
  return (
    <EmptyState
      icon={FileText}
      title="No documents uploaded"
      description="Upload documents to enable RAG-powered conversations and document search."
      action={onUpload ? { label: 'Upload Document', onClick: onUpload } : undefined}
    />
  );
}

export function EmptyAgentResults() {
  return (
    <EmptyState
      icon={Bot}
      title="No results yet"
      description="Run an agent query to see results and execution logs here."
    />
  );
}

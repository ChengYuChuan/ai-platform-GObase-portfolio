'use client';

import * as React from 'react';
import {
  CheckCircle,
  XCircle,
  AlertCircle,
  Info,
  ChevronDown,
  ChevronRight,
  Clock,
  Loader2,
} from 'lucide-react';
import { cn, formatRelativeTime } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import type { AgentExecution, AgentLog } from '@/types';

interface ExecutionLogProps {
  execution: AgentExecution;
  logs: AgentLog[];
  isLive?: boolean;
}

const logLevelConfig: Record<
  AgentLog['level'],
  { color: string; bgColor: string; icon: React.ElementType }
> = {
  info: { color: 'text-blue-500', bgColor: 'bg-blue-500/10', icon: Info },
  warning: { color: 'text-yellow-500', bgColor: 'bg-yellow-500/10', icon: AlertCircle },
  error: { color: 'text-red-500', bgColor: 'bg-red-500/10', icon: XCircle },
  success: { color: 'text-green-500', bgColor: 'bg-green-500/10', icon: CheckCircle },
};

const statusConfig: Record<
  AgentExecution['status'],
  { color: string; label: string }
> = {
  pending: { color: 'bg-muted', label: 'Pending' },
  running: { color: 'bg-blue-500', label: 'Running' },
  completed: { color: 'bg-green-500', label: 'Completed' },
  failed: { color: 'bg-red-500', label: 'Failed' },
};

export function ExecutionLog({ execution, logs, isLive = false }: ExecutionLogProps) {
  const [expandedLogs, setExpandedLogs] = React.useState<Set<string>>(new Set());
  const scrollRef = React.useRef<HTMLDivElement>(null);

  // Auto-scroll when new logs arrive
  React.useEffect(() => {
    if (isLive && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs, isLive]);

  const toggleLogExpansion = (logId: string) => {
    setExpandedLogs((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(logId)) {
        newSet.delete(logId);
      } else {
        newSet.add(logId);
      }
      return newSet;
    });
  };

  const duration = execution.endedAt
    ? (new Date(execution.endedAt).getTime() - new Date(execution.startedAt).getTime()) / 1000
    : null;

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <CardTitle className="text-lg">Execution Log</CardTitle>
            <Badge className={cn('text-white', statusConfig[execution.status].color)}>
              {isLive && execution.status === 'running' && (
                <Loader2 className="mr-1 h-3 w-3 animate-spin" />
              )}
              {statusConfig[execution.status].label}
            </Badge>
          </div>
          <div className="flex items-center gap-4 text-sm text-muted-foreground">
            <div className="flex items-center gap-1">
              <Clock className="h-4 w-4" />
              <span>{formatRelativeTime(execution.startedAt)}</span>
            </div>
            {duration !== null && (
              <span>{duration.toFixed(2)}s</span>
            )}
          </div>
        </div>
      </CardHeader>

      <CardContent>
        <ScrollArea className="h-[400px] pr-4" ref={scrollRef}>
          <div className="space-y-2">
            {logs.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-8 text-center">
                {isLive ? (
                  <>
                    <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                    <p className="mt-4 text-sm text-muted-foreground">
                      Waiting for logs...
                    </p>
                  </>
                ) : (
                  <>
                    <Info className="h-8 w-8 text-muted-foreground/50" />
                    <p className="mt-4 text-sm text-muted-foreground">
                      No logs available
                    </p>
                  </>
                )}
              </div>
            ) : (
              logs.map((log) => {
                const config = logLevelConfig[log.level];
                const Icon = config.icon;
                const isExpanded = expandedLogs.has(log.id);
                const hasDetails = log.metadata && Object.keys(log.metadata).length > 0;

                return (
                  <div
                    key={log.id}
                    className={cn(
                      'rounded-lg border p-3',
                      config.bgColor,
                      'border-transparent'
                    )}
                  >
                    <div className="flex items-start gap-3">
                      <Icon className={cn('h-4 w-4 mt-0.5 shrink-0', config.color)} />

                      <div className="min-w-0 flex-1">
                        <div className="flex items-center justify-between gap-2">
                          <p className="text-sm">{log.message}</p>
                          <span className="shrink-0 text-xs text-muted-foreground">
                            {new Date(log.timestamp).toLocaleTimeString()}
                          </span>
                        </div>

                        {log.step && (
                          <p className="mt-1 text-xs text-muted-foreground">
                            Step: {log.step}
                          </p>
                        )}

                        {hasDetails && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="mt-2 h-6 px-2 text-xs"
                            onClick={() => toggleLogExpansion(log.id)}
                          >
                            {isExpanded ? (
                              <ChevronDown className="mr-1 h-3 w-3" />
                            ) : (
                              <ChevronRight className="mr-1 h-3 w-3" />
                            )}
                            {isExpanded ? 'Hide details' : 'Show details'}
                          </Button>
                        )}

                        {isExpanded && log.metadata && (
                          <pre className="mt-2 overflow-x-auto rounded bg-muted p-2 text-xs">
                            {JSON.stringify(log.metadata, null, 2)}
                          </pre>
                        )}
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </ScrollArea>

        {/* Result */}
        {execution.result && (
          <div className="mt-4 rounded-lg border bg-muted p-4">
            <h4 className="mb-2 font-medium">Result</h4>
            <p className="text-sm">{execution.result.answer}</p>

            {execution.result.sources && execution.result.sources.length > 0 && (
              <div className="mt-3">
                <p className="text-xs font-medium text-muted-foreground">
                  Sources ({execution.result.sources.length})
                </p>
                <div className="mt-2 space-y-1">
                  {execution.result.sources.slice(0, 3).map((source, index) => (
                    <div key={index} className="text-xs text-muted-foreground">
                      • {source.content.slice(0, 100)}...
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Error */}
        {execution.error && (
          <div className="mt-4 rounded-lg border border-destructive/50 bg-destructive/10 p-4">
            <div className="flex items-center gap-2 text-destructive">
              <XCircle className="h-4 w-4" />
              <h4 className="font-medium">Error</h4>
            </div>
            <p className="mt-2 text-sm">{execution.error}</p>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

'use client';

import * as React from 'react';
import { Bot, Play, Settings, Clock, CheckCircle, XCircle, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import type { Agent } from '@/types';

interface AgentCardProps {
  agent: Agent;
  onRun?: (agent: Agent) => void;
  onConfigure?: (agent: Agent) => void;
  isRunning?: boolean;
}

const statusConfig: Record<
  Agent['status'],
  { color: string; icon: React.ElementType }
> = {
  active: { color: 'text-green-500', icon: CheckCircle },
  inactive: { color: 'text-muted-foreground', icon: XCircle },
  running: { color: 'text-blue-500', icon: Loader2 },
  error: { color: 'text-destructive', icon: XCircle },
};

export function AgentCard({ agent, onRun, onConfigure, isRunning = false }: AgentCardProps) {
  const status = isRunning ? 'running' : agent.status;
  const StatusIcon = statusConfig[status].icon;

  return (
    <Card className="flex flex-col">
      <CardHeader>
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
              <Bot className="h-5 w-5 text-primary" />
            </div>
            <div>
              <CardTitle className="text-lg">{agent.name}</CardTitle>
              <Badge variant="outline" className="mt-1 text-xs">
                {agent.type}
              </Badge>
            </div>
          </div>
          <div className={cn('flex items-center gap-1', statusConfig[status].color)}>
            <StatusIcon className={cn('h-4 w-4', isRunning && 'animate-spin')} />
            <span className="text-xs capitalize">{status}</span>
          </div>
        </div>
        <CardDescription className="mt-2">
          {agent.description}
        </CardDescription>
      </CardHeader>

      <CardContent className="flex-1">
        <div className="space-y-3 text-sm">
          {agent.capabilities && agent.capabilities.length > 0 && (
            <div>
              <p className="font-medium text-muted-foreground">Capabilities</p>
              <div className="mt-1 flex flex-wrap gap-1">
                {agent.capabilities.map((cap) => (
                  <Badge key={cap} variant="secondary" className="text-xs">
                    {cap}
                  </Badge>
                ))}
              </div>
            </div>
          )}

          {agent.lastRun && (
            <div className="flex items-center gap-2 text-muted-foreground">
              <Clock className="h-4 w-4" />
              <span>Last run: {new Date(agent.lastRun).toLocaleString()}</span>
            </div>
          )}

          {agent.config && (
            <div className="rounded bg-muted p-2 text-xs">
              <p className="font-medium">Configuration</p>
              <pre className="mt-1 text-muted-foreground">
                {JSON.stringify(agent.config, null, 2)}
              </pre>
            </div>
          )}
        </div>
      </CardContent>

      <CardFooter className="gap-2">
        <Button
          className="flex-1"
          onClick={() => onRun?.(agent)}
          disabled={isRunning || agent.status === 'error'}
        >
          {isRunning ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Running...
            </>
          ) : (
            <>
              <Play className="mr-2 h-4 w-4" />
              Run Agent
            </>
          )}
        </Button>
        <Button
          variant="outline"
          size="icon"
          onClick={() => onConfigure?.(agent)}
        >
          <Settings className="h-4 w-4" />
        </Button>
      </CardFooter>
    </Card>
  );
}

'use client';

import * as React from 'react';
import { Play, Square } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ExecutionLog } from './execution-log';
import { useAgent } from '@/lib/hooks/useAgent';
import type { Agent, AgentExecution, AgentLog } from '@/types';

interface AgentRunnerProps {
  agents: Agent[];
}

export function AgentRunner({ agents }: AgentRunnerProps) {
  const [selectedAgentType, setSelectedAgentType] = React.useState<string>('');
  const [question, setQuestion] = React.useState('');
  const [currentExecution, setCurrentExecution] = React.useState<AgentExecution | null>(null);
  const [logs, setLogs] = React.useState<AgentLog[]>([]);

  const { isLoading: isRunning, executeAgent, stopExecution } = useAgent();

  const selectedAgent = agents.find((a) => a.type === selectedAgentType);

  const handleRun = async () => {
    if (!selectedAgentType || !question.trim()) return;

    // Create a new execution
    const execution: AgentExecution = {
      id: Math.random().toString(36).substr(2, 9),
      agentType: selectedAgentType,
      status: 'running',
      startedAt: new Date().toISOString(),
      input: { question },
    };

    setCurrentExecution(execution);
    setLogs([]);

    try {
      // Add initial log
      const startLog: AgentLog = {
        id: '1',
        executionId: execution.id,
        timestamp: new Date().toISOString(),
        level: 'info',
        message: `Starting ${selectedAgentType} agent...`,
        step: 'initialization',
      };
      setLogs([startLog]);

      // Run the agent with streaming logs
      const result = await executeAgent({
        question: question.trim(),
        agentType: selectedAgentType,
      });

      if (result) {
        // Add completion logs
        const completionLogs: AgentLog[] = [
          {
            id: '2',
            executionId: execution.id,
            timestamp: new Date().toISOString(),
            level: 'info',
            message: 'Processing query...',
            step: 'processing',
          },
          {
            id: '3',
            executionId: execution.id,
            timestamp: new Date().toISOString(),
            level: 'success',
            message: 'Agent completed successfully',
            step: 'completion',
            metadata: { iterations: result.iterations },
          },
        ];

        setLogs((prev) => [...prev, ...completionLogs]);

        setCurrentExecution((prev) =>
          prev
            ? {
                ...prev,
                status: 'completed',
                endedAt: new Date().toISOString(),
                result,
              }
            : null
        );
      }
    } catch (error) {
      const errorLog: AgentLog = {
        id: 'error',
        executionId: execution.id,
        timestamp: new Date().toISOString(),
        level: 'error',
        message: error instanceof Error ? error.message : 'Execution failed',
        step: 'error',
      };
      setLogs((prev) => [...prev, errorLog]);

      setCurrentExecution((prev) =>
        prev
          ? {
              ...prev,
              status: 'failed',
              endedAt: new Date().toISOString(),
              error: error instanceof Error ? error.message : 'Unknown error',
            }
          : null
      );
    }
  };

  const handleCancel = () => {
    stopExecution();
    setCurrentExecution((prev) =>
      prev
        ? {
            ...prev,
            status: 'failed',
            endedAt: new Date().toISOString(),
            error: 'Execution cancelled by user',
          }
        : null
    );
  };

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      {/* Input Panel */}
      <Card>
        <CardHeader>
          <CardTitle>Run Agent</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Agent Selection */}
          <div className="space-y-2">
            <label className="text-sm font-medium">Select Agent</label>
            <Select value={selectedAgentType} onValueChange={setSelectedAgentType}>
              <SelectTrigger>
                <SelectValue placeholder="Choose an agent type..." />
              </SelectTrigger>
              <SelectContent>
                {agents.map((agent) => (
                  <SelectItem key={agent.type} value={agent.type}>
                    <div className="flex flex-col items-start">
                      <span className="font-medium">{agent.name}</span>
                      <span className="text-xs text-muted-foreground">
                        {agent.description}
                      </span>
                    </div>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Question Input */}
          <div className="space-y-2">
            <label className="text-sm font-medium">Question</label>
            <Textarea
              placeholder="Enter your question or task..."
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              className="min-h-[120px] resize-none"
            />
          </div>

          {/* Agent Info */}
          {selectedAgent && (
            <div className="rounded-lg bg-muted p-3 text-sm">
              <p className="font-medium">{selectedAgent.name}</p>
              <p className="mt-1 text-muted-foreground">{selectedAgent.description}</p>
              {selectedAgent.capabilities && (
                <div className="mt-2 flex flex-wrap gap-1">
                  {selectedAgent.capabilities.map((cap) => (
                    <span
                      key={cap}
                      className="rounded bg-background px-2 py-0.5 text-xs"
                    >
                      {cap}
                    </span>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Actions */}
          <div className="flex gap-2">
            {isRunning ? (
              <Button variant="destructive" onClick={handleCancel} className="flex-1">
                <Square className="mr-2 h-4 w-4" />
                Cancel
              </Button>
            ) : (
              <Button
                onClick={handleRun}
                disabled={!selectedAgentType || !question.trim()}
                className="flex-1"
              >
                <Play className="mr-2 h-4 w-4" />
                Run Agent
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Execution Log Panel */}
      <div>
        {currentExecution ? (
          <ExecutionLog
            execution={currentExecution}
            logs={logs}
            isLive={isRunning}
          />
        ) : (
          <Card className="flex h-full items-center justify-center">
            <CardContent className="py-12 text-center">
              <div className="rounded-full bg-muted p-4 mx-auto w-fit">
                <Play className="h-8 w-8 text-muted-foreground" />
              </div>
              <h3 className="mt-4 text-lg font-semibold">No Execution</h3>
              <p className="mt-2 text-sm text-muted-foreground">
                Select an agent and run it to see execution logs
              </p>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}

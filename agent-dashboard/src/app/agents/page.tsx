'use client';

import * as React from 'react';
import { AgentCard, AgentRunner } from '@/components/agents';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import type { Agent } from '@/types';

// Mock agents for demonstration
const mockAgents: Agent[] = [
  {
    id: '1',
    type: 'rag',
    name: 'RAG Agent',
    description: 'Retrieval-Augmented Generation agent for document Q&A',
    status: 'active',
    capabilities: ['document-search', 'question-answering', 'summarization'],
    config: {
      model: 'gpt-4',
      temperature: 0.7,
      maxTokens: 1000,
    },
    lastRun: new Date(Date.now() - 3600000).toISOString(),
  },
  {
    id: '2',
    type: 'research',
    name: 'Research Agent',
    description: 'Deep research agent for comprehensive analysis',
    status: 'active',
    capabilities: ['web-search', 'analysis', 'report-generation'],
    config: {
      model: 'gpt-4',
      maxIterations: 10,
    },
    lastRun: new Date(Date.now() - 86400000).toISOString(),
  },
  {
    id: '3',
    type: 'code',
    name: 'Code Agent',
    description: 'Code analysis and generation agent',
    status: 'active',
    capabilities: ['code-review', 'code-generation', 'debugging'],
    config: {
      model: 'gpt-4',
      language: 'python',
    },
  },
  {
    id: '4',
    type: 'summarizer',
    name: 'Summarizer Agent',
    description: 'Document and text summarization agent',
    status: 'inactive',
    capabilities: ['summarization', 'key-points', 'tldr'],
    config: {
      model: 'gpt-3.5-turbo',
      maxLength: 500,
    },
  },
];

export default function AgentsPage() {
  const [agents] = React.useState<Agent[]>(mockAgents);
  const [runningAgents, setRunningAgents] = React.useState<Set<string>>(new Set());

  const handleRunAgent = async (agent: Agent) => {
    setRunningAgents((prev) => new Set(prev).add(agent.id));

    // Simulate agent execution
    await new Promise((resolve) => setTimeout(resolve, 3000));

    setRunningAgents((prev) => {
      const newSet = new Set(prev);
      newSet.delete(agent.id);
      return newSet;
    });
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Agents</h1>
        <p className="text-muted-foreground">
          Configure and run specialized AI agents for various tasks.
        </p>
      </div>

      <Tabs defaultValue="runner" className="space-y-4">
        <TabsList>
          <TabsTrigger value="runner">Run Agent</TabsTrigger>
          <TabsTrigger value="gallery">Agent Gallery</TabsTrigger>
        </TabsList>

        <TabsContent value="runner" className="space-y-4">
          <AgentRunner agents={agents.filter((a) => a.status === 'active')} />
        </TabsContent>

        <TabsContent value="gallery" className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {agents.map((agent) => (
              <AgentCard
                key={agent.id}
                agent={agent}
                onRun={handleRunAgent}
                isRunning={runningAgents.has(agent.id)}
              />
            ))}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

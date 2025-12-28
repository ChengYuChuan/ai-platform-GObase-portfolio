'use client';

import * as React from 'react';
import { MessageSquare, FileText, Bot, DollarSign } from 'lucide-react';
import {
  UsageChart,
  ModelUsageChart,
  AgentStatsChart,
  StatsCard,
} from '@/components/analytics';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

// Mock data for demonstration
const usageData = [
  { date: 'Mon', queries: 120, tokens: 45 },
  { date: 'Tue', queries: 180, tokens: 72 },
  { date: 'Wed', queries: 150, tokens: 58 },
  { date: 'Thu', queries: 220, tokens: 89 },
  { date: 'Fri', queries: 280, tokens: 112 },
  { date: 'Sat', queries: 90, tokens: 35 },
  { date: 'Sun', queries: 75, tokens: 28 },
];

const modelUsageData = [
  { model: 'GPT-4', usage: 450000, cost: 13.5 },
  { model: 'GPT-3.5 Turbo', usage: 820000, cost: 1.64 },
  { model: 'Claude 3 Opus', usage: 120000, cost: 7.2 },
  { model: 'Claude 3 Sonnet', usage: 380000, cost: 5.7 },
];

const agentStatsData = [
  { name: 'RAG', value: 1250 },
  { name: 'Research', value: 450 },
  { name: 'Code', value: 320 },
  { name: 'Summarizer', value: 180 },
];

export default function AnalyticsPage() {
  const [timeRange, setTimeRange] = React.useState('7d');

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Analytics</h1>
          <p className="text-muted-foreground">
            Monitor usage metrics, performance, and costs.
          </p>
        </div>

        <Select value={timeRange} onValueChange={setTimeRange}>
          <SelectTrigger className="w-[180px]">
            <SelectValue placeholder="Select time range" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="24h">Last 24 hours</SelectItem>
            <SelectItem value="7d">Last 7 days</SelectItem>
            <SelectItem value="30d">Last 30 days</SelectItem>
            <SelectItem value="90d">Last 90 days</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Stats Overview */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatsCard
          title="Total Queries"
          value="1,115"
          trend={{ value: 12, label: 'from last week' }}
          icon={<MessageSquare className="h-4 w-4 text-muted-foreground" />}
        />
        <StatsCard
          title="Documents Processed"
          value="456"
          trend={{ value: 8, label: 'from last week' }}
          icon={<FileText className="h-4 w-4 text-muted-foreground" />}
        />
        <StatsCard
          title="Agent Executions"
          value="2,200"
          trend={{ value: 23, label: 'from last week' }}
          icon={<Bot className="h-4 w-4 text-muted-foreground" />}
        />
        <StatsCard
          title="Total Cost"
          value="$28.04"
          trend={{ value: -5, label: 'from last week' }}
          icon={<DollarSign className="h-4 w-4 text-muted-foreground" />}
        />
      </div>

      {/* Charts */}
      <div className="grid gap-6 lg:grid-cols-2">
        <UsageChart
          title="Usage Trends"
          description="Queries and token usage over time"
          data={usageData}
        />
        <ModelUsageChart
          title="Model Usage"
          description="Token consumption by model"
          data={modelUsageData}
        />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <AgentStatsChart
          title="Agent Distribution"
          description="Executions by agent type"
          data={agentStatsData}
        />

        {/* Top Queries Table */}
        <div className="rounded-lg border bg-card">
          <div className="border-b px-6 py-4">
            <h3 className="font-semibold">Recent Activity</h3>
            <p className="text-sm text-muted-foreground">
              Latest queries and executions
            </p>
          </div>
          <div className="divide-y">
            {[
              {
                query: 'What is the quarterly revenue?',
                agent: 'RAG',
                time: '2 min ago',
                tokens: 1250,
              },
              {
                query: 'Summarize the technical documentation',
                agent: 'Summarizer',
                time: '5 min ago',
                tokens: 890,
              },
              {
                query: 'Research market trends for Q4',
                agent: 'Research',
                time: '12 min ago',
                tokens: 3200,
              },
              {
                query: 'Review the authentication code',
                agent: 'Code',
                time: '25 min ago',
                tokens: 1800,
              },
              {
                query: 'Find related case studies',
                agent: 'RAG',
                time: '32 min ago',
                tokens: 950,
              },
            ].map((item, index) => (
              <div key={index} className="flex items-center justify-between px-6 py-3">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{item.query}</p>
                  <p className="text-xs text-muted-foreground">
                    {item.agent} • {item.time}
                  </p>
                </div>
                <div className="text-right text-sm text-muted-foreground">
                  {item.tokens.toLocaleString()} tokens
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

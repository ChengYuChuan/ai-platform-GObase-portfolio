import Link from 'next/link';
import {
  MessageSquare,
  FileText,
  Bot,
  BarChart3,
  ArrowRight,
  Zap,
  Shield,
  Globe,
} from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';

const features = [
  {
    title: 'AI Chat',
    description: 'Engage in real-time conversations with AI models using SSE streaming.',
    icon: MessageSquare,
    href: '/chat',
    color: 'text-blue-500',
    bgColor: 'bg-blue-500/10',
  },
  {
    title: 'Documents',
    description: 'Upload and manage documents for RAG-powered question answering.',
    icon: FileText,
    href: '/documents',
    color: 'text-green-500',
    bgColor: 'bg-green-500/10',
  },
  {
    title: 'Agents',
    description: 'Configure and run specialized AI agents for various tasks.',
    icon: Bot,
    href: '/agents',
    color: 'text-purple-500',
    bgColor: 'bg-purple-500/10',
  },
  {
    title: 'Analytics',
    description: 'Monitor usage metrics, performance, and costs in real-time.',
    icon: BarChart3,
    href: '/analytics',
    color: 'text-orange-500',
    bgColor: 'bg-orange-500/10',
  },
];

const highlights = [
  {
    title: 'Real-time Streaming',
    description: 'SSE-powered responses for instant feedback',
    icon: Zap,
  },
  {
    title: 'Enterprise Ready',
    description: 'Built with security and scalability in mind',
    icon: Shield,
  },
  {
    title: 'Multi-model Support',
    description: 'Works with OpenAI, Anthropic, and more',
    icon: Globe,
  },
];

export default function HomePage() {
  return (
    <div className="space-y-8">
      {/* Hero Section */}
      <div className="flex flex-col gap-4">
        <h1 className="text-3xl font-bold tracking-tight">
          Welcome to Agent Dashboard
        </h1>
        <p className="max-w-2xl text-muted-foreground">
          A comprehensive platform for managing AI agents, documents, and chat interactions.
          Built with Next.js, TypeScript, and modern best practices.
        </p>
      </div>

      {/* Quick Actions */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {features.map((feature) => {
          const Icon = feature.icon;
          return (
            <Link key={feature.href} href={feature.href}>
              <Card className="h-full transition-all hover:shadow-md hover:border-primary/50">
                <CardHeader className="pb-2">
                  <div className={`mb-2 inline-flex h-10 w-10 items-center justify-center rounded-lg ${feature.bgColor}`}>
                    <Icon className={`h-5 w-5 ${feature.color}`} />
                  </div>
                  <CardTitle className="text-lg">{feature.title}</CardTitle>
                </CardHeader>
                <CardContent>
                  <CardDescription>{feature.description}</CardDescription>
                  <div className="mt-4 flex items-center text-sm text-primary">
                    Get started
                    <ArrowRight className="ml-1 h-4 w-4" />
                  </div>
                </CardContent>
              </Card>
            </Link>
          );
        })}
      </div>

      {/* Stats Overview */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Total Conversations</CardDescription>
            <CardTitle className="text-3xl">1,284</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xs text-muted-foreground">
              <span className="text-green-500">+12%</span> from last month
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Documents Processed</CardDescription>
            <CardTitle className="text-3xl">456</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xs text-muted-foreground">
              <span className="text-green-500">+8%</span> from last month
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>Agent Executions</CardDescription>
            <CardTitle className="text-3xl">2,891</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xs text-muted-foreground">
              <span className="text-green-500">+23%</span> from last month
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Highlights */}
      <Card>
        <CardHeader>
          <CardTitle>Platform Highlights</CardTitle>
          <CardDescription>
            Built for developers who need powerful AI capabilities
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-6 md:grid-cols-3">
            {highlights.map((highlight) => {
              const Icon = highlight.icon;
              return (
                <div key={highlight.title} className="flex items-start gap-3">
                  <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                    <Icon className="h-5 w-5 text-primary" />
                  </div>
                  <div>
                    <h3 className="font-medium">{highlight.title}</h3>
                    <p className="text-sm text-muted-foreground">
                      {highlight.description}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>
        </CardContent>
      </Card>

      {/* CTA */}
      <Card className="bg-primary text-primary-foreground">
        <CardContent className="flex flex-col items-center justify-between gap-4 p-6 sm:flex-row">
          <div>
            <h3 className="text-lg font-semibold">Ready to get started?</h3>
            <p className="text-primary-foreground/80">
              Start a conversation with our AI assistant now.
            </p>
          </div>
          <Link href="/chat">
            <Button variant="secondary" size="lg">
              <MessageSquare className="mr-2 h-4 w-4" />
              Start Chatting
            </Button>
          </Link>
        </CardContent>
      </Card>
    </div>
  );
}

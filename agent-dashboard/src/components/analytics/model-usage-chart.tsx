'use client';

import * as React from 'react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';

interface ModelUsageChartProps {
  title: string;
  description?: string;
  data: Array<{
    model: string;
    usage: number;
    cost: number;
  }>;
}

const COLORS = [
  'hsl(221.2 83.2% 53.3%)',
  'hsl(142 76% 36%)',
  'hsl(47 100% 50%)',
  'hsl(0 84.2% 60.2%)',
  'hsl(262 83% 58%)',
];

export function ModelUsageChart({ title, description, data }: ModelUsageChartProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>
        <div className="h-[300px]">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={data}
              layout="vertical"
              margin={{ top: 5, right: 30, left: 80, bottom: 5 }}
            >
              <CartesianGrid strokeDasharray="3 3" className="stroke-muted" />
              <XAxis
                type="number"
                className="text-xs"
                tick={{ fill: 'hsl(var(--muted-foreground))' }}
              />
              <YAxis
                type="category"
                dataKey="model"
                className="text-xs"
                tick={{ fill: 'hsl(var(--muted-foreground))' }}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: 'hsl(var(--card))',
                  border: '1px solid hsl(var(--border))',
                  borderRadius: '0.5rem',
                }}
                labelStyle={{ color: 'hsl(var(--foreground))' }}
                formatter={(value, name) => {
                  const numValue = value as number;
                  return [
                    name === 'usage' ? `${numValue.toLocaleString()} tokens` : `$${numValue.toFixed(2)}`,
                    name === 'usage' ? 'Usage' : 'Cost',
                  ];
                }}
              />
              <Bar dataKey="usage" name="usage" radius={[0, 4, 4, 0]}>
                {data.map((_, index) => (
                  <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  );
}

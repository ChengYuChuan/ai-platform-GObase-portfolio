'use client';

import * as React from 'react';
import { usePathname } from 'next/navigation';
import { Bell, Moon, Sun, Search, User } from 'lucide-react';
import { useTheme } from 'next-themes';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';

// Map routes to page titles
const routeTitles: Record<string, string> = {
  '/': 'Dashboard',
  '/chat': 'Chat',
  '/documents': 'Documents',
  '/agents': 'Agents',
  '/analytics': 'Analytics',
  '/settings': 'Settings',
};

function getPageTitle(pathname: string): string {
  // Check exact match first
  if (routeTitles[pathname]) {
    return routeTitles[pathname];
  }

  // Check for prefix match
  for (const [route, title] of Object.entries(routeTitles)) {
    if (pathname.startsWith(route) && route !== '/') {
      return title;
    }
  }

  return 'Dashboard';
}

export function Header() {
  const pathname = usePathname();
  const { theme, setTheme } = useTheme();
  const [mounted, setMounted] = React.useState(false);

  // Prevent hydration mismatch
  React.useEffect(() => {
    setMounted(true);
  }, []);

  const pageTitle = getPageTitle(pathname);

  return (
    <TooltipProvider>
      <header className="flex h-14 items-center justify-between border-b bg-background px-4 lg:px-6">
        {/* Page Title */}
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-semibold">{pageTitle}</h1>
        </div>

        {/* Search Bar */}
        <div className="hidden flex-1 justify-center md:flex">
          <div className="relative w-full max-w-md">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              placeholder="Search documents, agents..."
              className="pl-9"
            />
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2">
          {/* Theme Toggle */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
              >
                {mounted && theme === 'dark' ? (
                  <Sun className="h-5 w-5" />
                ) : (
                  <Moon className="h-5 w-5" />
                )}
                <span className="sr-only">Toggle theme</span>
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>Toggle theme</p>
            </TooltipContent>
          </Tooltip>

          {/* Notifications */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" className="relative">
                <Bell className="h-5 w-5" />
                <span className="absolute right-1 top-1 h-2 w-2 rounded-full bg-destructive" />
                <span className="sr-only">Notifications</span>
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>Notifications</p>
            </TooltipContent>
          </Tooltip>

          {/* User Menu */}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="icon" className="rounded-full">
                <Avatar className="h-8 w-8">
                  <AvatarImage src="/avatar.png" alt="User" />
                  <AvatarFallback>
                    <User className="h-4 w-4" />
                  </AvatarFallback>
                </Avatar>
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              <p>Account</p>
            </TooltipContent>
          </Tooltip>
        </div>
      </header>
    </TooltipProvider>
  );
}

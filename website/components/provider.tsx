'use client';
import SearchDialog from '@/components/search';
import { RootProvider } from 'fumadocs-ui/provider/next';
import { type ReactNode } from 'react';

export function Provider({ children }: { children: ReactNode }) {
  return (
    <RootProvider
      search={{ SearchDialog }}
      // Light by default for everyone; switching is a choice, not an
      // inheritance from the OS. The picked theme persists locally.
      theme={{ defaultTheme: 'light', enableSystem: false }}
    >
      {children}
    </RootProvider>
  );
}

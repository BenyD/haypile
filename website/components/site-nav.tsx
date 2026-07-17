'use client';

import Link from 'next/link';
import { DownloadButton } from '@/components/download-dialog';
import { NavSearch } from '@/components/nav-search';
import { HaypileMark } from '@/components/logo';

export function SiteNav() {
  return (
    <header>
      <nav className="mx-auto flex h-16 w-full max-w-6xl items-center gap-6 px-4">
        <Link href="/" className="flex items-center gap-2 font-semibold tracking-tight">
          <HaypileMark className="size-5" />
          Haypile
        </Link>
        <div className="flex items-center gap-5 text-sm text-fd-muted-foreground">
          <Link href="/docs" className="hover:text-fd-foreground">
            Docs
          </Link>
          <Link
            href="https://github.com/BenyD/haypile"
            className="hover:text-fd-foreground"
          >
            GitHub
          </Link>
        </div>

        <NavSearch />

        <DownloadButton />
      </nav>
    </header>
  );
}

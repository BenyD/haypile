'use client';

import Link from 'next/link';
import { useSearchContext } from 'fumadocs-ui/contexts/search';
import { DownloadButton } from '@/components/download-dialog';
import { HaypileMark } from '@/components/logo';

export function SiteNav() {
  const { setOpenSearch } = useSearchContext();

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

        <button
          type="button"
          onClick={() => setOpenSearch(true)}
          className="mx-auto hidden w-full max-w-sm items-center gap-2 rounded-full border bg-fd-secondary px-4 py-1.5 text-sm text-fd-muted-foreground hover:bg-fd-accent sm:flex"
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            aria-hidden
          >
            <circle cx="11" cy="11" r="7" />
            <path d="m21 21-4.3-4.3" />
          </svg>
          Search docs
          <kbd className="ms-auto rounded border bg-fd-background px-1.5 text-xs">
            ⌘K
          </kbd>
        </button>

        <DownloadButton />
      </nav>
    </header>
  );
}

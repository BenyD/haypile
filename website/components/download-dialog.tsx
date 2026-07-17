'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { CopyBar } from '@/components/copy-command';

export function DownloadButton() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open]);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="ms-auto rounded-full bg-fd-primary px-4 py-1.5 text-sm font-medium text-fd-primary-foreground hover:opacity-90 sm:ms-0"
      >
        Download
      </button>

      {open ? (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
          onClick={() => setOpen(false)}
          role="dialog"
          aria-modal="true"
          aria-label="Install Haypile"
        >
          <div
            className="w-full max-w-md rounded-2xl border bg-fd-background p-6 sm:p-8"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start justify-between">
              <h2 className="text-lg font-semibold tracking-tight">
                Install Haypile
              </h2>
              <button
                type="button"
                onClick={() => setOpen(false)}
                aria-label="Close"
                className="rounded-md p-1 text-fd-muted-foreground hover:bg-fd-accent hover:text-fd-foreground"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden>
                  <path d="M18 6 6 18M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="mt-6">
              <p className="text-sm font-medium">macOS</p>
              <p className="mt-1 text-sm text-fd-muted-foreground">
                Homebrew, updates with brew upgrade.
              </p>
              <div className="mt-3">
                <CopyBar command="brew install BenyD/tap/hay" />
              </div>
            </div>

            <div className="mt-6 border-t pt-6">
              <p className="text-sm font-medium">Linux and macOS</p>
              <p className="mt-1 text-sm text-fd-muted-foreground">
                The install script detects your platform and fetches the
                right binary from GitHub releases.
              </p>
              <div className="mt-3">
                <CopyBar command="curl -fsSL haypile.sh | sh" />
              </div>
            </div>

            <div className="mt-6 border-t pt-6">
              <p className="text-sm font-medium">Windows and everything else</p>
              <p className="mt-1 text-sm text-fd-muted-foreground">
                One binary per platform. The embedding model is inside;
                nothing else to fetch.
              </p>
              <Link
                href="https://github.com/BenyD/haypile/releases/latest"
                className="mt-3 inline-block rounded-full border px-4 py-1.5 text-sm font-medium hover:bg-fd-accent"
              >
                Get it from GitHub
              </Link>
            </div>

            <p className="mt-6 text-sm text-fd-muted-foreground">
              First time?{' '}
              <Link
                href="/docs"
                className="underline underline-offset-4 hover:text-fd-foreground"
                onClick={() => setOpen(false)}
              >
                Follow the two minute tutorial
              </Link>
            </p>
          </div>
        </div>
      ) : null}
    </>
  );
}

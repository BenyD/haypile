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
              <p className="flex items-center gap-2 text-sm font-medium">
                <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor" className="text-fd-muted-foreground" aria-hidden>
                  <path d="M12.152 6.896c-.948 0-2.415-1.078-3.96-1.04-2.04.027-3.91 1.183-4.961 3.014-2.117 3.675-.546 9.103 1.519 12.09 1.013 1.454 2.208 3.09 3.792 3.03 1.52-.065 2.09-.987 3.935-.987 1.831 0 2.35.987 3.96.948 1.637-.026 2.676-1.48 3.676-2.948 1.156-1.688 1.636-3.325 1.662-3.415-.039-.013-3.182-1.221-3.22-4.857-.026-3.04 2.48-4.494 2.597-4.559-1.429-2.09-3.623-2.324-4.39-2.376-2-.156-3.675 1.09-4.61 1.09zM15.53 3.83c.843-1.012 1.4-2.427 1.245-3.83-1.207.052-2.662.805-3.532 1.818-.78.896-1.454 2.338-1.273 3.714 1.338.104 2.715-.688 3.56-1.702z" />
                </svg>
                macOS
              </p>
              <p className="mt-1 text-sm text-fd-muted-foreground">
                Homebrew, updates with brew upgrade.
              </p>
              <div className="mt-3">
                <CopyBar command="brew install BenyD/tap/hay" />
              </div>
            </div>

            <div className="mt-6 border-t pt-6">
              <p className="flex items-center gap-2 text-sm font-medium">
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-fd-muted-foreground" aria-hidden>
                  <polyline points="4 17 10 11 4 5" />
                  <line x1="12" y1="19" x2="20" y2="19" />
                </svg>
                Linux and macOS
              </p>
              <p className="mt-1 text-sm text-fd-muted-foreground">
                The install script detects your platform and fetches the
                right binary from GitHub releases.
              </p>
              <div className="mt-3">
                <CopyBar command="curl -fsSL haypile.sh | sh" />
              </div>
            </div>

            <div className="mt-6 border-t pt-6">
              <p className="flex items-center gap-2 text-sm font-medium">
                <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor" className="text-fd-muted-foreground" aria-hidden>
                  <path d="M0 3.449 9.75 2.1v9.451H0zm10.949-1.5L24 0v11.4H10.949zM0 12.6h9.75v9.451L0 20.699zm10.949 0H24V24l-13.051-1.801z" />
                </svg>
                Windows
              </p>
              <p className="mt-1 text-sm text-fd-muted-foreground">
                Run in PowerShell. Installs hay.exe and adds it to your PATH,
                no admin needed.
              </p>
              <div className="mt-3">
                <CopyBar command="irm https://haypile.sh/install.ps1 | iex" />
              </div>
            </div>

            <p className="mt-6 border-t pt-6 text-sm text-fd-muted-foreground">
              One binary per platform, embedding model inside. Grab it from{' '}
              <Link
                href="https://github.com/BenyD/haypile/releases/latest"
                className="underline underline-offset-4 hover:text-fd-foreground"
              >
                GitHub releases
              </Link>
              , or follow the{' '}
              <Link
                href="/docs"
                className="underline underline-offset-4 hover:text-fd-foreground"
                onClick={() => setOpen(false)}
              >
                two minute tutorial
              </Link>
              .
            </p>
          </div>
        </div>
      ) : null}
    </>
  );
}

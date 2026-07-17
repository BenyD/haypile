'use client';

import { useState } from 'react';

/* Compact copy bar: mono command truncated to one line, square copy
   button at the end. For feature sections and docs callouts. */
export function CopyBar({ command }: { command: string }) {
  const [copied, setCopied] = useState(false);

  return (
    <div className="flex w-full max-w-md items-center gap-2 rounded-lg border bg-fd-secondary py-1.5 pl-4 pr-1.5">
      <code className="min-w-0 flex-1 truncate font-mono text-[13px] text-fd-muted-foreground">
        {command}
      </code>
      <button
        type="button"
        onClick={() => {
          void navigator.clipboard.writeText(command);
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        }}
        className="flex size-7 shrink-0 items-center justify-center rounded-md bg-fd-primary text-fd-primary-foreground hover:opacity-90"
        aria-label="Copy command"
      >
        {copied ? (
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
            <path d="M20 6L9 17l-5-5" />
          </svg>
        ) : (
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
            <rect x="9" y="9" width="13" height="13" rx="2" />
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
          </svg>
        )}
      </button>
    </div>
  );
}

export function CopyCommand({ command }: { command: string }) {
  const [copied, setCopied] = useState(false);

  return (
    <button
      type="button"
      onClick={() => {
        void navigator.clipboard.writeText(command);
        setCopied(true);
        setTimeout(() => setCopied(false), 1500);
      }}
      className="group flex items-center gap-3 rounded-full border bg-fd-secondary px-6 py-3.5 font-mono text-sm transition-colors hover:bg-fd-accent"
      aria-label="Copy install command"
    >
      <span>{command}</span>
      <span className="text-fd-muted-foreground group-hover:text-fd-foreground">
        {copied ? (
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
            <path d="M20 6L9 17l-5-5" />
          </svg>
        ) : (
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
            <rect x="9" y="9" width="13" height="13" rx="2" />
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
          </svg>
        )}
      </span>
    </button>
  );
}

'use client';

import { useTheme } from 'next-themes';
import { useEffect, useState } from 'react';

/* Footer theme switch: a mini segmented sun/moon control. The active
   side is highlighted. Rendered only after mount so server and client
   markup always agree. */
export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  if (!mounted) {
    return <span className="h-6 w-12" aria-hidden />;
  }

  const dark = resolvedTheme === 'dark';
  const side = (active: boolean) =>
    'flex h-[18px] w-[22px] items-center justify-center rounded-full transition-colors ' +
    (active
      ? 'bg-fd-accent text-fd-foreground'
      : 'text-fd-muted-foreground hover:text-fd-foreground');

  return (
    <div
      role="radiogroup"
      aria-label="Theme"
      className="flex rounded-full border p-0.5"
    >
      <button
        type="button"
        role="radio"
        aria-checked={!dark}
        aria-label="Light theme"
        onClick={() => setTheme('light')}
        className={side(!dark)}
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden>
          <circle cx="12" cy="12" r="4" />
          <path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
        </svg>
      </button>
      <button
        type="button"
        role="radio"
        aria-checked={dark}
        aria-label="Dark theme"
        onClick={() => setTheme('dark')}
        className={side(dark)}
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
          <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z" />
        </svg>
      </button>
    </div>
  );
}

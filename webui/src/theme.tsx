import { useEffect, useState } from 'preact/hooks';

/* Theme preference: system, light, or dark. The index.html boot script
   resolves it before first paint; this control keeps <html data-theme>
   in sync from then on, including live OS changes while on system. */

export type ThemePref = 'system' | 'light' | 'dark';
const KEY = 'hay-theme';

function apply(pref: ThemePref) {
  const dark =
    pref === 'dark' ||
    (pref === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
}

export function ThemeToggle() {
  const [pref, setPref] = useState<ThemePref>(
    () => (localStorage.getItem(KEY) as ThemePref) || 'system',
  );

  useEffect(() => {
    apply(pref);
    localStorage.setItem(KEY, pref);
    if (pref !== 'system') return;
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => apply('system');
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, [pref]);

  const options: { value: ThemePref; label: string; icon: preact.JSX.Element }[] = [
    {
      value: 'light',
      label: 'Light theme',
      icon: (
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden>
          <circle cx="12" cy="12" r="4" />
          <path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
        </svg>
      ),
    },
    {
      value: 'system',
      label: 'System theme',
      icon: (
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden>
          <rect x="2" y="4" width="20" height="13" rx="2" />
          <path d="M8 21h8m-4-4v4" />
        </svg>
      ),
    },
    {
      value: 'dark',
      label: 'Dark theme',
      icon: (
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden>
          <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
        </svg>
      ),
    },
  ];

  return (
    <div class="flex items-center rounded-full border border-neutral-200 p-0.5 dark:border-neutral-800" role="radiogroup" aria-label="Theme">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={pref === o.value}
          aria-label={o.label}
          title={o.label}
          onClick={() => setPref(o.value)}
          class={`relative flex size-6 items-center justify-center rounded-full transition-[color,background-color] duration-150 after:absolute after:inset-x-0 after:-inset-y-2 ${
            pref === o.value
              ? 'bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900'
              : 'text-neutral-400 hover:text-neutral-700 dark:hover:text-neutral-300'
          }`}
        >
          {o.icon}
        </button>
      ))}
    </div>
  );
}

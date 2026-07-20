'use client';

import { useEffect, useRef, useState } from 'react';

/* Mobile companion to the sticky nav: a collapsed disclosure at the top
   of the page. Picking a section closes it and jumps there. */
export function MobileLegalToc({
  sections,
}: {
  sections: { id: string; title: string }[];
}) {
  const ref = useRef<HTMLDetailsElement>(null);

  return (
    <details ref={ref} className="group mt-8 rounded-lg border lg:hidden">
      <summary className="flex cursor-pointer list-none items-center justify-between px-4 py-3 text-sm font-medium [&::-webkit-details-marker]:hidden">
        On this page
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="text-fd-muted-foreground transition-transform group-open:rotate-180"
          aria-hidden
        >
          <path d="m6 9 6 6 6-6" />
        </svg>
      </summary>
      <ul className="border-t p-2">
        {sections.map((s) => (
          <li key={s.id}>
            <a
              href={`#${s.id}`}
              onClick={() => {
                if (ref.current) ref.current.open = false;
              }}
              className="block rounded-md px-2 py-1.5 text-sm text-fd-muted-foreground hover:bg-fd-accent hover:text-fd-foreground"
            >
              {s.title}
            </a>
          </li>
        ))}
      </ul>
    </details>
  );
}

/* Sticky section nav for legal pages: tracks the section in view and
   highlights it, Stripe style. */
export function LegalToc({
  sections,
}: {
  sections: { id: string; title: string }[];
}) {
  const [active, setActive] = useState(sections[0]?.id);

  /* Active = the last section whose heading has crossed the reading
     line. An intersection band cannot do this: sections near the end
     are shorter than the remaining scroll room and would never
     activate, so the bottom of the page explicitly wins for the last
     section. */
  useEffect(() => {
    const pick = () => {
      const doc = document.documentElement;
      if (window.innerHeight + window.scrollY >= doc.scrollHeight - 2) {
        setActive(sections[sections.length - 1]?.id);
        return;
      }
      const line = window.innerHeight * 0.25;
      let current = sections[0]?.id;
      for (const s of sections) {
        const el = document.getElementById(s.id);
        if (el && el.getBoundingClientRect().top <= line) current = s.id;
      }
      setActive(current);
    };
    pick();
    window.addEventListener('scroll', pick, { passive: true });
    window.addEventListener('resize', pick);
    return () => {
      window.removeEventListener('scroll', pick);
      window.removeEventListener('resize', pick);
    };
  }, [sections]);

  return (
    <nav className="sticky top-24 hidden self-start lg:block" aria-label="Sections">
      <ul className="space-y-1 border-l">
        {sections.map((s) => (
          <li key={s.id}>
            <a
              href={`#${s.id}`}
              className={
                '-ml-px block border-l py-1 pl-4 text-sm transition-colors ' +
                (active === s.id
                  ? 'border-fd-foreground font-medium text-fd-foreground'
                  : 'border-transparent text-fd-muted-foreground hover:text-fd-foreground')
              }
            >
              {s.title}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}

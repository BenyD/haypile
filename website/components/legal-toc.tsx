'use client';

import { useEffect, useState } from 'react';

/* Sticky section nav for legal pages: tracks the section in view and
   highlights it, Stripe style. */
export function LegalToc({
  sections,
}: {
  sections: { id: string; title: string }[];
}) {
  const [active, setActive] = useState(sections[0]?.id);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) setActive(e.target.id);
        }
      },
      { rootMargin: '-20% 0px -70% 0px' },
    );
    for (const s of sections) {
      const el = document.getElementById(s.id);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
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

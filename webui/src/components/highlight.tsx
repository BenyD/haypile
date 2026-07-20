import type { ComponentChildren } from 'preact';

/* Emphasizes query terms inside displayed text so a reader can see at a
   glance why a result matched. Everything renders as text nodes; the
   marks carry the styling. */

export function termsOf(query: string): string[] {
  return query
    .toLowerCase()
    .split(/\s+/)
    .map((w) => w.replace(/^[^\p{L}\p{N}]+|[^\p{L}\p{N}]+$/gu, ''))
    .filter((w) => w.length >= 3);
}

export function Highlight({ text, terms }: { text: string; terms: string[] }) {
  if (terms.length === 0) return <>{text}</>;
  const re = new RegExp(
    `(${terms.map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`,
    'gi',
  );
  const out: ComponentChildren[] = [];
  let last = 0;
  for (const m of text.matchAll(re)) {
    if (m.index! > last) out.push(text.slice(last, m.index));
    out.push(
      <mark class="rounded bg-neutral-200/80 px-0.5 text-inherit dark:bg-neutral-700/70">
        {m[0]}
      </mark>,
    );
    last = m.index! + m[0].length;
  }
  if (last < text.length) out.push(text.slice(last));
  return <>{out}</>;
}

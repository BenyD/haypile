import { useEffect, useRef, useState } from 'preact/hooks';
import { api, citeLabel, flowText, headingOf, plainText, type Passage, type SearchResult } from '../api';
import { Highlight, termsOf } from './highlight';
import { isMarkdownPath, Markdown } from './markdown';

/* Chunks are cut with deliberate overlap so retrieval keeps context at
   boundaries; rendered back to back that overlap reads as a repeated
   paragraph. Trim each passage's prefix where it repeats the previous
   passage's tail. Overlap is an exact copy, so exact matching works. */
function dedupeOverlap(passages: Passage[]): Passage[] {
  return passages.map((p, i) => {
    if (i === 0) return p;
    const prev = passages[i - 1].text;
    const cap = Math.min(600, prev.length, p.text.length);
    for (let k = cap; k >= 30; k--) {
      if (prev.endsWith(p.text.slice(0, k))) {
        return { ...p, text: p.text.slice(k).replace(/^\s+/, '') };
      }
    }
    return p;
  });
}

/* Side sheet showing a cited passage in context: the chunk the citation
   points at, highlighted, with its neighbors around it. */
export function SourcePanel({ result, query, onClose }: { result: SearchResult; query: string; onClose: () => void }) {
  const [passages, setPassages] = useState<Passage[] | null>(null);
  const [error, setError] = useState('');
  const currentRef = useRef<HTMLDivElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setPassages(null);
    setError('');
    api
      .chunk(result.path, result.chunk, 1)
      .then((r) => setPassages(dedupeOverlap(r.passages)))
      .catch((e: Error) => setError(e.message));
  }, [result]);

  // Land the reader on the match inside the cited chunk when there is
  // one, else on the cited chunk itself. Never on a neighbor's match:
  // the citation, not the query, decides where the panel opens.
  useEffect(() => {
    const mark = currentRef.current?.querySelector('mark');
    (mark ?? currentRef.current)?.scrollIntoView({ block: 'center' });
  }, [passages]);

  return (
    <aside
      class="fixed inset-y-0 right-0 z-40 flex w-[min(460px,92vw)] flex-col bg-white shadow-[-1px_0_0_rgb(0_0_0/0.06),-8px_0_24px_rgb(0_0_0/0.05),-24px_0_64px_rgb(0_0_0/0.07)] motion-safe:animate-panel-in dark:bg-neutral-950 dark:shadow-[-1px_0_0_rgb(255_255_255/0.08),-24px_0_64px_rgb(0_0_0/0.5)]"
      aria-label="Source passage"
    >
      <div class="flex items-start justify-between gap-3 border-b border-neutral-200 px-6 py-4 dark:border-neutral-800">
        <div class="min-w-0">
          <div class="font-mono text-[13px] font-semibold">{citeLabel(result)}</div>
          <div class="mt-1 break-all text-xs text-neutral-400 dark:text-neutral-500">{result.path}</div>
        </div>
        <button
          type="button"
          onClick={onClose}
          title="Close (Esc)"
          aria-label="Close"
          class="relative -m-2 p-2 text-xl leading-none text-neutral-400 transition-[color,scale] duration-150 after:absolute after:-inset-2 hover:text-neutral-900 active:scale-[0.96] dark:hover:text-neutral-100"
        >
          &times;
        </button>
      </div>

      <div ref={bodyRef} class="overflow-y-auto px-6 py-4">
        {error ? (
          <p class="text-sm text-neutral-500">Could not load the passage: {error}</p>
        ) : passages === null ? (
          <p class="text-sm text-neutral-400">Loading&hellip;</p>
        ) : (
          passages.map((p, i) => {
            // The label already names the heading; do not repeat it as
            // the body's first line.
            const body = headingOf(p.text)
              ? p.text.replace(/^#{1,6}\s+.+\n*/, '')
              : p.text;
            return (
            <div
              key={p.chunk}
              ref={p.current ? currentRef : undefined}
              style={{ animationDelay: `${i * 50}ms` }}
              class={`mb-2.5 rounded-xl px-4 py-2.5 text-[13.5px] leading-relaxed [text-wrap:pretty] motion-safe:animate-enter ${
                p.current
                  ? 'bg-neutral-50 text-neutral-900 shadow-[0_0_0_1px_rgb(0_0_0/0.08)] dark:bg-neutral-900 dark:text-neutral-100 dark:shadow-[0_0_0_1px_rgb(255_255_255/0.1)]'
                  : 'text-neutral-500 dark:text-neutral-400'
              }`}
            >
              <div class="mb-1 font-mono text-[11px] tabular-nums text-neutral-400 dark:text-neutral-500">
                {p.page ? `page ${p.page}` : headingOf(p.text) ?? `section ${p.chunk + 1}`}
              </div>
              {isMarkdownPath(result.path) ? (
                <Markdown text={body} />
              ) : (
                <span class="whitespace-pre-wrap">
                  <Highlight text={flowText(plainText(body))} terms={termsOf(query)} />
                </span>
              )}
            </div>
            );
          })
        )}
      </div>
    </aside>
  );
}

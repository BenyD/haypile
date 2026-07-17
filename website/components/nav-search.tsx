'use client';

import { useRouter } from 'next/navigation';
import { useRef, useState } from 'react';
import { useDocsSearch } from 'fumadocs-core/search/client';
import { oramaStaticClient } from 'fumadocs-core/search/client/orama-static';
import { create } from '@orama/orama';
import { useSearchContext } from 'fumadocs-ui/contexts/search';

function initOrama() {
  return create({
    schema: { _: 'string' },
    language: 'english',
  });
}

type Grouped = { url: string; title: string; snippet?: string };

/* The index returns matches wrapped in <mark> markers. Render them as
   real emphasis instead of literal tags; odd split segments are the
   highlighted parts. */
function Highlighted({ text }: { text: string }) {
  const parts = text.split(/<\/?mark>/);
  return (
    <>
      {parts.map((part, i) =>
        i % 2 === 1 ? (
          <mark key={i} className="bg-transparent font-semibold text-inherit">
            {part}
          </mark>
        ) : (
          part
        ),
      )}
    </>
  );
}

/* Inline nav search for the marketing pages: results populate in a
   dropdown under the field as you type. The docs section keeps its own
   command dialog; this shares the same static index. */
export function NavSearch() {
  const router = useRouter();
  const { setOpenSearch } = useSearchContext();
  const [open, setOpen] = useState(false);
  const wrapper = useRef<HTMLDivElement>(null);
  const { search, setSearch, query } = useDocsSearch({
    client: oramaStaticClient({ initOrama }),
  });

  const results: Grouped[] = [];
  if (query.data && query.data !== 'empty') {
    for (const item of query.data) {
      if (item.type === 'page') {
        results.push({ url: item.url, title: String(item.content) });
      } else {
        const page = results.find((r) => item.url.startsWith(r.url));
        if (page && !page.snippet) page.snippet = String(item.content);
      }
    }
  }
  const top = results.slice(0, 5);
  const showPanel = open && search.length > 0;

  return (
    <div
      ref={wrapper}
      className="relative mx-auto hidden w-full max-w-sm sm:block"
      onBlur={(e) => {
        if (!wrapper.current?.contains(e.relatedTarget as Node)) setOpen(false);
      }}
    >
      <div className="flex items-center gap-2 rounded-full border bg-fd-secondary px-4 py-1.5 font-mono text-[13px]">
        <span className="select-none text-fd-muted-foreground" aria-hidden>
          $
        </span>
        <input
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setOpen(false);
            if (e.key === 'Enter' && top[0]) {
              setOpen(false);
              router.push(top[0].url);
            }
          }}
          placeholder="search docs"
          aria-label="Search docs"
          className="w-full bg-transparent outline-none placeholder:text-fd-muted-foreground"
        />
      </div>

      {showPanel ? (
        <div className="absolute inset-x-0 top-full z-50 mt-2 overflow-hidden rounded-xl border bg-fd-background text-left">
          {top.length > 0 ? (
            <ul>
              {top.map((r) => (
                <li key={r.url}>
                  <button
                    type="button"
                    onClick={() => {
                      setOpen(false);
                      router.push(r.url);
                    }}
                    className="block w-full px-4 py-3 text-left hover:bg-fd-accent"
                  >
                    <span className="block text-sm font-medium">
                      <Highlighted text={r.title} />
                    </span>
                    {r.snippet ? (
                      <span className="mt-0.5 block truncate text-xs text-fd-muted-foreground">
                        <Highlighted text={r.snippet} />
                      </span>
                    ) : null}
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="px-4 py-3 text-sm text-fd-muted-foreground">
              No results for &quot;{search}&quot;
            </p>
          )}
          <button
            type="button"
            onClick={() => {
              setOpen(false);
              setOpenSearch(true);
            }}
            className="block w-full border-t px-4 py-2.5 text-center text-sm font-medium hover:bg-fd-accent"
          >
            View all →
          </button>
        </div>
      ) : null}
    </div>
  );
}

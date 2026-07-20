import { useCallback, useEffect, useRef, useState } from 'preact/hooks';
import { api, askStream, citeLabel, flowText, headingOf, plainText, type IndexStats, type SearchResult, type Status } from './api';
import { ThemeToggle } from './theme';
import { Highlight, termsOf } from './components/highlight';
import { SourcePanel } from './components/panel';
import { SourcesModal } from './components/sources';

/* Answer text with [n] markers turned into buttons that open the
   matching source. */
function AnswerText({ text, sources, onCite }: {
  text: string;
  sources: SearchResult[];
  onCite: (r: SearchResult) => void;
}) {
  const parts = text.split(/(\[\d+\])/);
  return (
    <div class="whitespace-pre-wrap text-[15px] leading-relaxed [text-wrap:pretty]">
      {parts.map((part, i) => {
        const m = part.match(/^\[(\d+)\]$/);
        const src = m ? sources[Number(m[1]) - 1] : undefined;
        return src ? (
          <button
            key={i}
            type="button"
            onClick={() => onCite(src)}
            class="rounded-md border border-neutral-200 bg-neutral-50 px-1.5 font-mono text-xs align-[1px] transition-[border-color,scale] duration-150 hover:border-neutral-400 active:scale-[0.96] dark:border-neutral-800 dark:bg-neutral-900 dark:hover:border-neutral-600"
          >
            {part}
          </button>
        ) : (
          part
        );
      })}
    </div>
  );
}

export function App() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [status, setStatus] = useState<Status | null>(null);
  const [panel, setPanel] = useState<SearchResult | null>(null);
  const [managing, setManaging] = useState(false);

  const [answer, setAnswer] = useState<string | null>(null); // null = no ask active
  const [askState, setAskState] = useState<'idle' | 'thinking' | 'answering'>('idle');
  const [askNote, setAskNote] = useState('');

  // Indexing lives here, not in the sources modal: closing the modal
  // must not orphan a running add or its outcome.
  const [indexing, setIndexing] = useState('');
  const [indexNote, setIndexNote] = useState('');

  const inputRef = useRef<HTMLInputElement>(null);
  const searchSeq = useRef(0);
  const asking = useRef<AbortController | null>(null);
  const debounce = useRef<ReturnType<typeof setTimeout>>();

  const refreshStatus = useCallback(() => {
    api.status().then(setStatus).catch(() => {});
  }, []);
  useEffect(() => {
    refreshStatus();
    const id = setInterval(refreshStatus, 15000);
    return () => clearInterval(id);
  }, [refreshStatus]);

  const search = useCallback(async (q: string) => {
    const seq = ++searchSeq.current;
    if (!q.trim()) {
      setResults([]);
      return;
    }
    try {
      const r = await api.query(q);
      if (seq === searchSeq.current) setResults(r.results ?? []);
    } catch {
      /* daemon briefly away; the next keystroke retries */
    }
  }, []);

  const addSource = useCallback(
    async (path: string, tag: string): Promise<boolean> => {
      setIndexing(path);
      setIndexNote('');
      try {
        const stats: IndexStats = await api.addSource(path, tag);
        setIndexNote(
          `Indexed ${stats.Indexed} files (${stats.Chunks} chunks), ${stats.Skipped} unchanged.`,
        );
        return true;
      } catch (e) {
        setIndexNote(e instanceof Error ? e.message : String(e));
        return false;
      } finally {
        setIndexing('');
        refreshStatus();
        setTimeout(() => setIndexNote(''), 8000);
      }
    },
    [refreshStatus],
  );

  const resetAsk = () => {
    asking.current?.abort();
    asking.current = null;
    setAnswer(null);
    setAskState('idle');
    setAskNote('');
  };

  const onInput = (e: Event) => {
    const q = (e.target as HTMLInputElement).value;
    setQuery(q);
    resetAsk();
    clearTimeout(debounce.current);
    debounce.current = setTimeout(() => void search(q), 150);
  };

  const ask = async () => {
    const question = query.trim();
    if (!question || asking.current) return;
    resetAsk();
    setAnswer('');
    setAskState('thinking');
    const ctl = new AbortController();
    asking.current = ctl;
    try {
      await askStream(
        question,
        {
          sources: (s) => {
            setResults(s);
            setAskState('answering');
          },
          token: (t) => setAnswer((a) => (a ?? '') + t),
          error: (message) => setAskNote(message),
          done: () => setAskState('idle'),
        },
        ctl.signal,
      );
      setAskState('idle');
    } catch (e) {
      if (e instanceof DOMException && e.name === 'AbortError') return;
      const message = e instanceof Error ? e.message : String(e);
      setAskState('idle');
      setAskNote(
        (e as { status?: number }).status === 503
          ? `No local LLM found, showing search results instead.\n\n${message}`
          : message,
      );
      void search(question);
    } finally {
      if (asking.current === ctl) asking.current = null;
    }
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setPanel(null);
      // "/" jumps to search, but never while typing somewhere else (the
      // sources modal's path field is full of slashes).
      const t = e.target as HTMLElement;
      const typing =
        t instanceof HTMLInputElement || t instanceof HTMLTextAreaElement || t.isContentEditable;
      if (e.key === '/' && !typing && document.activeElement !== inputRef.current) {
        e.preventDefault();
        inputRef.current?.focus({ preventScroll: true });
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);

  const empty = status !== null && status.files === 0;
  // The search is the app's center of gravity: it sits mid-viewport
  // while idle and glides up once there is anything to show.
  const engaged = query.trim() !== '' || answer !== null || askNote !== '' || empty;

  return (
    <div class="text-neutral-900 dark:text-neutral-100">
      <header class="mx-auto flex max-w-2xl items-center gap-2 px-6 pt-6">
        <svg viewBox="0 0 24 24" fill="currentColor" class="size-5" aria-hidden>
          <rect x="9" y="3" width="6" height="3.4" rx="1.7" />
          <rect x="6" y="8.3" width="12" height="3.4" rx="1.7" />
          <rect x="3" y="13.6" width="18" height="3.4" rx="1.7" />
          <rect x="3" y="18.9" width="18" height="3.4" rx="1.7" opacity="0.35" />
        </svg>
        <span class="font-semibold tracking-tight">Haypile</span>
        <span class="ms-auto flex items-center gap-2">
          <button
            type="button"
            onClick={() => setManaging(true)}
            class="rounded-full border border-neutral-200 px-3.5 py-1.5 text-[13px] text-neutral-600 transition-[border-color,color,scale] duration-150 hover:border-neutral-400 hover:text-neutral-900 active:scale-[0.96] dark:border-neutral-800 dark:text-neutral-400 dark:hover:border-neutral-600 dark:hover:text-neutral-100"
          >
            Sources
          </button>
          <ThemeToggle />
        </span>
      </header>

      <main
        class={`mx-auto max-w-2xl px-6 pb-16 transition-[padding-top] duration-500 [transition-timing-function:var(--ease-swift)] ${
          engaged ? 'pt-10' : 'pt-[26vh]'
        }`}
      >
        {!engaged ? (
          <div class="mb-7 text-center motion-safe:animate-enter">
            <h1 class="text-2xl font-semibold tracking-tight [text-wrap:balance]">
              Search your documents
            </h1>
            <p class="mt-1.5 text-sm text-neutral-500 [text-wrap:pretty]">
              By meaning, not just keywords. Ask questions, get cited answers. Nothing leaves your machine.
            </p>
          </div>
        ) : null}

        <div class="flex gap-2">
          <input
            ref={inputRef}
            type="text"
            value={query}
            onInput={onInput}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) void ask();
            }}
            placeholder="Search, or ask a question"
            aria-label="Search or ask"
            autocomplete="off"
            spellcheck={false}
            class="min-w-0 flex-1 rounded-2xl border border-neutral-200 bg-white px-5 py-3.5 text-base shadow-[0_1px_2px_rgb(0_0_0/0.04),0_4px_12px_rgb(0_0_0/0.03)] outline-none transition-[border-color,box-shadow] duration-200 placeholder:text-neutral-400 focus:border-neutral-400 focus:shadow-[0_1px_2px_rgb(0_0_0/0.06),0_8px_24px_rgb(0_0_0/0.06)] dark:border-neutral-800 dark:bg-neutral-950 dark:shadow-none dark:placeholder:text-neutral-600 dark:focus:border-neutral-600"
          />
          <button
            type="button"
            onClick={() => void ask()}
            disabled={askState !== 'idle' || !query.trim()}
            title="Ask (Cmd+Enter)"
            class="rounded-full bg-neutral-900 px-6 text-sm font-medium text-white transition-[opacity,scale] duration-150 hover:opacity-90 active:scale-[0.96] disabled:opacity-40 dark:bg-neutral-100 dark:text-neutral-900"
          >
            Ask
          </button>
        </div>
        <p class={`mt-2.5 text-xs text-neutral-400 dark:text-neutral-500 ${engaged ? 'mb-7' : 'text-center'}`}>
          Typing searches as you go. Ask answers with citations.{' '}
          <kbd class="rounded border border-b-2 border-neutral-200 px-1 font-mono text-[11px] dark:border-neutral-800">/</kbd>{' '}
          focuses.
        </p>

        {empty ? (
          <section class="mb-5 rounded-2xl border border-neutral-200 px-6 py-5 motion-safe:animate-pop-in dark:border-neutral-800">
            <h2 class="text-[15px] font-semibold [text-wrap:balance]">Nothing indexed yet</h2>
            <p class="mt-1 text-sm text-neutral-500 [text-wrap:pretty]">
              Add a folder or file here, or run{' '}
              <code class="rounded-md bg-neutral-100 px-1.5 py-0.5 font-mono text-[12.5px] dark:bg-neutral-900">hay add ~/Documents</code>{' '}
              in your terminal. The index stays on this machine.
            </p>
            <button
              type="button"
              onClick={() => setManaging(true)}
              class="mt-4 rounded-full bg-neutral-900 px-5 py-2 text-sm font-medium text-white transition-[opacity,scale] duration-150 hover:opacity-90 active:scale-[0.96] dark:bg-neutral-100 dark:text-neutral-900"
            >
              Add a source
            </button>
          </section>
        ) : null}

        {answer !== null || askNote ? (
          <section class="mb-5 rounded-2xl border border-neutral-200 px-6 py-5 motion-safe:animate-pop-in dark:border-neutral-800">
            <div class="mb-1.5 flex items-baseline gap-2.5">
              <span class="text-xs font-semibold uppercase tracking-wider text-neutral-400 dark:text-neutral-500">
                Answer
              </span>
              {askState !== 'idle' ? (
                <span class="text-xs text-neutral-400 motion-safe:animate-fade-in">{askState}</span>
              ) : null}
            </div>
            {answer !== null ? (
              answer === '' && askState !== 'idle' ? (
                <p class="text-neutral-400">&hellip;</p>
              ) : (
                <AnswerText text={answer} sources={results} onCite={setPanel} />
              )
            ) : null}
            {askNote ? (
              <p class="mt-3 whitespace-pre-wrap rounded-xl bg-neutral-50 px-4 py-2.5 text-[13.5px] text-neutral-500 [text-wrap:pretty] dark:bg-neutral-900">
                {askNote}
              </p>
            ) : null}
          </section>
        ) : null}

        <section aria-label="Results">
          {results.map((r, i) => (
            <button
              key={`${r.path}:${r.chunk}`}
              type="button"
              onClick={() => setPanel(r)}
              style={{ animationDelay: `${Math.min(i, 8) * 35}ms` }}
              class="mb-2.5 block w-full rounded-2xl border border-neutral-200 px-5 py-3.5 text-left transition-[border-color,background-color,scale] duration-150 [transition-timing-function:var(--ease-swift)] hover:border-neutral-400 hover:bg-neutral-50 focus-visible:border-neutral-400 focus-visible:outline-none active:scale-[0.99] motion-safe:animate-enter dark:border-neutral-800 dark:hover:border-neutral-600 dark:hover:bg-neutral-900"
            >
              <div class="flex items-baseline gap-2 font-mono text-[12.5px]">
                <span class="tabular-nums text-neutral-400 dark:text-neutral-500">[{i + 1}]</span>
                <span>{citeLabel(r)}</span>
              </div>
              <div class="mt-1 whitespace-pre-wrap text-[13.5px] text-neutral-500 [text-wrap:pretty] dark:text-neutral-400">
                <Highlight
                  text={flowText(
                    plainText(
                      headingOf(r.snippet) ? r.snippet.replace(/^#{1,6}\s+.+\n*/, '') : r.snippet,
                    ),
                  )}
                  terms={termsOf(query)}
                />
              </div>
            </button>
          ))}
        </section>

      </main>

      {status && !engaged ? (
        <p class="fixed inset-x-0 bottom-6 flex justify-center gap-5 text-[12.5px] tabular-nums text-neutral-400 motion-safe:animate-fade-in dark:text-neutral-500">
          <span>{status.files} files, {status.chunks} chunks indexed</span>
          <span>{status.model || 'keyword-only mode'}</span>
          <span>outbound connections: {status.outbound_connections}</span>
        </p>
      ) : null}

      {(indexing || indexNote) && !managing ? (
        <p class="fixed inset-x-0 bottom-6 flex justify-center px-6 text-[12.5px] text-neutral-400 motion-safe:animate-fade-in dark:text-neutral-500">
          <span class="max-w-full truncate">
            {indexing ? `Indexing ${indexing}…` : indexNote}
          </span>
        </p>
      ) : null}

      {panel ? <SourcePanel result={panel} query={query} onClose={() => setPanel(null)} /> : null}
      {managing ? (
        <SourcesModal
          onClose={() => setManaging(false)}
          onChanged={() => {
            refreshStatus();
            if (query.trim()) void search(query);
          }}
          indexing={indexing}
          indexNote={indexNote}
          addSource={async (p, t) => {
            const ok = await addSource(p, t);
            if (ok && query.trim()) void search(query);
            return ok;
          }}
        />
      ) : null}
    </div>
  );
}

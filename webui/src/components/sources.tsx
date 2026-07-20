import { useEffect, useRef, useState } from 'preact/hooks';
import { api, type Browse, type Source } from '../api';

/* Sources manager: everything `hay add`, `hay list`, and `hay remove`
   do, from the browser: folders or single files. Picking works by asking
   the daemon to list the filesystem; a web page cannot read paths itself.
   The path field autocompletes against the daemon (the Syncthing
   pattern); the in-page browser is the primary picker and the native OS
   dialog is offered inside it where the platform has one. */

export type SourcesModalProps = {
  onClose: () => void;
  onChanged: () => void;
  /* Indexing is owned by the app, not this modal: closing the modal
     must not orphan a running add. */
  indexing: string;
  indexNote: string;
  addSource: (path: string, tag: string) => Promise<boolean>;
};

type Suggestion = { name: string; path: string; dir: boolean };

export function SourcesModal({ onClose, onChanged, indexing, indexNote, addSource }: SourcesModalProps) {
  const [sources, setSources] = useState<Source[]>([]);
  const [picking, setPicking] = useState(false);
  const [nativeOk, setNativeOk] = useState(true);
  const [path, setPath] = useState('');
  const [tag, setTag] = useState('');
  const [tagOpen, setTagOpen] = useState(false);
  const [busy, setBusy] = useState('');
  const [note, setNote] = useState('');
  const [sugg, setSugg] = useState<Suggestion[]>([]);
  // -1 means nothing highlighted: Enter submits the form instead of
  // grabbing a suggestion the user never asked for.
  const [suggIdx, setSuggIdx] = useState(-1);
  const pathInput = useRef<HTMLInputElement>(null);
  const suggTimer = useRef<number | undefined>(undefined);
  const suggSeq = useRef(0);

  useEffect(() => {
    pathInput.current?.focus();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      // Escape peels one layer: the in-page browser first, then the modal.
      // (The suggestion dropdown handles its own Escape and stops it.)
      setPicking((p) => {
        if (!p) onClose();
        return false;
      });
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  const refresh = () =>
    api.sources().then((r) => setSources(r.sources ?? [])).catch(() => {});
  useEffect(() => {
    void refresh();
  }, []);
  // A finished add (possibly started before this modal opened) refreshes
  // the list.
  useEffect(() => {
    if (!indexing) void refresh();
  }, [indexing]);

  /* Autocomplete: list the typed path's parent directory and filter its
     entries on the last segment. Debounced; a sequence number drops
     answers that arrive after the input moved on. */
  const complete = (value: string) => {
    window.clearTimeout(suggTimer.current);
    const seq = ++suggSeq.current;
    if (!value.startsWith('/')) {
      setSugg([]);
      return;
    }
    suggTimer.current = window.setTimeout(() => {
      const cut = value.lastIndexOf('/');
      const dir = cut === 0 ? '/' : value.slice(0, cut);
      const prefix = value.slice(cut + 1).toLowerCase();
      api
        .browse(dir)
        .then((b) => {
          if (seq !== suggSeq.current) return;
          const match = (n: string) => n.toLowerCase().startsWith(prefix);
          const list: Suggestion[] = [
            ...b.dirs.filter((d) => match(d.name)).map((d) => ({ ...d, dir: true })),
            ...b.files.filter((f) => match(f.name)).map((f) => ({ ...f, dir: false })),
          ].slice(0, 8);
          // The sole exact match is the path already in the box; nothing
          // to offer.
          if (list.length === 1 && list[0].path === value) list.length = 0;
          setSugg(list);
          setSuggIdx(-1);
        })
        .catch(() => {
          if (seq === suggSeq.current) setSugg([]);
        });
    }, 150);
  };

  const accept = (s: Suggestion) => {
    const v = s.dir ? s.path + '/' : s.path;
    setPath(v);
    if (s.dir) complete(v);
    else setSugg([]);
    pathInput.current?.focus();
  };

  const onPathKey = (e: KeyboardEvent) => {
    if (sugg.length === 0) return;
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSuggIdx((i) => (i + 1) % sugg.length);
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSuggIdx((i) => (i <= 0 ? sugg.length - 1 : i - 1));
        break;
      case 'Tab':
        e.preventDefault();
        accept(sugg[Math.max(0, suggIdx)]);
        break;
      case 'Enter':
        if (suggIdx >= 0) {
          e.preventDefault();
          accept(sugg[suggIdx]);
        } else {
          setSugg([]);
        }
        break;
      case 'Escape':
        e.stopPropagation();
        setSugg([]);
        break;
    }
  };

  const add = async () => {
    const p = path.trim();
    if (!p || indexing) return;
    if (await addSource(p, tag.trim())) {
      setPath('');
      setTag('');
      setTagOpen(false);
      setSugg([]);
      await refresh();
    }
  };

  const remove = async (p: string) => {
    setBusy(p);
    try {
      await api.removeSource(p);
      await refresh();
      onChanged();
    } catch (e) {
      setNote(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  };

  // The OS dialog, reachable from inside the in-page browser. 501 means
  // the platform has none; the button disappears and the in-page browser
  // simply is the picker.
  const nativePick = async () => {
    try {
      const picked = await api.pick('folder');
      if (picked !== null) {
        setPath(picked);
        setPicking(false);
      }
    } catch {
      setNativeOk(false);
    }
  };

  return (
    <div
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4 motion-safe:animate-fade-in"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label="Sources"
    >
      <div
        class="max-h-[85vh] w-full max-w-lg overflow-y-auto rounded-3xl bg-white p-6 shadow-[0_0_0_1px_rgb(0_0_0/0.06),0_8px_24px_rgb(0_0_0/0.08),0_24px_64px_rgb(0_0_0/0.12)] motion-safe:animate-pop-in dark:bg-neutral-950 dark:shadow-[0_0_0_1px_rgb(255_255_255/0.09),0_24px_64px_rgb(0_0_0/0.6)]"
        onClick={(e) => e.stopPropagation()}
      >
        <div class="flex items-start justify-between">
          <h2 class="text-lg font-semibold tracking-tight">Sources</h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            class="text-xl leading-none text-neutral-400 hover:text-neutral-900 dark:hover:text-neutral-100"
          >
            &times;
          </button>
        </div>

        <div class="mt-5 space-y-2">
          {sources.length === 0 ? (
            <p class="text-sm text-neutral-500">Nothing indexed yet. Add a folder or file below.</p>
          ) : (
            sources.map((s) => (
              <div
                key={s.path}
                class="flex items-center justify-between gap-3 rounded-xl border border-neutral-200 px-4 py-3 dark:border-neutral-800"
              >
                <div class="min-w-0">
                  <div class="truncate font-mono text-[13px]">{s.path}</div>
                  <div class="mt-0.5 flex items-center gap-1.5 text-xs text-neutral-400 dark:text-neutral-500">
                    <span>
                      {s.files} files, {s.chunks} chunks
                    </span>
                    {s.tag ? (
                      <span
                        title={`CLI and MCP searches can be scoped to this source with the tag "${s.tag}"`}
                        class="rounded-full bg-neutral-100 px-2 py-0.5 text-[11px] text-neutral-500 dark:bg-neutral-900 dark:text-neutral-400"
                      >
                        {s.tag}
                      </span>
                    ) : null}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => remove(s.path)}
                  disabled={busy !== ''}
                  title="Removes this source from the index. Your files are not touched."
                  class="shrink-0 rounded-full border border-neutral-200 px-3 py-1 text-xs text-neutral-500 transition-[border-color,color,scale] duration-150 hover:border-red-300 hover:text-red-600 active:scale-[0.96] disabled:opacity-40 dark:border-neutral-800 dark:hover:border-red-900 dark:hover:text-red-400"
                >
                  {busy === s.path ? 'Removing…' : 'Remove'}
                </button>
              </div>
            ))
          )}
        </div>

        <form
          class="mt-6 border-t border-neutral-200 pt-5 dark:border-neutral-800"
          onSubmit={(e) => {
            e.preventDefault();
            void add();
          }}
        >
          <p class="text-sm font-medium">Add a folder or file</p>
          <div class="mt-3 flex gap-2">
            <div class="relative min-w-0 flex-1">
              <input
                ref={pathInput}
                type="text"
                value={path}
                onInput={(e) => {
                  const v = (e.target as HTMLInputElement).value;
                  setPath(v);
                  complete(v);
                }}
                onKeyDown={onPathKey}
                onBlur={() => window.setTimeout(() => setSugg([]), 150)}
                placeholder="/absolute/path (suggests as you type)"
                aria-label="Path to index"
                autocomplete="off"
                spellcheck={false}
                class="w-full rounded-xl border border-neutral-200 bg-transparent px-3.5 py-2 font-mono text-[13px] outline-none focus:border-neutral-400 dark:border-neutral-800 dark:focus:border-neutral-600"
              />
              {sugg.length > 0 ? (
                <div class="absolute inset-x-0 top-full z-10 mt-1 overflow-hidden rounded-xl border border-neutral-200 bg-white py-1 shadow-lg dark:border-neutral-800 dark:bg-neutral-950">
                  {sugg.map((s, i) => (
                    <button
                      key={s.path}
                      type="button"
                      // mousedown, not click: it fires before the input's
                      // blur closes the dropdown.
                      onMouseDown={(e) => {
                        e.preventDefault();
                        accept(s);
                      }}
                      class={`block w-full truncate px-3.5 py-1.5 text-left font-mono text-[13px] ${
                        i === suggIdx ? 'bg-neutral-100 dark:bg-neutral-900' : ''
                      } ${s.dir ? '' : 'text-neutral-500'} hover:bg-neutral-100 dark:hover:bg-neutral-900`}
                    >
                      {s.name}
                      {s.dir ? '/' : ''}
                    </button>
                  ))}
                </div>
              ) : null}
            </div>
            <button
              type="button"
              onClick={() => {
                setSugg([]);
                setPicking(true);
              }}
              class="rounded-xl border border-neutral-200 px-3.5 py-2 text-sm transition-[border-color,scale] duration-150 hover:border-neutral-400 active:scale-[0.96] dark:border-neutral-800 dark:hover:border-neutral-600"
            >
              Browse…
            </button>
          </div>
          {/* Tags only matter outside this UI (hay search --tag, the MCP
              tag argument), so the field stays folded away by default. */}
          {tagOpen ? (
            <div class="mt-2">
              <input
                type="text"
                value={tag}
                onInput={(e) => setTag((e.target as HTMLInputElement).value)}
                placeholder="tag, e.g. acme-litigation"
                aria-label="Tag"
                spellcheck={false}
                class="w-full rounded-xl border border-neutral-200 bg-transparent px-3.5 py-2 text-sm outline-none focus:border-neutral-400 dark:border-neutral-800 dark:focus:border-neutral-600"
              />
              <p class="mt-1.5 text-xs text-neutral-400 dark:text-neutral-500">
                Lets the CLI and MCP tools search just this source, like{' '}
                <code class="font-mono text-[11px]">hay search --tag acme-litigation</code>.
              </p>
            </div>
          ) : null}
          <div class="mt-3 flex items-center justify-between gap-3">
            <button
              type="button"
              onClick={() => {
                if (tagOpen) setTag('');
                setTagOpen(!tagOpen);
              }}
              class="text-sm text-neutral-400 transition-colors hover:text-neutral-900 dark:hover:text-neutral-100"
            >
              {tagOpen ? 'No tag' : '+ Tag this source'}
            </button>
            <button
              type="submit"
              disabled={!path.trim() || indexing !== ''}
              class="rounded-full bg-neutral-900 px-5 py-2 text-sm font-medium text-white transition-[opacity,scale] duration-150 hover:opacity-90 active:scale-[0.96] disabled:opacity-40 dark:bg-neutral-100 dark:text-neutral-900"
            >
              {indexing !== '' ? 'Indexing…' : 'Add'}
            </button>
          </div>
          {indexing ? (
            <p class="mt-3 text-sm text-neutral-500">
              Indexing <span class="font-mono text-[12.5px]">{indexing}</span>. You can close
              this window; indexing continues and search stays available.
            </p>
          ) : indexNote ? (
            <p class="mt-3 text-sm text-neutral-500">{indexNote}</p>
          ) : null}
          {note ? <p class="mt-3 text-sm text-red-600 dark:text-red-400">{note}</p> : null}
        </form>

        {picking ? (
          <FolderBrowser
            startPath={path.trim()}
            nativeOk={nativeOk}
            onNative={nativePick}
            onPick={(p) => {
              setPath(p);
              setPicking(false);
            }}
            onClose={() => setPicking(false)}
          />
        ) : null}
      </div>
    </div>
  );
}

function FolderBrowser({
  startPath,
  nativeOk,
  onNative,
  onPick,
  onClose,
}: {
  startPath: string;
  nativeOk: boolean;
  onNative: () => Promise<void>;
  onPick: (path: string) => void;
  onClose: () => void;
}) {
  const [view, setView] = useState<Browse | null>(null);
  const [error, setError] = useState('');
  const [nativeBusy, setNativeBusy] = useState(false);

  const load = (path?: string) =>
    api
      .browse(path)
      .then((b) => {
        setView(b);
        setError('');
      })
      .catch((e: Error) => setError(e.message));
  // Open where the user already is: the typed path if it resolves, its
  // parent if the last segment is partial, home otherwise.
  useEffect(() => {
    const p = startPath.startsWith('/') ? startPath.replace(/\/+$/, '') || '/' : '';
    if (!p) {
      void load();
      return;
    }
    api
      .browse(p)
      .then((b) => setView(b))
      .catch(() => {
        const parent = p.slice(0, p.lastIndexOf('/')) || '/';
        void api
          .browse(parent)
          .then((b) => setView(b))
          .catch(() => load());
      });
  }, []);

  return (
    <div class="mt-4 rounded-xl border border-neutral-200 dark:border-neutral-800">
      <div class="flex items-center justify-between gap-3 border-b border-neutral-200 px-4 py-2.5 dark:border-neutral-800">
        <div class="truncate font-mono text-xs text-neutral-500">{view?.path ?? '…'}</div>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close folder browser"
          class="text-lg leading-none text-neutral-400 hover:text-neutral-900 dark:hover:text-neutral-100"
        >
          &times;
        </button>
      </div>
      <div class="max-h-56 overflow-y-auto p-2">
        {error ? <p class="px-2 py-1 text-sm text-neutral-500">{error}</p> : null}
        {view?.parent ? (
          <button
            type="button"
            onClick={() => load(view.parent)}
            class="block w-full rounded-lg px-3 py-1.5 text-left font-mono text-[13px] text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-900"
          >
            ..
          </button>
        ) : null}
        {view?.dirs.map((d) => (
          <button
            key={d.path}
            type="button"
            onClick={() => load(d.path)}
            class="block w-full truncate rounded-lg px-3 py-1.5 text-left font-mono text-[13px] hover:bg-neutral-100 dark:hover:bg-neutral-900"
          >
            {d.name}/
          </button>
        ))}
        {view?.files.map((f) => (
          <button
            key={f.path}
            type="button"
            onClick={() => onPick(f.path)}
            title="Add this file"
            class="block w-full truncate rounded-lg px-3 py-1.5 text-left font-mono text-[13px] text-neutral-500 hover:bg-neutral-100 hover:text-neutral-900 dark:hover:bg-neutral-900 dark:hover:text-neutral-100"
          >
            {f.name}
          </button>
        ))}
        {view && view.dirs.length === 0 && view.files.length === 0 && !error ? (
          <p class="px-2 py-1 text-sm text-neutral-400">Nothing indexable in here.</p>
        ) : null}
      </div>
      <div class="flex items-center justify-between gap-3 border-t border-neutral-200 px-4 py-2.5 dark:border-neutral-800">
        {nativeOk ? (
          <button
            type="button"
            disabled={nativeBusy}
            onClick={() => {
              setNativeBusy(true);
              void onNative().finally(() => setNativeBusy(false));
            }}
            class="text-xs text-neutral-400 transition-colors hover:text-neutral-900 disabled:opacity-40 dark:hover:text-neutral-100"
          >
            {nativeBusy ? 'Opening…' : 'System picker…'}
          </button>
        ) : (
          <span />
        )}
        <button
          type="button"
          disabled={!view}
          onClick={() => view && onPick(view.path)}
          class="rounded-full bg-neutral-900 px-4 py-1.5 text-xs font-medium text-white transition-[opacity,scale] duration-150 hover:opacity-90 active:scale-[0.96] disabled:opacity-40 dark:bg-neutral-100 dark:text-neutral-900"
        >
          Use this folder
        </button>
      </div>
    </div>
  );
}

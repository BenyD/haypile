/* Typed client for the daemon's localhost API. */

export type SearchResult = {
  path: string;
  page?: number;
  chunk: number;
  snippet: string;
  score: number;
  label?: string;
};

export type Source = { path: string; tag?: string; files: number; chunks: number };

export type Status = {
  ok: boolean;
  version: string;
  model?: string;
  files: number;
  chunks: number;
  outbound_connections: number;
  sources?: Source[] | null;
};

export type Passage = { chunk: number; page?: number; text: string; current: boolean };

export type Browse = {
  path: string;
  parent?: string;
  dirs: { name: string; path: string }[];
  files: { name: string; path: string }[];
};

export type IndexStats = {
  Indexed: number;
  Skipped: number;
  Failed: number;
  Chunks: number;
  ScanSkipped: number;
};

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

async function call<T>(input: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(input, init);
  if (!resp.ok) {
    let message = `the daemon answered ${resp.status}`;
    try {
      const body = await resp.json();
      if (body.error) message = body.error;
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError(message, resp.status);
  }
  return resp.json() as Promise<T>;
}

const post = (body: unknown): RequestInit => ({
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
});

export const api = {
  status: () => call<Status>('/api/status'),
  query: (query: string, limit = 10) =>
    call<{ results: SearchResult[] | null }>('/api/query', post({ query, limit })),
  chunk: (path: string, chunk: number, window = 1) =>
    call<{ path: string; passages: Passage[] }>(
      `/api/chunk?path=${encodeURIComponent(path)}&chunk=${chunk}&window=${window}`,
    ),
  browse: (path?: string) =>
    call<Browse>(`/api/browse${path ? `?path=${encodeURIComponent(path)}` : ''}`),
  sources: () => call<{ sources: Source[] | null }>('/api/sources'),
  /* Opens the OS file dialog via the daemon (a native process may; a web
     page may not). Resolves null on cancel; throws ApiError 501 when the
     platform has no dialog, which callers treat as "use the in-page
     browser". */
  pick: async (kind: 'folder' | 'file'): Promise<string | null> => {
    const resp = await fetch(`/api/pick?kind=${kind}`, { method: 'POST' });
    if (resp.status === 204) return null;
    if (!resp.ok) {
      let message = `the daemon answered ${resp.status}`;
      try {
        const body = await resp.json();
        if (body.error) message = body.error;
      } catch {
        /* non-JSON error body */
      }
      throw new ApiError(message, resp.status);
    }
    return ((await resp.json()) as { path: string }).path;
  },
  addSource: (path: string, tag: string) =>
    call<IndexStats>('/api/sources', post({ path, tag })),
  removeSource: (path: string) =>
    call<{ removed: boolean }>('/api/sources', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    }),
};

export type AskHandlers = {
  sources: (s: SearchResult[]) => void;
  token: (t: string) => void;
  error: (message: string) => void;
  done: () => void;
};

/* POST /api/ask streams server-sent events; fetch + a small parser since
   EventSource cannot POST. */
export async function askStream(
  question: string,
  on: AskHandlers,
  signal: AbortSignal,
): Promise<void> {
  const resp = await fetch('/api/ask', { ...post({ question }), signal });
  if (!resp.ok) {
    let message = `the daemon answered ${resp.status}`;
    try {
      const body = await resp.json();
      if (body.error) message = body.error;
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError(message, resp.status);
  }

  const reader = resp.body!.getReader();
  const decoder = new TextDecoder();
  let buf = '';
  const handle = (block: string) => {
    let event = 'message';
    let data = '';
    for (const line of block.split('\n')) {
      if (line.startsWith('event: ')) event = line.slice(7);
      else if (line.startsWith('data: ')) data += line.slice(6);
    }
    if (!data) return;
    const payload = JSON.parse(data);
    if (event === 'sources') on.sources(payload);
    else if (event === 'token') on.token(payload);
    else if (event === 'error') on.error(payload.message);
    else if (event === 'done') on.done();
  };

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    let sep;
    while ((sep = buf.indexOf('\n\n')) >= 0) {
      handle(buf.slice(0, sep));
      buf = buf.slice(sep + 2);
    }
  }
}

/* Chunk text keeps markdown heading markers because the retriever wants
   them as context; readers do not. Strip them for display only. */
export function plainText(s: string): string {
  return s.replace(/^#{1,6}\s+/gm, '');
}

/* Extracted text (PDFs especially) keeps the source's hard line wraps,
   which read as clutter on screen. Reflow: blank lines stay paragraph
   breaks, single newlines become spaces, and bullet markers restart
   their own line so lists survive the reflow. */
export function flowText(s: string): string {
  return s
    .replace(/\r/g, '')
    .split(/\n{2,}/)
    .map((p) => p.replace(/\s+/g, ' ').trim().replace(/\s+([•·▪‣])\s*/g, '\n$1 '))
    .filter(Boolean)
    .join('\n\n');
}

/* headingOf pulls the section name out of a chunk that starts with a
   markdown heading. Chunks are split at headings, so most carry one. */
export function headingOf(text?: string): string | null {
  const m = text?.match(/^#{1,6}\s+(.+)/);
  return m ? m[1].trim() : null;
}

/* citeLabel names a citation the way a person would look it up: file
   plus its real page or heading; the chunk ordinal only as a last
   resort for formats with neither. */
export function citeLabel(r: SearchResult): string {
  const name = r.path.slice(r.path.lastIndexOf('/') + 1);
  if (r.page) return `${name}, page ${r.page}`;
  const heading = headingOf(r.snippet);
  if (heading) return `${name}, ${heading}`;
  return r.label || `${name}, section ${r.chunk + 1}`;
}

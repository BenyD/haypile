import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  ApiError,
  askStream,
  citeLabel,
  flowText,
  headingOf,
  plainText,
  type SearchResult,
} from './api';

/* askStream hand-rolls SSE parsing over fetch (EventSource cannot
   POST), so the framing rules live here: events split on blank lines,
   multiple data: lines concatenate, and chunk boundaries from the
   network can land anywhere, including mid-token. */

function streamOf(chunks: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder();
  return new ReadableStream({
    start(controller) {
      for (const c of chunks) controller.enqueue(enc.encode(c));
      controller.close();
    },
  });
}

function recorder() {
  const calls: unknown[][] = [];
  return {
    calls,
    on: {
      sources: (s: SearchResult[]) => calls.push(['sources', s]),
      token: (t: string) => calls.push(['token', t]),
      error: (m: string) => calls.push(['error', m]),
      done: () => calls.push(['done']),
    },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('askStream', () => {
  it('delivers sources, tokens, and done in order', async () => {
    const body = streamOf([
      'event: sources\ndata: [{"path":"/a.pdf","page":4,"chunk":1,"snippet":"x","score":1}]\n\n',
      'event: token\ndata: "Hello"\n\n',
      'event: token\ndata: " world"\n\n',
      'event: done\ndata: {}\n\n',
    ]);
    vi.stubGlobal('fetch', async () => new Response(body, { status: 200 }));

    const r = recorder();
    await askStream('q', r.on, new AbortController().signal);
    expect(r.calls).toEqual([
      ['sources', [{ path: '/a.pdf', page: 4, chunk: 1, snippet: 'x', score: 1 }]],
      ['token', 'Hello'],
      ['token', ' world'],
      ['done'],
    ]);
  });

  it('survives chunk boundaries anywhere, including mid-token', async () => {
    const body = streamOf([
      'event: tok',
      'en\ndata: "Hel',
      'lo"\n',
      '\nevent: done\ndata: {}\n\n',
    ]);
    vi.stubGlobal('fetch', async () => new Response(body, { status: 200 }));

    const r = recorder();
    await askStream('q', r.on, new AbortController().signal);
    expect(r.calls).toEqual([['token', 'Hello'], ['done']]);
  });

  it('concatenates multiple data lines in one event', async () => {
    const body = streamOf(['event: token\ndata: "ab\ndata: c"\n\n']);
    vi.stubGlobal('fetch', async () => new Response(body, { status: 200 }));

    const r = recorder();
    await askStream('q', r.on, new AbortController().signal);
    expect(r.calls).toEqual([['token', 'abc']]);
  });

  it('surfaces error events through the error handler', async () => {
    const body = streamOf(['event: error\ndata: {"message":"model went away"}\n\n']);
    vi.stubGlobal('fetch', async () => new Response(body, { status: 200 }));

    const r = recorder();
    await askStream('q', r.on, new AbortController().signal);
    expect(r.calls).toEqual([['error', 'model went away']]);
  });

  it('throws ApiError with the server message on a non-200 reply', async () => {
    vi.stubGlobal(
      'fetch',
      async () => new Response(JSON.stringify({ error: 'no LLM endpoint' }), { status: 503 }),
    );

    const r = recorder();
    await expect(askStream('q', r.on, new AbortController().signal)).rejects.toMatchObject({
      message: 'no LLM endpoint',
      status: 503,
    });
    expect(r.calls).toEqual([]);
  });
});

describe('plainText', () => {
  it('strips markdown heading markers, keeps body text', () => {
    expect(plainText('# Title\nbody\n## Sub\nmore')).toBe('Title\nbody\nSub\nmore');
  });
  it('leaves hashtags without a space alone', () => {
    expect(plainText('#nofilter')).toBe('#nofilter');
  });
});

describe('flowText', () => {
  it('joins wrapped lines and keeps paragraph breaks', () => {
    expect(flowText('one\ntwo\n\nthree')).toBe('one two\n\nthree');
  });
  it('drops carriage returns', () => {
    expect(flowText('a\r\nb')).toBe('a b');
  });
  it('restarts bullets on their own line', () => {
    expect(flowText('intro • first • second')).toBe('intro\n• first\n• second');
  });
});

describe('headingOf', () => {
  it('pulls the heading a chunk starts with', () => {
    expect(headingOf('## Section Two\ntext')).toBe('Section Two');
  });
  it('returns null without a leading heading', () => {
    expect(headingOf('plain text')).toBeNull();
    expect(headingOf(undefined)).toBeNull();
  });
});

describe('citeLabel', () => {
  const base: SearchResult = { path: '/docs/msa.pdf', chunk: 10, snippet: '', score: 1 };
  it('prefers the real page', () => {
    expect(citeLabel({ ...base, page: 4, snippet: '# Ignored' })).toBe('msa.pdf, page 4');
  });
  it('falls back to the chunk heading', () => {
    expect(citeLabel({ ...base, path: '/notes.md', snippet: '# Intro\nbody' })).toBe(
      'notes.md, Intro',
    );
  });
  it('uses the server label before the ordinal', () => {
    expect(citeLabel({ ...base, label: 'msa.pdf, part 2' })).toBe('msa.pdf, part 2');
  });
  it('last resort is the 1-based section ordinal', () => {
    expect(citeLabel(base)).toBe('msa.pdf, section 11');
  });
});

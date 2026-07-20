import type { ComponentChildren } from 'preact';

/* A deliberately small markdown renderer for passage previews: headings,
   lists, blockquotes, paragraphs, and inline bold/italic/code. It emits
   Preact nodes, never HTML strings, so document content cannot inject
   markup no matter what it contains. Anything unrecognized stays text. */

function inline(text: string): ComponentChildren[] {
  const out: ComponentChildren[] = [];
  // `code`, **bold**, *em* or _em_
  const re = /(`[^`]+`|\*\*[^*]+\*\*|\*[^*\s][^*]*\*|_[^_\s][^_]*_)/g;
  let last = 0;
  for (const m of text.matchAll(re)) {
    if (m.index! > last) out.push(text.slice(last, m.index));
    const tok = m[0];
    if (tok.startsWith('`')) {
      out.push(
        <code class="rounded bg-neutral-100 px-1 py-0.5 font-mono text-[12px] dark:bg-neutral-800">
          {tok.slice(1, -1)}
        </code>,
      );
    } else if (tok.startsWith('**')) {
      out.push(<strong class="font-semibold">{tok.slice(2, -2)}</strong>);
    } else {
      out.push(<em>{tok.slice(1, -1)}</em>);
    }
    last = m.index! + tok.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

export function Markdown({ text }: { text: string }) {
  const blocks: ComponentChildren[] = [];
  const lines = text.split('\n');
  let para: string[] = [];
  let list: string[] = [];

  const flushPara = () => {
    if (para.length) {
      blocks.push(<p class="mb-2 last:mb-0">{inline(para.join(' '))}</p>);
      para = [];
    }
  };
  const flushList = () => {
    if (list.length) {
      blocks.push(
        <ul class="mb-2 list-disc space-y-0.5 ps-5 last:mb-0">
          {list.map((item, i) => (
            <li key={i}>{inline(item)}</li>
          ))}
        </ul>,
      );
      list = [];
    }
  };

  for (const raw of lines) {
    const line = raw.trimEnd();
    const heading = line.match(/^(#{1,6})\s+(.*)/);
    const item = line.match(/^\s*(?:[-*+]|\d+\.)\s+(.*)/);
    if (heading) {
      flushPara();
      flushList();
      blocks.push(
        <p class="mb-1.5 mt-3 font-semibold first:mt-0 last:mb-0">{inline(heading[2])}</p>,
      );
    } else if (item) {
      flushPara();
      list.push(item[1]);
    } else if (line.startsWith('> ')) {
      flushPara();
      flushList();
      blocks.push(
        <blockquote class="mb-2 border-s-2 border-neutral-200 ps-3 text-neutral-500 last:mb-0 dark:border-neutral-700">
          {inline(line.slice(2))}
        </blockquote>,
      );
    } else if (line === '') {
      flushPara();
      flushList();
    } else {
      flushList();
      para.push(line);
    }
  }
  flushPara();
  flushList();

  return <div>{blocks}</div>;
}

/* Markdown formats get rendered; everything else is already prose. */
export function isMarkdownPath(path: string): boolean {
  return /\.(md|markdown)$/i.test(path);
}

'use client';

import { useEffect, useRef, useState } from 'react';

/* The hero terminal: runs an attract-mode demo on loop until the
   visitor clicks, then becomes an interactive simulated shell. Nothing
   real is indexed; outputs are canned but query-aware. */

type Row = { prompt: boolean; text: string; bright?: boolean };
type Step = { kind: 'cmd'; text: string } | { kind: 'out'; lines: string[]; bright?: boolean };

const demo: Step[] = [
  { kind: 'cmd', text: 'hay add ~/Documents' },
  { kind: 'out', lines: ['Indexed 214 files (1,892 chunks).', 'Embedded 1,892 chunks for semantic search.', ''] },
  { kind: 'cmd', text: 'hay search "agreement cancellation"' },
  {
    kind: 'out',
    lines: [
      ' 1. vendor-deal.docx (chunk 2)',
      '    Either party may terminate this',
      '    Agreement with sixty days notice.',
      ' 2. vendor-agreement.pdf (page 1)',
      '    This Agreement may be terminated...',
      '',
    ],
  },
  { kind: 'cmd', text: 'hay ask "what notice period applies?"' },
  {
    kind: 'out',
    lines: [
      'The vendor agreement requires a 60-day',
      'written notice period for termination [1].',
      '',
      'Sources:',
      '  [1] vendor-deal.docx (chunk 2)',
      '',
    ],
  },
  { kind: 'cmd', text: 'hay status' },
  { kind: 'out', lines: ['Outbound connections: 0'], bright: true },
];

/* Commands the interactive shell can ghost-complete, in suggestion
   priority order. */
const completions = [
  'hay help',
  'hay add ~/Documents',
  'hay search "agreement cancellation"',
  'hay ask "what notice period applies?"',
  'hay status',
  'hay list',
  'clear',
];

const TYPE_MS = 38;
const OUT_LINE_MS = 90;
const AFTER_CMD_MS = 420;
const AFTER_OUT_MS = 900;
const LOOP_PAUSE_MS = 3200;

function quoted(input: string): string {
  const m = input.match(/"([^"]*)"/);
  return m ? m[1] : input.split(' ').slice(2).join(' ');
}

/* The simulated shell. Query-aware where it can be, honest that it is
   a demo where it cannot. */
function run(line: string): Row[] {
  const out = (lines: string[], bright = false): Row[] =>
    lines.map((text) => ({ prompt: false, text, bright }));
  const cmd = line.trim();
  if (cmd === '') return [];
  if (cmd === 'hay help' || cmd === 'hay' || cmd === 'help')
    return out([
      'This is a live demo shell. Try:',
      '  hay add <folder>        index a folder',
      '  hay search "<query>"    search by meaning',
      '  hay ask "<question>"    answer with citations',
      '  hay status              the privacy receipt',
      '  hay list                indexed folders',
      '  clear                   wipe the screen',
      '',
    ]);
  if (cmd === 'clear') return [{ prompt: false, text: '\u0000CLEAR' }];
  if (cmd.startsWith('hay add')) {
    const path = cmd.replace('hay add', '').trim() || '~/Documents';
    return out([
      `Indexed 214 files (1,892 chunks) from ${path}.`,
      'Embedded 1,892 chunks for semantic search.',
      `Watching ${path} for changes.`,
      '',
    ]);
  }
  if (cmd.startsWith('hay search')) {
    const q = quoted(cmd) || 'agreement cancellation';
    return out([
      ` 1. vendor-deal.docx (chunk 2)`,
      `    Either party may terminate this`,
      `    Agreement with sixty days notice.`,
      ` 2. meridian-msa.pdf (page 4)`,
      `    ...closest match for "${q.slice(0, 28)}"`,
      '',
    ]);
  }
  if (cmd.startsWith('hay ask')) {
    const q = quoted(cmd);
    return [
      ...out(['Answering with llama3.2 (localhost:11434)...', '']),
      ...out(
        [
          q
            ? `On "${q.slice(0, 40)}": the vendor agreement`
            : 'The vendor agreement',
          'requires a 60-day written notice period [1].',
          '',
        ],
        true,
      ),
      ...out(['Sources:', '  [1] vendor-deal.docx (chunk 2)', '']),
    ];
  }
  if (cmd === 'hay status')
    return [
      ...out([
        'Daemon:   running (demo)',
        'Indexed:  1 source, 214 files',
        'Model:    bundled/all-MiniLM-L6-v2',
      ]),
      ...out(['Outbound connections: 0'], true),
      ...out(['']),
    ];
  if (cmd === 'hay list')
    return out(['FOLDER          TAG    FILES  CHUNKS', '~/Documents            214    1,892', '']);
  if (cmd.startsWith('brew install'))
    return out(['That one belongs in your real terminal :)', '']);
  return out([`command not found: ${cmd.split(' ')[0]} (try hay help)`, '']);
}

export function HeroTerminal() {
  const [rows, setRows] = useState<Row[]>([]);
  const [typing, setTyping] = useState<string | null>(null);
  const [interactive, setInteractive] = useState(false);
  const [input, setInput] = useState('');
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const inputRef = useRef<HTMLInputElement>(null);
  const scroller = useRef<HTMLPreElement>(null);
  const live = useRef(true);
  const history = useRef<string[]>([]);
  const histIdx = useRef<number | null>(null);

  const suggestion = interactive && input ? completions.find((c) => c.startsWith(input) && c !== input) : undefined;
  const ghost = suggestion ? suggestion.slice(input.length) : '';

  // Attract mode: the looping demo.
  useEffect(() => {
    if (interactive) return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      setRows(
        demo.flatMap((s): Row[] =>
          s.kind === 'cmd'
            ? [{ prompt: true, text: s.text }]
            : s.lines.map((l) => ({ prompt: false, text: l, bright: s.bright })),
        ),
      );
      return;
    }

    let step = 0;
    let char = 0;
    let line = 0;
    let done: Row[] = [];
    live.current = true;

    const tick = () => {
      if (!live.current) return;
      const s = demo[step];
      if (!s) {
        timer.current = setTimeout(() => {
          step = 0;
          char = 0;
          line = 0;
          done = [];
          setRows([]);
          setTyping(null);
          tick();
        }, LOOP_PAUSE_MS);
        return;
      }
      if (s.kind === 'cmd') {
        if (char <= s.text.length) {
          setTyping(s.text.slice(0, char));
          char += 1;
          timer.current = setTimeout(tick, TYPE_MS);
        } else {
          done = [...done, { prompt: true, text: s.text }];
          setRows(done);
          setTyping(null);
          char = 0;
          step += 1;
          timer.current = setTimeout(tick, AFTER_CMD_MS);
        }
      } else {
        if (line < s.lines.length) {
          done = [...done, { prompt: false, text: s.lines[line], bright: s.bright }];
          setRows(done);
          line += 1;
          timer.current = setTimeout(tick, OUT_LINE_MS);
        } else {
          line = 0;
          step += 1;
          timer.current = setTimeout(tick, AFTER_OUT_MS);
        }
      }
    };

    tick();
    return () => {
      live.current = false;
      clearTimeout(timer.current);
    };
  }, [interactive]);

  // Interactive mode entry.
  const goInteractive = () => {
    if (interactive) return;
    live.current = false;
    clearTimeout(timer.current);
    setTyping(null);
    setRows([
      { prompt: false, text: 'Live demo shell. Nothing here touches your files.' },
      { prompt: false, text: '' },
      { prompt: false, text: 'Try:  hay search "agreement cancellation"' },
      { prompt: false, text: '      hay ask "what notice period applies?"' },
      { prompt: false, text: '      hay status' },
      { prompt: false, text: '' },
      { prompt: false, text: 'Tab completes. Type hay help for everything.' },
      { prompt: false, text: '' },
    ]);
    setInteractive(true);
    // preventScroll: focusing must not yank the page when the terminal is
    // partly scrolled off-screen.
    setTimeout(() => inputRef.current?.focus({ preventScroll: true }), 0);
  };

  const submit = () => {
    const result = run(input);
    if (input.trim()) history.current = [...history.current, input];
    histIdx.current = null;
    if (result.some((r) => r.text === '\u0000CLEAR')) {
      setRows([]);
    } else {
      setRows((prev) => [...prev, { prompt: true, text: input }, ...result]);
    }
    setInput('');
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      submit();
      return;
    }
    // Tab (or right arrow at the end of the line) accepts the ghost
    // suggestion, fish style.
    if (
      ghost &&
      (e.key === 'Tab' ||
        (e.key === 'ArrowRight' && e.currentTarget.selectionStart === input.length))
    ) {
      e.preventDefault();
      setInput(suggestion!);
      return;
    }
    if (e.key === 'Tab') {
      e.preventDefault();
      return;
    }
    // Arrow keys walk command history.
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      const h = history.current;
      if (!h.length) return;
      histIdx.current = histIdx.current === null ? h.length - 1 : Math.max(0, histIdx.current - 1);
      setInput(h[histIdx.current]);
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      const h = history.current;
      if (histIdx.current === null) return;
      histIdx.current = histIdx.current + 1 >= h.length ? null : histIdx.current + 1;
      setInput(histIdx.current === null ? '' : h[histIdx.current]);
    }
  };

  // Keep the newest lines visible.
  useEffect(() => {
    const el = scroller.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [rows, typing, input]);

  return (
    <div
      className="flex w-full cursor-text justify-center overflow-hidden rounded-2xl border bg-cover bg-center p-5 sm:p-16 sm:py-24"
      style={{ backgroundImage: "url('/hero-bg.jpg')" }}
      onClick={interactive ? () => inputRef.current?.focus({ preventScroll: true }) : goInteractive}
    >
      <div className="relative w-full max-w-5xl overflow-hidden rounded-xl border bg-fd-background text-left">
      <div className="relative flex items-center gap-1.5 border-b px-4 py-2.5">
        <span className="size-2.5 rounded-full bg-[#ff5f57]" />
        <span className="size-2.5 rounded-full bg-[#febc2e]" />
        <span className="size-2.5 rounded-full bg-[#28c840]" />
        <span className="pointer-events-none absolute inset-x-0 text-center font-mono text-xs text-fd-muted-foreground">
          haypile
        </span>
      </div>
      <pre
        ref={scroller}
        className="h-[30rem] overflow-y-auto p-5 font-mono text-[13px] leading-6"
        style={{ scrollbarWidth: interactive ? undefined : 'none' }}
        aria-label="Haypile interactive demo"
      >
        <code className="flex min-h-full flex-col justify-end">
          {rows.map((r, i) => (
            <span key={i} className={r.prompt || r.bright ? undefined : 'text-fd-muted-foreground'}>
              {r.prompt ? '$ ' : ''}
              {r.text}
              {'\n'}
            </span>
          ))}
          {!interactive && typing !== null ? (
            <span>
              {'$ '}
              {typing}
              <span className="animate-pulse">▍</span>
            </span>
          ) : null}
          {interactive ? (
            <span className="flex">
              {'$ '}
              <span className="relative flex-1">
                {ghost ? (
                  <span aria-hidden className="pointer-events-none absolute inset-0">
                    <span className="invisible">{input}</span>
                    <span className="text-fd-muted-foreground/60">{ghost}</span>
                  </span>
                ) : null}
                <input
                  ref={inputRef}
                  value={input}
                  onChange={(e) => {
                    histIdx.current = null;
                    setInput(e.target.value);
                  }}
                  onKeyDown={onKeyDown}
                  className="w-full bg-transparent font-mono outline-none"
                  aria-label="Type a demo command"
                  spellCheck={false}
                  autoComplete="off"
                  autoCapitalize="none"
                  autoCorrect="off"
                />
              </span>
            </span>
          ) : null}
        </code>
      </pre>
      </div>
    </div>
  );
}

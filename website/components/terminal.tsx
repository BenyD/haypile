/* A static terminal window: renders a finished session transcript.
   No animation, no client JS. Used by the feature rows. */

export type Step =
  | { kind: 'cmd'; text: string }
  | { kind: 'out'; lines: string[]; bright?: boolean };

type Row = { prompt: boolean; text: string; bright?: boolean };

function transcript(script: Step[]): Row[] {
  return script.flatMap((s): Row[] =>
    s.kind === 'cmd'
      ? [{ prompt: true, text: s.text }]
      : s.lines.map((l) => ({ prompt: false, text: l, bright: s.bright })),
  );
}

export function Terminal({ script }: { script: Step[] }) {
  return (
    <div className="w-full overflow-hidden rounded-xl border bg-fd-background text-left">
      <div className="flex items-center gap-1.5 border-b px-4 py-2.5">
        <span className="size-2.5 rounded-full bg-[#ff5f57]" />
        <span className="size-2.5 rounded-full bg-[#febc2e]" />
        <span className="size-2.5 rounded-full bg-[#28c840]" />
      </div>
      <pre className="overflow-x-auto p-5 font-mono text-[13px] leading-6" aria-label="Haypile example session">
        <code>
          {transcript(script).map((r, i) => (
            <span
              key={i}
              className={r.prompt || r.bright ? undefined : 'text-fd-muted-foreground'}
            >
              {r.prompt ? '$ ' : ''}
              {r.text}
              {'\n'}
            </span>
          ))}
        </code>
      </pre>
    </div>
  );
}

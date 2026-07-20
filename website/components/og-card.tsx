import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { Mark } from '@/app/icon';

/* The social share card, in the brand's own language: neutral dark
   surface, the haystack mark, a miniature of the landing page's hero
   terminal, and the one color the brand allows itself, the green of
   the privacy receipt. Rendered by next/og at build time. */

/* Fonts for ImageResponse, all vendored under lib/og (OFL licensed):
   Inter matches the site's own typeface and JetBrains Mono makes the
   terminal read as a real terminal. Passing a fonts array replaces
   next/og's default font, so the sans must be supplied explicitly. */
export async function ogFonts() {
  const dir = join(process.cwd(), 'lib/og');
  const [sans, sansBold, mono] = await Promise.all([
    readFile(join(dir, 'Inter-Regular.ttf')),
    readFile(join(dir, 'Inter-Bold.ttf')),
    readFile(join(dir, 'JetBrainsMono-Regular.ttf')),
  ]);
  return [
    { name: 'Inter', data: sans, style: 'normal' as const, weight: 400 as const },
    { name: 'Inter', data: sansBold, style: 'normal' as const, weight: 700 as const },
    { name: 'JetBrains Mono', data: mono, style: 'normal' as const, weight: 400 as const },
  ];
}

function TermLine({ children, color }: { children: React.ReactNode; color?: string }) {
  return (
    <div
      style={{
        display: 'flex',
        fontFamily: 'JetBrains Mono',
        fontSize: 17,
        lineHeight: 1.8,
        color: color ?? '#a1a1aa',
        whiteSpace: 'pre',
      }}
    >
      {children}
    </div>
  );
}

function Prompt({ cmd }: { cmd: string }) {
  return (
    <TermLine>
      <span style={{ color: '#52525b' }}>{'$ '}</span>
      <span style={{ color: '#fafafa' }}>{cmd}</span>
    </TermLine>
  );
}

export function OGCard({ title, description }: { title: string; description?: string }) {
  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        display: 'flex',
        background: '#0a0a0a',
        color: '#e5e5e5',
        padding: 64,
        fontFamily: 'Inter',
      }}
    >
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'space-between',
          flexGrow: 1,
          paddingRight: 56,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 18 }}>
          <div style={{ width: 48, height: 48, position: 'relative', display: 'flex' }}>
            <Mark background="transparent" fill="#fafafa" />
          </div>
          <div style={{ fontSize: 32, fontWeight: 600, color: '#fafafa' }}>Haypile</div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 20, maxWidth: 620 }}>
          <div style={{ fontSize: 54, fontWeight: 700, color: '#fafafa', lineHeight: 1.12 }}>
            {title}
          </div>
          {description ? (
            <div style={{ fontSize: 26, color: '#a3a3a3', lineHeight: 1.45 }}>{description}</div>
          ) : null}
        </div>

        <div style={{ display: 'flex', fontSize: 22, color: '#737373' }}>haypile.sh</div>
      </div>

      <div
        style={{
          width: 430,
          display: 'flex',
          flexDirection: 'column',
          alignSelf: 'center',
          borderRadius: 18,
          background: '#101013',
          border: '1px solid #27272a',
          boxShadow: '0 24px 80px rgba(0,0,0,0.55)',
        }}
      >
        <div
          style={{
            display: 'flex',
            gap: 8,
            padding: '16px 20px',
            borderBottom: '1px solid #1f1f23',
          }}
        >
          {[0, 1, 2].map((i) => (
            <div
              key={i}
              style={{ width: 12, height: 12, borderRadius: 9999, background: '#2e2e33' }}
            />
          ))}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', padding: '20px 24px 24px' }}>
          <Prompt cmd="hay add ~/Documents" />
          <TermLine>Indexed 214 files (1,892 chunks).</TermLine>
          <TermLine> </TermLine>
          <Prompt cmd={'hay ask "notice period?"'} />
          <TermLine>60 days written notice [1]</TermLine>
          <TermLine color="#71717a">{'  [1] vendor-deal.docx, p.4'}</TermLine>
          <TermLine> </TermLine>
          <Prompt cmd="hay status" />
          <TermLine color="#4ade80">Outbound connections: 0</TermLine>
        </div>
      </div>
    </div>
  );
}

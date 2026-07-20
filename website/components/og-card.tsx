import { Mark } from '@/app/icon';

/* The social share card, in the brand's own language: neutral dark
   surface, the haystack mark, and the one color the brand allows itself,
   the green of the privacy receipt. Rendered by next/og at build time. */
export function OGCard({ title, description }: { title: string; description?: string }) {
  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'space-between',
        background: '#0a0a0a',
        color: '#e5e5e5',
        padding: 72,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 20 }}>
        <div style={{ width: 52, height: 52, position: 'relative', display: 'flex' }}>
          <Mark background="transparent" fill="#fafafa" />
        </div>
        <div style={{ fontSize: 34, fontWeight: 600, color: '#fafafa' }}>Haypile</div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 20, maxWidth: 950 }}>
        <div style={{ fontSize: 62, fontWeight: 700, color: '#fafafa', lineHeight: 1.15 }}>
          {title}
        </div>
        {description ? (
          <div style={{ fontSize: 28, color: '#a3a3a3', lineHeight: 1.45 }}>{description}</div>
        ) : null}
      </div>

      <div style={{ display: 'flex', fontSize: 24, color: '#737373' }}>haypile.sh</div>
    </div>
  );
}

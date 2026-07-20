import { ImageResponse } from 'next/og';

/* PNG favicon rendered from the brand mark at build time. The SVG stays
   the primary icon; this covers surfaces that want raster (search result
   favicons, pinned tabs on older browsers). */

export const dynamic = 'force-static';
export const size = { width: 192, height: 192 };
export const contentType = 'image/png';

export default function Icon() {
  return new ImageResponse(<Mark background="transparent" fill="#09090b" />, size);
}

export function Mark({ background, fill }: { background: string; fill: string }) {
  const bar = (left: number, top: number, width: number) => (
    <div
      style={{
        position: 'absolute',
        left: `${left}%`,
        top: `${top}%`,
        width: `${width}%`,
        height: '14.2%',
        borderRadius: 9999,
        background: fill,
      }}
    />
  );
  return (
    <div style={{ width: '100%', height: '100%', display: 'flex', background }}>
      {bar(37.5, 12.5, 25)}
      {bar(25, 34.6, 50)}
      {bar(12.5, 56.7, 75)}
      <div
        style={{
          position: 'absolute',
          left: '12.5%',
          top: '78.8%',
          width: '75%',
          height: '14.2%',
          borderRadius: 9999,
          background: fill,
          opacity: 0.35,
        }}
      />
    </div>
  );
}

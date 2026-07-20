import { ImageResponse } from 'next/og';
import { OGCard, ogFonts } from '@/components/og-card';

export const revalidate = false;

export async function GET() {
  return new ImageResponse(
    (
      <OGCard
        title="Private search and Q&A for your documents"
        description="One binary. Fully local. Citations always."
      />
    ),
    { width: 1200, height: 630, fonts: await ogFonts() },
  );
}

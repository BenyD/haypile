import { ImageResponse } from 'next/og';
import { generate as DefaultImage } from 'fumadocs-ui/og';
import { appName } from '@/lib/shared';

export const revalidate = false;

export function GET() {
  return new ImageResponse(
    (
      <DefaultImage
        title="Private search and Q&A for your documents"
        description="One binary. Fully local. Citations always."
        site={appName}
      />
    ),
    { width: 1200, height: 630 },
  );
}

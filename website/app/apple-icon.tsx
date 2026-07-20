import { ImageResponse } from 'next/og';
import { Mark } from './icon';

/* iOS home screen icon: the mark on a solid tile (Apple flattens
   transparency to black, so ship the background ourselves). */

export const dynamic = 'force-static';
export const size = { width: 180, height: 180 };
export const contentType = 'image/png';

export default function AppleIcon() {
  return new ImageResponse(<Mark background="#fafafa" fill="#09090b" />, size);
}

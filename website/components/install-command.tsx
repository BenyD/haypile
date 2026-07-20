'use client';

import { useEffect, useState } from 'react';
import { CopyCommand } from '@/components/copy-command';

/* The one-liner for each platform. macOS is the prerendered default:
   the static export cannot see the visitor, so the swap happens on
   hydration via navigator. */
export const installCommands = {
  mac: 'brew install BenyD/tap/hay',
  linux: 'curl -fsSL haypile.sh | sh',
  windows: 'irm https://haypile.sh/install.ps1 | iex',
} as const;

type OS = keyof typeof installCommands;

export function detectOS(platform: string, ua: string): OS {
  const p = (platform || ua).toLowerCase();
  if (p.includes('win')) return 'windows';
  if (p.includes('mac') || /iphone|ipad/.test(p)) return 'mac';
  return 'linux';
}

/* CopyCommand that shows the visitor's own install one-liner. */
export function InstallCommand() {
  const [os, setOS] = useState<OS>('mac');

  useEffect(() => {
    const nav = navigator as Navigator & { userAgentData?: { platform?: string } };
    setOS(detectOS(nav.userAgentData?.platform ?? nav.platform ?? '', nav.userAgent));
  }, []);

  return <CopyCommand command={installCommands[os]} />;
}

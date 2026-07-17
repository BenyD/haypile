import type { Metadata } from 'next';
import Link from 'next/link';
import { LegalPage, type LegalSection } from '../legal';

export const metadata: Metadata = {
  title: 'Privacy Policy | Haypile',
  description: 'What Haypile collects: nothing. What this website collects: almost nothing.',
};

const sections: LegalSection[] = [
  {
    id: 'short-version',
    title: 'The short version',
    body: (
      <p>
        The Haypile software collects nothing. It has no telemetry, no
        analytics, no accounts, and no update checks. Your documents and your
        index never leave your machine. This is not a promise we ask you to
        trust: run <code>hay status</code> and the software reports its own
        outbound connection count, which is zero.
      </p>
    ),
  },
  {
    id: 'software',
    title: 'The software',
    body: (
      <ul>
        <li>
          Everything Haypile indexes stays in a single database file on your
          disk, under your control. We never see it, transmit it, or hold
          keys to it.
        </li>
        <li>
          Haypile makes no network requests. The embedding model ships inside
          the binary. If you choose to use <code>hay ask</code>, generation
          happens on a server running on your own machine.
        </li>
        <li>
          Optional integrations you configure, such as connecting an AI
          client over MCP, are under that client&apos;s privacy terms, not
          ours. Haypile itself still sends nothing anywhere.
        </li>
      </ul>
    ),
  },
  {
    id: 'website',
    title: 'This website',
    body: (
      <p>
        This site is a static site with no analytics scripts, no cookies set
        by us, and no tracking. Our hosting provider may keep standard,
        short-lived access logs (IP address, requested page) to operate the
        service, as virtually all web hosts do.
      </p>
    ),
  },
  {
    id: 'downloads',
    title: 'Downloads',
    body: (
      <p>
        Binaries are distributed through GitHub Releases and Homebrew.
        Downloading through them is subject to GitHub&apos;s and
        Homebrew&apos;s own privacy policies. We see only the aggregate
        download counts those platforms publish.
      </p>
    ),
  },
  {
    id: 'changes',
    title: 'Changes',
    body: (
      <p>
        If this policy ever changes, the change will be visible in the public
        repository&apos;s history, loudly. Our standing commitment: if the
        software ever gains any network feature, it will be opt-in,
        documented, and off by default.
      </p>
    ),
  },
  {
    id: 'contact',
    title: 'Contact',
    body: (
      <p>
        Questions:{' '}
        <Link href="https://github.com/BenyD/haypile/issues" className="underline underline-offset-4">
          open an issue
        </Link>{' '}
        or email hello@haypile.sh.
      </p>
    ),
  },
];

export default function PrivacyPage() {
  return <LegalPage title="Privacy Policy" updated="July 17, 2026" sections={sections} />;
}

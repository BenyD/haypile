import type { Metadata } from 'next';
import Link from 'next/link';
import { LegalPage, type LegalSection } from '../legal';

export const metadata: Metadata = {
  title: 'Terms | Haypile',
  description: 'The terms for using Haypile: AGPL-3.0, provided as is.',
};

const sections: LegalSection[] = [
  {
    id: 'software',
    title: 'The software',
    body: (
      <>
        <p>
          Haypile is open source software licensed under the{' '}
          <Link
            href="https://github.com/BenyD/haypile/blob/main/LICENSE"
            className="underline underline-offset-4"
          >
            GNU Affero General Public License v3.0
          </Link>
          . The license governs your use, modification, and distribution of
          the software; these terms do not replace or narrow it.
        </p>
        <p>
          In plain words: you may use Haypile freely, for anything, forever.
          If you modify it and offer it to others as a service, the AGPL
          requires you to share your modifications under the same license.
        </p>
      </>
    ),
  },
  {
    id: 'no-warranty',
    title: 'No warranty',
    body: (
      <p>
        As the AGPL states, the software is provided as is, without warranty
        of any kind. You are responsible for your own data, including backing
        up your documents and your index. Search and answers are produced by
        statistical models and can be wrong; citations exist so you can
        verify against the source. Do not treat any output as legal,
        medical, or financial advice.
      </p>
    ),
  },
  {
    id: 'website',
    title: 'This website',
    body: (
      <p>
        The content of this site is provided for information. We may change
        or take down the site at any time. Nothing here grants rights to the
        Haypile name or logo beyond what the license grants for the software
        itself.
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

export default function TermsPage() {
  return <LegalPage title="Terms" updated="July 17, 2026" sections={sections} />;
}

import type { Metadata } from 'next';
import Link from 'next/link';
import { Terminal, type Step } from '@/components/terminal';
import { CopyBar } from '@/components/copy-command';
import { InstallCommand } from '@/components/install-command';
import { HeroTerminal } from '@/components/hero-terminal';

export const metadata: Metadata = {
  title: { absolute: 'Haypile: fast, private search and Q&A for your documents' },
  description:
    'One binary that watches your folders, indexes every PDF, Word file, and note, and answers questions with citations. Fully local semantic search: no cloud, no Python, no vector database, zero outbound connections.',
  alternates: { canonical: '/' },
};

/* Structured data: tells search engines and AI assistants exactly what
   this is (free, open source, cross-platform desktop software). */
const jsonLd = {
  '@context': 'https://schema.org',
  '@type': 'SoftwareApplication',
  name: 'Haypile',
  applicationCategory: 'UtilitiesApplication',
  operatingSystem: 'macOS, Linux, Windows',
  description:
    'Private search and Q&A for your documents. One binary indexes PDF, Word, Markdown, and text files locally and answers questions with citations. Zero outbound network connections.',
  url: 'https://haypile.sh',
  downloadUrl: 'https://github.com/BenyD/haypile/releases',
  license: 'https://www.gnu.org/licenses/agpl-3.0.html',
  offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
  softwareRequirements: 'None. The embedding model is bundled in the binary.',
};

const features: {
  title: string;
  body: string;
  command?: string;
  href: string;
  linkText: string;
  script: Step[];
}[] = [
  {
    title: 'Everything on your disk, searchable',
    body: 'Grep matches strings and gets nothing from a PDF. Haypile extracts and embeds every document once, scanned pages included via your local vision model, then watches the folder: ask for agreement cancellation and the termination clause surfaces in milliseconds, page cited, nothing re-read.',
    href: '/docs/guides/search',
    linkText: 'Learn about search',
    script: [
      { kind: 'cmd', text: 'hay add ~/contracts' },
      {
        kind: 'out',
        lines: ['Indexed 214 files (1,892 chunks).', 'Embedded 1,892 chunks for search.', ''],
      },
      { kind: 'cmd', text: 'hay search "agreement cancellation"' },
      {
        kind: 'out',
        lines: [
          ' 1. vendor-deal.docx (chunk 2)',
          '    Either party may terminate this',
          '    Agreement with sixty days notice.',
          ' 2. vendor-agreement.pdf (page 1)',
          '    This Agreement may be terminated',
          '    by either party...',
        ],
      },
    ],
  },
  {
    title: 'Answers with receipts',
    body: 'Your local LLM answers from the retrieved passages and cites the file and page for every claim, so a wrong answer has nowhere to hide. Fully offline: it works on a plane, and on documents that are not allowed near a cloud.',
    href: '/docs/guides/ask',
    linkText: 'Set up hay ask',
    script: [
      { kind: 'cmd', text: 'hay ask "what notice period applies?"' },
      { kind: 'out', lines: ['Answering with llama3.2...', ''] },
      {
        kind: 'out',
        bright: true,
        lines: [
          'The vendor agreement requires a 60-day',
          'written notice period for termination [1].',
          '',
        ],
      },
      {
        kind: 'out',
        lines: ['Sources:', '  [1] vendor-deal.docx (chunk 2)', '  [2] meridian-msa.pdf (page 4)'],
      },
    ],
  },
  {
    title: 'Your documents stay yours',
    body: 'Client files, medical records, anything under NDA: some documents must never leave your machine. The model ships inside the binary, the index is one SQLite file on your disk, and even OCR of scanned pages runs against a model on your machine, never a cloud. hay status proves the outbound count is zero.',
    href: '/docs/explanation/privacy',
    linkText: 'How privacy is verified',
    script: [
      { kind: 'cmd', text: 'hay status' },
      {
        kind: 'out',
        lines: [
          'Daemon:   running (up 3d 4h)',
          'Index:    ~/.haypile/haypile.db',
          'Indexed:  3 sources, 1,214 files',
          'Model:    bundled/all-MiniLM-L6-v2',
        ],
      },
      { kind: 'out', lines: ['Outbound connections: 0'] },
    ],
  },
  {
    title: 'Works with your AI tools',
    body: 'Every agent session starts amnesiac, and reading 300 documents into context burns the tokens your task needed. One line gives Claude Code or Cursor an index instead: the top passages in a kilobyte, from your whole disk, fresh in every session.',
    command: 'claude mcp add --transport http haypile http://localhost:11500/mcp',
    href: '/docs/guides/claude-code',
    linkText: 'Connect Claude Code',
    script: [
      { kind: 'cmd', text: 'claude mcp add --transport http \\' },
      { kind: 'out', bright: true, lines: ['    haypile http://localhost:11500/mcp'] },
      { kind: 'out', lines: ['Added HTTP MCP server haypile', ''] },
      { kind: 'cmd', text: 'claude' },
      {
        kind: 'out',
        lines: [
          '> what does the vendor deal say',
          '  about payment?',
          '⏺ search_documents("payment terms")',
          '  Invoices are due net-45',
          '  [vendor-deal.docx (chunk 3)]...',
        ],
      },
    ],
  },
];

export default function HomePage() {
  return (
    <main className="flex flex-1 flex-col">
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      {/* Hero: headline and install first, then the product demos
          itself in a self-running terminal. */}
      <section className="flex flex-col items-center px-6 pb-28 pt-20 text-center sm:pt-24">
        <h1 className="max-w-xl text-4xl font-semibold tracking-tight sm:text-5xl sm:leading-[1.12]">
          Fast, private search and Q&A for your documents
        </h1>
        <div className="mt-12">
          <InstallCommand />
        </div>
        <p className="mt-4 text-sm text-fd-muted-foreground">
          run this in your terminal, or{' '}
          <Link
            href="https://github.com/BenyD/haypile/releases"
            className="underline underline-offset-4 hover:text-fd-foreground"
          >
            download from GitHub
          </Link>
        </p>
        <div className="mt-16 w-full max-w-6xl">
          <HeroTerminal />
        </div>
      </section>

      {/* Features in a Z pattern: each shows a finished session
          transcript. */}
      <section className="mx-auto flex w-full max-w-6xl flex-col gap-24 px-6 pb-28">
        {features.map((f, i) => (
          <div
            key={f.title}
            className="grid items-center gap-10 lg:grid-cols-2 lg:gap-16"
          >
            <div className={i % 2 === 1 ? 'lg:order-2' : undefined}>
              <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
                {f.title}
              </h2>
              <p className="mt-4 max-w-md leading-7 text-fd-muted-foreground">
                {f.body}
              </p>
              {f.command ? (
                <div className="mt-6">
                  <CopyBar command={f.command} />
                </div>
              ) : null}
              <Link
                href={f.href}
                className="mt-5 inline-block text-sm text-fd-foreground underline-offset-4 hover:underline"
              >
                {f.linkText} →
              </Link>
            </div>
            <div
              className={
                'rounded-2xl bg-fd-secondary/60 p-5 sm:p-8 ' +
                (i % 2 === 1 ? 'lg:order-1' : '')
              }
            >
              <Terminal script={f.script} />
            </div>
          </div>
        ))}
      </section>

      {/* Get started */}
      <section className="flex flex-col items-center px-6 pb-24 pt-4 text-center">
        <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          Get started with Haypile
        </h2>
        <div className="mt-8">
          <InstallCommand />
        </div>
        <p className="mt-4 text-sm text-fd-muted-foreground">
          then{' '}
          <Link
            href="/docs"
            className="underline underline-offset-4 hover:text-fd-foreground"
          >
            read the two minute tutorial
          </Link>
        </p>
      </section>
    </main>
  );
}

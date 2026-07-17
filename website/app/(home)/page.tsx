import Link from 'next/link';
import { CopyBar, CopyCommand } from '@/components/copy-command';

const features: {
  title: string;
  body: string;
  command?: string;
  href: string;
  linkText: string;
  terminal: React.ReactNode;
}[] = [
  {
    title: 'Everything on your disk, searchable',
    body: 'Grep matches strings and gets nothing from a PDF. Haypile extracts and embeds every document once, then watches the folder: ask for agreement cancellation and the termination clause surfaces in milliseconds, page cited, nothing re-read.',
    href: '/docs/guides/search',
    linkText: 'Learn about search',
    terminal: (
      <>
        <Prompt cmd="hay add ~/contracts" />
        <Dim>
          {'Indexed 214 files (1,892 chunks).\n'}
          {'Embedded 1,892 chunks for search.\n\n'}
        </Dim>
        <Prompt cmd={'hay search "agreement cancellation"'} />
        <Dim>
          {' 1. vendor-deal.docx · chunk 2\n'}
          {'    Either party may terminate this\n'}
          {'    Agreement with sixty days notice.\n'}
          {' 2. vendor-agreement.pdf · page 1\n'}
          {'    This Agreement may be terminated\n'}
          {'    by either party...'}
        </Dim>
      </>
    ),
  },
  {
    title: 'Answers with receipts',
    body: 'Your local LLM answers from the retrieved passages and cites the file and page for every claim, so a wrong answer has nowhere to hide. Fully offline: it works on a plane, and on documents that are not allowed near a cloud.',
    href: '/docs/guides/ask',
    linkText: 'Set up hay ask',
    terminal: (
      <>
        <Prompt cmd={'hay ask "what notice period applies?"'} />
        <Dim>{'Answering with llama3.2...\n\n'}</Dim>
        {'The vendor agreement requires a 60-day\n'}
        {'written notice period for termination [1].\n\n'}
        <Dim>
          {'Sources:\n'}
          {'  [1] vendor-deal.docx · chunk 2\n'}
          {'  [2] meridian-msa.pdf · page 4'}
        </Dim>
      </>
    ),
  },
  {
    title: 'Your documents stay yours',
    body: 'Client files, medical records, anything under NDA: some documents must never leave your machine. The model ships inside the binary, the index is one SQLite file on your disk, and hay status proves the outbound count is zero.',
    href: '/docs/explanation/privacy',
    linkText: 'How privacy is verified',
    terminal: (
      <>
        <Prompt cmd="hay status" />
        <Dim>
          {'Daemon:   running (up 3d 4h)\n'}
          {'Index:    ~/.haypile/haypile.db\n'}
          {'Indexed:  3 sources, 1,214 files\n'}
          {'Model:    bundled/all-MiniLM-L6-v2\n'}
        </Dim>
        {'Outbound connections: 0'}
      </>
    ),
  },
  {
    title: 'Works with your AI tools',
    body: 'Every agent session starts amnesiac, and reading 300 documents into context burns the tokens your task needed. One line gives Claude Code or Cursor an index instead: the top passages in a kilobyte, from your whole disk, fresh in every session.',
    command: 'claude mcp add --transport http haypile http://localhost:11500/mcp',
    href: '/docs/guides/claude-code',
    linkText: 'Connect Claude Code',
    terminal: (
      <>
        <Prompt cmd={'claude mcp add --transport http \\'} />
        {'    haypile http://localhost:11500/mcp\n'}
        <Dim>{'Added HTTP MCP server haypile\n\n'}</Dim>
        <Prompt cmd="claude" />
        <Dim>
          {'> what does the vendor deal say\n'}
          {'  about payment?\n'}
          {'⏺ search_documents("payment terms")\n'}
          {'  Invoices are due net-45\n'}
          {'  [vendor-deal.docx · chunk 3]...'}
        </Dim>
      </>
    ),
  },
];

export default function HomePage() {
  return (
    <main className="flex flex-1 flex-col">
      {/* Hero: headline, command, caption. */}
      <section className="flex flex-col items-center px-4 pb-28 pt-24 text-center sm:pt-32">
        <h1 className="max-w-xl text-4xl font-semibold tracking-tight sm:text-5xl sm:leading-[1.12]">
          Private search and Q&A for your documents
        </h1>
        <div className="mt-12">
          <CopyCommand command="brew install BenyD/tap/hay" />
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
      </section>

      {/* Features in a Z pattern: text and terminal swap sides each row. */}
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
              <div className="overflow-hidden rounded-xl border bg-fd-background">
                <div className="flex items-center gap-1.5 border-b px-4 py-2.5">
                  <span className="size-2.5 rounded-full bg-[#ff5f57]" />
                  <span className="size-2.5 rounded-full bg-[#febc2e]" />
                  <span className="size-2.5 rounded-full bg-[#28c840]" />
                </div>
                <pre className="overflow-x-auto p-5 text-left text-[13px] leading-6">
                  <code>{f.terminal}</code>
                </pre>
              </div>
            </div>
          </div>
        ))}
      </section>

      {/* Get started */}
      <section className="flex flex-col items-center px-4 pb-16 pt-8 text-center">
        <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          Get started with Haypile
        </h2>
        <div className="mt-8">
          <CopyCommand command="brew install BenyD/tap/hay" />
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

function Prompt({ cmd }: { cmd: string }) {
  return (
    <>
      <span className="text-fd-muted-foreground">$ </span>
      {cmd + '\n'}
    </>
  );
}

function Dim({ children }: { children: React.ReactNode }) {
  return <span className="text-fd-muted-foreground">{children}</span>;
}

import Link from 'next/link';

const features = [
  {
    title: 'Search that understands meaning',
    body: 'Hybrid retrieval merges semantic and keyword search. "Agreement cancellation" finds termination clauses; exact case numbers still match exactly.',
  },
  {
    title: 'Citations, always',
    body: 'Every result and every answer points to the source file and page. If you cannot trace it, you cannot trust it.',
  },
  {
    title: 'Verifiably private',
    body: 'Zero external connections is a feature you can check, not a promise you have to believe. Run hay status and count them yourself: the answer is 0.',
  },
  {
    title: 'Built to build on',
    body: 'REST API and MCP server on localhost:11500. Claude Code, Cursor, and your own scripts can use your documents as a knowledge source.',
  },
];

export default function HomePage() {
  return (
    <main className="flex flex-1 flex-col items-center px-4 py-16 text-center">
      <h1 className="max-w-3xl text-4xl font-bold tracking-tight sm:text-5xl">
        Private search and Q&A for your documents
      </h1>
      <p className="mt-6 max-w-2xl text-lg text-fd-muted-foreground">
        One binary that watches your folders, indexes every document, and
        answers questions about them. Fully local, fully private.
      </p>

      <pre className="mt-10 w-full max-w-xl overflow-x-auto rounded-lg border bg-fd-secondary p-4 text-left text-sm leading-7">
        <code>
          {'brew install BenyD/tap/hay\n\n'}
          {'hay add ~/Documents\n'}
          {'hay search "termination clause"\n'}
          {'hay ask "what did the contract say about termination?"'}
        </code>
      </pre>

      <div className="mt-8 flex gap-3">
        <Link
          href="/docs"
          className="rounded-full bg-fd-primary px-6 py-2.5 text-sm font-medium text-fd-primary-foreground hover:opacity-90"
        >
          Get started
        </Link>
        <Link
          href="https://github.com/BenyD/haypile"
          className="rounded-full border px-6 py-2.5 text-sm font-medium hover:bg-fd-accent"
        >
          GitHub
        </Link>
      </div>

      <div className="mt-20 grid w-full max-w-4xl grid-cols-1 gap-4 text-left sm:grid-cols-2">
        {features.map((f) => (
          <div key={f.title} className="rounded-lg border bg-fd-card p-6">
            <h2 className="font-semibold">{f.title}</h2>
            <p className="mt-2 text-sm leading-6 text-fd-muted-foreground">
              {f.body}
            </p>
          </div>
        ))}
      </div>

      <p className="mt-16 max-w-2xl text-sm text-fd-muted-foreground">
        Everyone says finding information in your files is like finding a
        needle in a haystack. Haypile is the haystack that finds its own
        needles.
      </p>
    </main>
  );
}

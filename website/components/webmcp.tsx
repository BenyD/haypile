'use client';

import { useEffect } from 'react';

/* WebMCP (https://webmachinelearning.github.io/webmcp): an agent driving
   a browser can call these instead of scraping the page.

   The docs search index is the same static Orama bundle the search
   dialog uses, so an agent gets the site's own search rather than
   whatever it can infer from the rendered HTML. */

interface Tool {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
  execute: (args: never) => Promise<{ content: { type: 'text'; text: string }[] }>;
}

interface ModelContext {
  provideContext?: (context: { tools: Tool[] }) => void;
  clearContext?: () => void;
}

const INSTALL = {
  macos: 'brew install BenyD/tap/hay',
  linux: 'curl -fsSL haypile.sh | sh',
  windows: 'irm https://haypile.sh/install.ps1 | iex',
};

const text = (body: string) => ({ content: [{ type: 'text' as const, text: body }] });

export function WebMCP() {
  useEffect(() => {
    const modelContext = (navigator as Navigator & { modelContext?: ModelContext }).modelContext;
    if (!modelContext?.provideContext) return;

    let cancelled = false;

    async function register() {
      // Loaded only once an agent is actually present, so the search
      // bundle stays off the critical path for human visitors.
      const [{ oramaStaticClient }, { create }] = await Promise.all([
        import('fumadocs-core/search/client/orama-static'),
        import('@orama/orama'),
      ]);
      if (cancelled) return;

      const client = oramaStaticClient({
        initOrama: () => create({ schema: { _: 'string' }, language: 'english' }),
      });

      modelContext?.provideContext?.({
        tools: [
          {
            name: 'search_haypile_docs',
            description:
              "Search Haypile's documentation and return matching passages with their URLs. Use for questions about installing, configuring, or connecting Haypile.",
            inputSchema: {
              type: 'object',
              properties: {
                query: { type: 'string', description: 'What to search for' },
                limit: { type: 'number', description: 'Maximum results, default 5' },
              },
              required: ['query'],
            },
            async execute({ query, limit = 5 }: { query: string; limit?: number }) {
              const results = await client.search(query);
              if (!Array.isArray(results) || results.length === 0) {
                return text(`No matches for ${JSON.stringify(query)} in the Haypile docs.`);
              }

              const lines = results.slice(0, limit).map((result) => {
                const where = result.breadcrumbs?.join(' > ') ?? '';
                // The index marks matched terms with <mark>; agents want
                // the prose, not the highlighting.
                const passage = result.content.replaceAll(/<\/?mark>/g, '');
                return `- https://haypile.sh${result.url}\n  ${where}\n  ${passage}`;
              });

              return text(lines.join('\n\n'));
            },
          },
          {
            name: 'get_haypile_install_command',
            description: 'The one-line command that installs Haypile on a given platform.',
            inputSchema: {
              type: 'object',
              properties: {
                platform: { type: 'string', enum: Object.keys(INSTALL) },
              },
              required: ['platform'],
            },
            async execute({ platform }: { platform: keyof typeof INSTALL }) {
              const command = INSTALL[platform];
              if (!command) {
                return text(`Unknown platform. Supported: ${Object.keys(INSTALL).join(', ')}.`);
              }
              return text(command);
            },
          },
        ] as unknown as Tool[],
      });
    }

    void register();

    return () => {
      cancelled = true;
      modelContext?.clearContext?.();
    };
  }, []);

  return null;
}

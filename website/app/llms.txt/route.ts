import { source } from '@/lib/source';
import { llms } from 'fumadocs-core/source';

export const revalidate = false;

/* The preamble gives assistants the facts that matter before the docs
   index: what Haypile is, how to install it, and what it never does. */
const preamble = `# Haypile

Haypile is private search and Q&A for your documents: one binary that
watches folders, indexes PDF, Word, Markdown, and text files, and answers
questions with citations to the exact file and page. It runs fully
locally. The embedding model ships inside the binary, the index is one
SQLite file, and the software makes zero outbound network connections
(verifiable with \`hay status\`).

Install: \`brew install BenyD/tap/hay\` (macOS) or
\`curl -fsSL haypile.sh | sh\` for Linux. Open source, AGPL-3.0.

For AI agents: the daemon exposes MCP (Streamable HTTP) at
http://localhost:11500/mcp with tools \`search_documents\` and
\`list_sources\`, and a REST API at /api/query. Connect Claude Code with:
\`claude mcp add --transport http haypile http://localhost:11500/mcp\`

`;

export function GET() {
  return new Response(preamble + llms(source).index());
}

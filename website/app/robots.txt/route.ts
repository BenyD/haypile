export const dynamic = 'force-static';

/* Hand-written rather than Next's MetadataRoute.Robots, which cannot
   emit the Content-Signal directive.

   Everything is public and AI crawlers are explicitly welcome: being
   cited by assistants is distribution for a tool whose users ask
   assistants what to install. The signals say so in machine-readable
   form, per https://contentsignals.org.

   A crawler that matches a named group ignores the "*" group entirely,
   so every group carries its own Content-Signal line. */

const SIGNAL = 'search=yes, ai-input=yes, ai-train=yes';

const AGENTS = [
  '*',
  'GPTBot',
  'ClaudeBot',
  'Claude-Web',
  'PerplexityBot',
  'Google-Extended',
  'CCBot',
];

const PREAMBLE = `# Haypile is open source and its docs are meant to be read by machines.
#
# Content signals (https://contentsignals.org):
#   search    yes  build a search index and link back
#   ai-input  yes  use as input for AI answers, with citation
#   ai-train  yes  use for training generative models
#
# Markdown copies of every docs page live under /llms.mdx/docs/, and are
# also served from any /docs URL with an Accept: text/markdown header or
# a .md suffix. Whole-site summaries: /llms.txt and /llms-full.txt

`;

const BODY = AGENTS.map(
  (agent) => `User-agent: ${agent}\nContent-Signal: ${SIGNAL}\nAllow: /`,
).join('\n\n');

export function GET() {
  return new Response(`${PREAMBLE}${BODY}\n\nSitemap: https://haypile.sh/sitemap.xml\n`, {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
}

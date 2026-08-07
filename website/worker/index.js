/* Serves `curl haypile.sh | sh` and `irm haypile.sh/install.ps1 | iex`,
 * and hands agents Markdown instead of HTML.
 *
 * Every request runs through this script (www redirect needs all
 * paths); assets still serve from the static build via the binding.
 * Terminal user agents get the matching install script, which ships as a
 * static asset (/install.sh, /install.ps1); browsers fall through to the
 * landing page. */

/* The build writes a Markdown twin of every docs page under /llms.mdx:
 *   /docs                 -> /llms.mdx/docs/content.md
 *   /docs/guides/search   -> /llms.mdx/docs/guides/search/content.md
 * That path is a build convention no agent would guess, so this maps the
 * real docs URL onto it. Returns null for anything outside /docs. */
function markdownPathFor(pathname) {
  const page = pathname.replace(/\.md$/, '').replace(/\/+$/, '') || '/';
  if (page !== '/docs' && !page.startsWith('/docs/')) return null;
  return `/llms.mdx${page}/content.md`;
}

/* Advertises the machine-readable copies: the API catalog and llms.txt
 * everywhere, plus the Markdown twin on docs pages. */
function linkHeader(pathname) {
  const links = [
    '</.well-known/api-catalog>; rel="api-catalog"; type="application/linkset+json"',
    '</llms.txt>; rel="llms-txt"; type="text/plain"',
  ];
  const markdown = markdownPathFor(pathname);
  if (markdown) links.push(`<${markdown}>; rel="alternate"; type="text/markdown"`);
  return links.join(', ');
}

/* Agent discovery documents, served from the Worker rather than the
 * static build for two reasons: Next's exporter and Cloudflare's asset
 * server both treat dot-prefixed directories inconsistently, and the
 * skills digest has to be taken over the exact bytes we serve.
 *
 * The source files are copied into the build by the `build` script, so
 * the version and skill text stay in step with the repo root. */

const SITE = 'https://haypile.sh';

// The skill text, as shipped, and the conventional URL it is served from.
const SKILL_ASSET = '/agent-skills/haypile/SKILL.md';
const SKILL_PATH = '/.well-known/agent-skills/haypile/SKILL.md';

const json = (body, type = 'application/json') =>
  new Response(`${JSON.stringify(body, null, 2)}\n`, {
    headers: { 'content-type': `${type}; charset=utf-8` },
  });

async function sha256Hex(bytes) {
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/* One skill, described by its own frontmatter so the index cannot drift
 * from the file it points at. The digest covers the bytes we serve. */
async function skillsIndex(env, origin) {
  const asset = await env.ASSETS.fetch(new URL(SKILL_ASSET, origin));
  if (!asset.ok) return null;

  const bytes = await asset.arrayBuffer();
  const text = new TextDecoder().decode(bytes);
  const description = text.match(/^description:\s*(.+)$/m);

  return json({
    $schema: 'https://schemas.agentskills.io/discovery/0.2.0/schema.json',
    skills: [
      {
        name: 'haypile',
        type: 'skill-md',
        description: description ? description[1].trim() : 'Local document search for agents.',
        url: `${SITE}${SKILL_PATH}`,
        digest: `sha256:${await sha256Hex(bytes)}`,
      },
    ],
  });
}

/* Built from the same server.json that goes to the MCP registry, so the
 * version never has to be updated twice.
 *
 * The endpoint is a loopback address on purpose. Haypile's MCP server
 * runs on the user's own machine, and there is no hosted one to point
 * at; an agent reading this card needs the install line first, which is
 * what the packages carry. */
async function mcpServerCard(env, origin) {
  const asset = await env.ASSETS.fetch(new URL('/mcp-server.json', origin));
  if (!asset.ok) return null;

  const server = await asset.json();

  return json({
    name: server.name,
    title: server.title,
    description: server.description,
    version: server.version,
    websiteUrl: server.websiteUrl,
    repository: server.repository,
    packages: server.packages,
    remotes: [{ type: 'streamable-http', url: 'http://localhost:11500/mcp' }],
  });
}

/* RFC 9727. The anchors are loopback for the same reason as the server
 * card: the API belongs to the reader's machine, not to this origin.
 * There is no OpenAPI document to cite, so each entry carries only the
 * human and Markdown documentation that does exist. */
function apiCatalog() {
  const entry = (anchor, doc) => ({
    anchor,
    'service-doc': [
      { href: `${SITE}${doc}`, type: 'text/html' },
      { href: `${SITE}/llms.mdx${doc}/content.md`, type: 'text/markdown' },
    ],
  });

  return json(
    {
      linkset: [
        entry('http://localhost:11500/api/query', '/docs/reference/api'),
        entry('http://localhost:11500/mcp', '/docs/guides/claude-code'),
      ],
    },
    'application/linkset+json',
  );
}

/* Returns a Response for the discovery documents, or null to let the
 * request fall through to the static build. */
async function wellKnown(pathname, env, origin) {
  if (pathname === '/.well-known/api-catalog') return apiCatalog();
  if (pathname === '/.well-known/agent-skills/index.json') return skillsIndex(env, origin);

  // Cloudflare's readiness check reads the card at the SEP-1649 path;
  // SEP-2127 moved it to /.well-known/mcp.json. Serve both.
  if (pathname === '/.well-known/mcp.json' || pathname === '/.well-known/mcp/server-card.json') {
    return mcpServerCard(env, origin);
  }

  if (pathname === SKILL_PATH) {
    const asset = await env.ASSETS.fetch(new URL(SKILL_ASSET, origin));
    if (!asset.ok) return null;
    return new Response(asset.body, {
      headers: { 'content-type': 'text/markdown; charset=utf-8' },
    });
  }

  return null;
}

export default {
  async fetch(request, env) {
    const ua = request.headers.get('user-agent') ?? '';
    const reqUrl = new URL(request.url);

    // Canonicalize in one hop: http becomes https, www becomes apex.
    // Plain-HTTP detection: the runtime may normalize request.url to
    // https behind the proxy, but cf.tlsVersion is only set when the
    // visitor actually connected over TLS.
    const overTls = Boolean(request.cf && request.cf.tlsVersion);
    if (!overTls || reqUrl.protocol === 'http:' || reqUrl.hostname === 'www.haypile.sh') {
      reqUrl.protocol = 'https:';
      if (reqUrl.hostname === 'www.haypile.sh') reqUrl.hostname = 'haypile.sh';
      return Response.redirect(reqUrl.toString(), 301);
    }

    if (/\b(curl|wget)\b/i.test(ua)) {
      const script = await env.ASSETS.fetch(new URL('/install.sh', reqUrl.origin));
      return new Response(script.body, {
        status: script.status,
        headers: { 'content-type': 'text/x-shellscript; charset=utf-8' },
      });
    }

    // PowerShell installers: an explicit /install.ps1 path, or `irm
    // haypile.sh | iex` (its user agent carries "PowerShell").
    if (reqUrl.pathname === '/install.ps1' || /\bpowershell\b/i.test(ua)) {
      const script = await env.ASSETS.fetch(new URL('/install.ps1', reqUrl.origin));
      return new Response(script.body, {
        status: script.status,
        headers: { 'content-type': 'text/plain; charset=utf-8' },
      });
    }

    if (reqUrl.pathname.startsWith('/.well-known/')) {
      const doc = await wellKnown(reqUrl.pathname, env, reqUrl.origin);
      if (doc) return doc;
    }

    // Agents asking a docs URL for Markdown get Markdown: either an
    // explicit Accept, or a .md suffix on the page path. A missing twin
    // falls through to the normal HTML response rather than 404ing.
    const accept = request.headers.get('accept') ?? '';
    if (reqUrl.pathname.endsWith('.md') || accept.includes('text/markdown')) {
      const markdown = markdownPathFor(reqUrl.pathname);
      if (markdown) {
        const doc = await env.ASSETS.fetch(new URL(markdown, reqUrl.origin));
        if (doc.ok) {
          return new Response(doc.body, {
            status: doc.status,
            headers: {
              'content-type': 'text/markdown; charset=utf-8',
              link: linkHeader(reqUrl.pathname),
            },
          });
        }
      }
    }

    // Only real pages get the headers: a 404 must not advertise a
    // Markdown twin that was never built.
    const asset = await env.ASSETS.fetch(request);
    if (!asset.ok || !(asset.headers.get('content-type') ?? '').includes('text/html')) return asset;

    const headers = new Headers(asset.headers);
    headers.set('link', linkHeader(reqUrl.pathname));
    return new Response(asset.body, {
      status: asset.status,
      statusText: asset.statusText,
      headers,
    });
  },
};

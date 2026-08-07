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

/* Advertises the machine-readable copies: llms.txt everywhere, plus the
 * Markdown twin on docs pages. */
function linkHeader(pathname) {
  const links = ['</llms.txt>; rel="llms-txt"; type="text/plain"'];
  const markdown = markdownPathFor(pathname);
  if (markdown) links.push(`<${markdown}>; rel="alternate"; type="text/markdown"`);
  return links.join(', ');
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

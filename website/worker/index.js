/* Serves `curl haypile.sh | sh` and `irm haypile.sh/install.ps1 | iex`.
 *
 * Every request runs through this script (www redirect needs all
 * paths); assets still serve from the static build via the binding.
 * Terminal user agents get the matching install script, which ships as a
 * static asset (/install.sh, /install.ps1); browsers fall through to the
 * landing page. */
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

    return env.ASSETS.fetch(request);
  },
};

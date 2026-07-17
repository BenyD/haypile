/* Serves `curl haypile.sh | sh`.
 *
 * Terminal user agents asking for the root get the install script, which
 * Pages hosts as a static asset at /install.sh (same-zone subrequests
 * bypass this Worker, so there is no recursion). Everything else passes
 * through to the website untouched. */
export default {
  async fetch(request) {
    const url = new URL(request.url);
    const ua = request.headers.get('user-agent') ?? '';

    if (url.pathname === '/' && /\b(curl|wget)\b/i.test(ua)) {
      const script = await fetch(new URL('/install.sh', url.origin));
      return new Response(script.body, {
        status: script.status,
        headers: { 'content-type': 'text/x-shellscript; charset=utf-8' },
      });
    }

    return fetch(request);
  },
};

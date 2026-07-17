/* Serves `curl haypile.sh | sh`.
 *
 * Only the root path reaches this script (run_worker_first: ["/"]).
 * Terminal user agents get the install script, which ships as a static
 * asset at /install.sh; browsers fall through to the landing page. */
export default {
  async fetch(request, env) {
    const ua = request.headers.get('user-agent') ?? '';

    if (/\b(curl|wget)\b/i.test(ua)) {
      const url = new URL(request.url);
      const script = await env.ASSETS.fetch(new URL('/install.sh', url.origin));
      return new Response(script.body, {
        status: script.status,
        headers: { 'content-type': 'text/x-shellscript; charset=utf-8' },
      });
    }

    return env.ASSETS.fetch(request);
  },
};

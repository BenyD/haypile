import type { MetadataRoute } from 'next';

export const dynamic = 'force-static';

/* Everything is public and AI crawlers are explicitly welcome: being
   cited by assistants is distribution for a tool whose users ask
   assistants what to install. */
export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      { userAgent: '*', allow: '/' },
      { userAgent: 'GPTBot', allow: '/' },
      { userAgent: 'ClaudeBot', allow: '/' },
      { userAgent: 'Claude-Web', allow: '/' },
      { userAgent: 'PerplexityBot', allow: '/' },
      { userAgent: 'Google-Extended', allow: '/' },
      { userAgent: 'CCBot', allow: '/' },
    ],
    sitemap: 'https://haypile.sh/sitemap.xml',
  };
}

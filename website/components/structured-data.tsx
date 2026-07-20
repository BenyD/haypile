/* JSON-LD for rich results: who publishes the site, what the site is,
   and the software itself (free, cross-platform, open source). Rendered
   once in the root layout; content is entirely our own constants. */
export function StructuredData() {
  const url = 'https://haypile.sh';
  const data = {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'Organization',
        '@id': `${url}/#organization`,
        name: 'Haypile',
        url,
        logo: `${url}/favicon.svg`,
      },
      {
        '@type': 'WebSite',
        '@id': `${url}/#website`,
        name: 'Haypile',
        url,
        publisher: { '@id': `${url}/#organization` },
      },
      {
        '@type': 'SoftwareApplication',
        name: 'Haypile',
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'macOS, Linux, Windows',
        url,
        downloadUrl: 'https://github.com/BenyD/haypile/releases/latest',
        description:
          'Private search and Q&A for your documents. One binary indexes your files locally and answers questions with citations; nothing leaves your machine.',
        offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
        isAccessibleForFree: true,
        license: 'https://www.gnu.org/licenses/agpl-3.0.html',
      },
    ],
  };

  return (
    <script
      type="application/ld+json"
      dangerouslySetInnerHTML={{ __html: JSON.stringify(data) }}
    />
  );
}

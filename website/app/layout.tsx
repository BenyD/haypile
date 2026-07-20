import type { Metadata, Viewport } from 'next';
import { Inter } from 'next/font/google';
import { Provider } from '@/components/provider';
import { StructuredData } from '@/components/structured-data';
import './global.css';

const inter = Inter({
  subsets: ['latin'],
});

export const metadata: Metadata = {
  metadataBase: new URL('https://haypile.sh'),
  title: {
    template: '%s | Haypile',
    default: 'Haypile: fast, private search and Q&A for your documents',
  },
  description:
    'Haypile is a local, private semantic search engine for your documents. One binary indexes your PDFs, Word files, and notes, answers questions with citations, and never sends anything off your machine.',
  applicationName: 'Haypile',
  keywords: [
    'local semantic search',
    'private document search',
    'offline RAG',
    'search PDF documents',
    'local AI document search',
    'MCP server documents',
    'self-hosted document search',
  ],
  // Each page canonicalizes to itself; the worker already canonicalizes
  // scheme and host, this covers the path.
  alternates: {
    canonical: './',
  },
  openGraph: {
    siteName: 'Haypile',
    type: 'website',
    url: 'https://haypile.sh',
    title: 'Haypile: fast, private search and Q&A for your documents',
    description:
      'One binary indexes your documents locally and answers questions with citations. Nothing leaves your machine.',
    images: ['/og/home/image.png'],
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Haypile: fast, private search and Q&A for your documents',
    description:
      'One binary indexes your documents locally and answers questions with citations. Nothing leaves your machine.',
  },
  icons: {
    icon: [
      { url: '/favicon.svg', type: 'image/svg+xml' },
      { url: '/icon', sizes: '192x192', type: 'image/png' },
    ],
    apple: '/apple-icon',
  },
};

export const viewport: Viewport = {
  colorScheme: 'light dark',
};

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="en" className={inter.className} suppressHydrationWarning>
      <body className="flex flex-col min-h-screen">
        <StructuredData />
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}

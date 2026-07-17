import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import { Provider } from '@/components/provider';
import './global.css';

const inter = Inter({
  subsets: ['latin'],
});

export const metadata: Metadata = {
  metadataBase: new URL('https://haypile.sh'),
  title: {
    template: '%s | Haypile',
    default: 'Haypile: private search and Q&A for your documents',
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
  openGraph: {
    siteName: 'Haypile',
    type: 'website',
    images: ['/og/home/image.png'],
  },
  twitter: {
    card: 'summary_large_image',
  },
  icons: {
    icon: '/favicon.svg',
  },
};

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="en" className={inter.className} suppressHydrationWarning>
      <body className="flex flex-col min-h-screen">
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}

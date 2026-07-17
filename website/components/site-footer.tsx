import Link from 'next/link';
import { ThemeToggle } from '@/components/theme-toggle';

export function SiteFooter() {
  return (
    <footer>
      <div className="mx-auto flex w-full max-w-6xl flex-col items-center justify-between gap-3 px-6 py-8 text-sm text-fd-muted-foreground sm:flex-row">
        <span>© 2026 Haypile</span>
        <div className="flex flex-wrap items-center gap-5">
          <Link href="https://github.com/BenyD/haypile" className="hover:text-fd-foreground">
            GitHub
          </Link>
          <Link href="/privacy" className="hover:text-fd-foreground">
            Privacy
          </Link>
          <Link href="/terms" className="hover:text-fd-foreground">
            Terms
          </Link>
          <ThemeToggle />
        </div>
      </div>
    </footer>
  );
}

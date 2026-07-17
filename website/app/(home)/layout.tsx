import { SiteFooter } from '@/components/site-footer';
import { SiteNav } from '@/components/site-nav';

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <div className="flex min-h-svh flex-col">
      <SiteNav />
      {children}
      <SiteFooter />
    </div>
  );
}

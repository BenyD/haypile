import { LegalToc } from '@/components/legal-toc';

export type LegalSection = {
  id: string;
  title: string;
  body: React.ReactNode;
};

/* Legal page layout: sticky section nav left, readable prose column
   right. The nav tracks scroll position. */
export function LegalPage({
  title,
  updated,
  sections,
}: {
  title: string;
  updated: string;
  sections: LegalSection[];
}) {
  return (
    <main className="mx-auto w-full max-w-5xl flex-1 px-6 py-16">
      <div className="grid gap-12 lg:grid-cols-[220px_1fr]">
        <LegalToc sections={sections.map(({ id, title }) => ({ id, title }))} />

        <article className="max-w-2xl">
          <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>
          <p className="mt-2 text-sm text-fd-muted-foreground">
            Last updated: {updated}
          </p>
          <div className="mt-12 space-y-12">
            {sections.map((s) => (
              <section key={s.id} id={s.id} className="scroll-mt-24">
                <h2 className="text-lg font-semibold tracking-tight">
                  {s.title}
                </h2>
                <div className="mt-3 space-y-3 leading-7 text-fd-muted-foreground [&_li]:mt-2 [&_ul]:list-disc [&_ul]:pl-5">
                  {s.body}
                </div>
              </section>
            ))}
          </div>
        </article>
      </div>
    </main>
  );
}

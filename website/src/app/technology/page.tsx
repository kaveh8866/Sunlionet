import Link from "next/link";
import { PageHeader } from "../../components/ui/PageHeader";
import { siteCopy } from "../../content/siteCopy";

export const dynamic = "force-static";

export default async function TechnologyPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const lang = resolved.lang === "fa" ? "fa" : "en";
  const basePrefix = resolved.lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const hrefFor = (href: string) => `${basePrefix}${href}`;
  const copy = siteCopy[lang].technology;

  return (
    <div className="mx-auto w-full max-w-5xl px-4 py-12">
      <div className="grid gap-10">
        <PageHeader
          eyebrow="SunLionet"
          title={copy.title}
          subtitle={copy.subtitle}
          actions={
            <>
              <Link
                href={hrefFor("/architecture")}
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                {siteCopy[lang].nav.architecture}
              </Link>
              <Link
                href={hrefFor("/download")}
                prefetch={false}
                className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
              >
                {siteCopy[lang].nav.download}
              </Link>
            </>
          }
        />

        <section className="rounded-3xl border border-border bg-card/60 p-8 md:p-10 shadow-[0_0_0_1px_var(--border)]">
          <ul className="list-disc ps-6 space-y-3 text-muted-foreground">
            {copy.bullets.map((b) => (
              <li key={b}>{b}</li>
            ))}
          </ul>
        </section>
      </div>
    </div>
  );
}


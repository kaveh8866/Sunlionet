import Link from "next/link";
import { notFound } from "next/navigation";
import { Callout } from "../../../../components/ui/Callout";
import { PageHeader } from "../../../../components/ui/PageHeader";
import { getContentIndex, readContentMarkdownBySlug } from "../../../../lib/content/fs";
import { renderMarkdown } from "../../../../lib/docs/markdown";

export const dynamic = "force-static";

export async function generateStaticParams() {
  const en = await getContentIndex("updates", "en");
  const fa = await getContentIndex("updates", "fa");
  return [
    ...en.map((e) => ({ lang: "en", slug: e.slug })),
    ...fa.map((e) => ({ lang: "fa", slug: e.slug })),
  ];
}

export default async function Page({ params }: { params: Promise<{ lang: string; slug: string[] }> }) {
  const resolved = await params;
  const resolvedBase = `/${resolved.lang}`;
  const isFa = resolved.lang === "fa";
  const lang = isFa ? "fa" : "en";

  const direct = await readContentMarkdownBySlug("updates", lang, resolved.slug);
  const fallback = direct ? null : isFa ? await readContentMarkdownBySlug("updates", "en", resolved.slug) : null;
  const usedEnglishFallback = Boolean(fallback && !direct);
  const resolvedUpdate = direct ?? fallback;

  if (!resolvedUpdate) notFound();

  const rendered = renderMarkdown(resolvedUpdate.raw, { baseSlug: resolvedUpdate.entry.slug });

  return (
    <div className="grid gap-10">
      <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">
        <Link href={`${resolvedBase}/updates`} prefetch={false} className="hover:text-foreground transition-colors">
          {isFa ? "به‌روزرسانی‌ها" : "Updates"}
        </Link>
        <span>{" / "}</span>
        <span className="font-mono">{resolved.slug.join("/")}</span>
      </div>

      {usedEnglishFallback ? (
        <Callout title="نسخه فارسی هنوز آماده نیست" tone="warning">
          این به‌روزرسانی هنوز به فارسی ترجمه نشده است. فعلاً نسخه انگلیسی نمایش داده می‌شود.
        </Callout>
      ) : null}

      <PageHeader title={resolvedUpdate.entry.title} />

      <article className="docs-prose min-w-0" lang={lang === "fa" ? "fa" : undefined} dir={lang === "fa" ? "rtl" : undefined}>
        {rendered.nodes}
      </article>
    </div>
  );
}

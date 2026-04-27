import Link from "next/link";
import { notFound } from "next/navigation";
import { Callout } from "../../../../components/ui/Callout";
import { PageHeader } from "../../../../components/ui/PageHeader";
import { getContentIndex, readContentMarkdownBySlug } from "../../../../lib/content/fs";
import { renderMarkdown } from "../../../../lib/docs/markdown";

export const dynamic = "force-static";

export async function generateStaticParams() {
  const en = await getContentIndex("blog", "en");
  const fa = await getContentIndex("blog", "fa");
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

  const direct = await readContentMarkdownBySlug("blog", lang, resolved.slug);
  const fallback = direct ? null : isFa ? await readContentMarkdownBySlug("blog", "en", resolved.slug) : null;
  const usedEnglishFallback = Boolean(fallback && !direct);
  const resolvedPost = direct ?? fallback;

  if (!resolvedPost) notFound();

  const rendered = renderMarkdown(resolvedPost.raw, { baseSlug: resolvedPost.entry.slug });
  const renderLang = resolvedPost.entry.lang;

  return (
    <div className="grid gap-10">
      <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">
        <Link href={`${resolvedBase}/blog`} prefetch={false} className="hover:text-foreground transition-colors">
          {isFa ? "بلاگ" : "Blog"}
        </Link>
        <span>{" / "}</span>
        <span className="font-mono">{resolved.slug.join("/")}</span>
      </div>

      {usedEnglishFallback ? (
        <Callout title="نسخه فارسی هنوز آماده نیست" tone="warning">
          این پست هنوز به فارسی ترجمه نشده است. فعلاً نسخه انگلیسی نمایش داده می‌شود.
        </Callout>
      ) : null}

      <PageHeader title={resolvedPost.entry.title} />

      <article
        className="docs-prose min-w-0"
        lang={renderLang === "fa" ? "fa" : undefined}
        dir={renderLang === "fa" ? "rtl" : undefined}
      >
        {rendered.nodes}
      </article>
    </div>
  );
}

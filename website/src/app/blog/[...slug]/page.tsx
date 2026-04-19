import Link from "next/link";
import { notFound } from "next/navigation";
import { Callout } from "../../../components/ui/Callout";
import { PageHeader } from "../../../components/ui/PageHeader";
import { getContentIndex, readContentMarkdownBySlug } from "../../../lib/content/fs";
import { renderMarkdown } from "../../../lib/docs/markdown";

export const dynamic = "force-static";

export async function generateStaticParams() {
  const en = await getContentIndex("blog", "en");
  return en.map((e) => ({ slug: e.slug }));
}

export default async function BlogPostPage({
  params,
  basePrefix,
}: {
  params: Promise<{ slug: string[] }> | { slug: string[] };
  basePrefix?: string;
}) {
  const { slug } = await params;
  const resolvedBase = basePrefix?.trim() ? basePrefix : "";
  const isFa = resolvedBase === "/fa";
  const lang = isFa ? "fa" : "en";

  const direct = await readContentMarkdownBySlug("blog", lang, slug);
  const fallback = direct ? null : isFa ? await readContentMarkdownBySlug("blog", "en", slug) : null;
  const usedEnglishFallback = Boolean(fallback && !direct);
  const resolved = direct ?? fallback;

  if (!resolved) notFound();

  const rendered = renderMarkdown(resolved.raw, {
    baseSlug: resolved.entry.slug,
    
  });
  const renderLang = resolved.entry.lang;

  return (
    <div className="grid gap-10">
      <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">
        <Link href={`${resolvedBase}/blog`} prefetch={false} className="hover:text-foreground transition-colors">
          {isFa ? "بلاگ" : "Blog"}
        </Link>
        <span>{" / "}</span>
        <span className="font-mono">{slug.join("/")}</span>
      </div>

      {usedEnglishFallback ? (
        <Callout title="نسخه فارسی هنوز آماده نیست" tone="warning">
          این پست هنوز به فارسی ترجمه نشده است. فعلاً نسخه انگلیسی نمایش داده می‌شود.
        </Callout>
      ) : null}

      <PageHeader title={resolved.entry.title} />

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

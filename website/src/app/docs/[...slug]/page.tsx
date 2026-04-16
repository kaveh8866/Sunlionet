import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { DocToc } from "../../../components/ui/DocToc";
import { PageHeader } from "../../../components/ui/PageHeader";
import { Callout } from "../../../components/ui/Callout";
import { getDocsIndex, readDocMarkdownBySlug } from "../../../lib/docs/fs";
import { renderMarkdown } from "../../../lib/docs/markdown";

export const dynamic = "force-static";

const legacyRedirects: Record<string, string> = {
  "install-linux": "/docs/user/install-linux",
  "install-android": "/docs/user/install-android",
  "install-ios": "/docs/install",
  "install-raspberrypi": "/docs/user/install-linux",
  mobile: "/docs/android/architecture",
  signal: "/docs/signal",
  security: "/docs/user/safety",
  verification: "/docs/outside/verification",
};

function normalizeSlug(slug: string[]) {
  if (slug.length === 0) return ["index"];
  if (slug.length === 1 && slug[0] === "index") return ["index"];
  if (slug.at(-1) === "index") return slug;
  return slug;
}

function tryLegacyRedirect(slug: string[]) {
  if (slug.length !== 1) return null;
  return legacyRedirects[slug[0] ?? ""] ?? null;
}

function tryIndexFallback(slug: string[]) {
  if (slug.length === 0) return null;
  if (slug.at(-1) === "index") return null;
  return [...slug, "index"];
}

export async function generateStaticParams() {
  const entries = await getDocsIndex();
  const params: Array<{ slug: string[] }> = [];

  for (const e of entries) {
    if (e.slug.length === 1 && e.slug[0] === "index") continue;
    params.push({ slug: e.slug });
    if (e.slug.length >= 2 && e.slug.at(-1) === "index") {
      params.push({ slug: e.slug.slice(0, -1) });
    }
  }

  for (const [legacy] of Object.entries(legacyRedirects)) {
    params.push({ slug: [legacy] });
  }

  return params;
}

export default async function DocPage({ params }: { params: Promise<{ slug: string[] }> }) {
  const { slug } = await params;

  const legacy = tryLegacyRedirect(slug);
  if (legacy) redirect(legacy);

  const normalized = normalizeSlug(slug);
  if (normalized.length === 1 && normalized[0] === "index") redirect("/docs");

  const direct = await readDocMarkdownBySlug(normalized);
  const indexFallback = direct ? null : tryIndexFallback(normalized);
  const fallback = indexFallback ? await readDocMarkdownBySlug(indexFallback) : null;
  const resolved = direct ?? fallback;

  if (!resolved) notFound();

  const rendered = renderMarkdown(resolved.raw, { baseSlug: resolved.doc.slug });
  const isFarsi = resolved.doc.slug[0] === "fa";

  const crumbs = [
    { href: "/docs", label: isFarsi ? "مستندات" : "Docs" },
    ...resolved.doc.slug
      .filter((p) => p !== "index")
      .map((p, idx) => {
        const full = resolved.doc.slug.filter((x) => x !== "index").slice(0, idx + 1);
        return { href: `/docs/${full.join("/")}`, label: p };
      }),
  ];

  return (
    <div className="grid gap-8">
      <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">
        {crumbs.map((c, idx) => (
          <span key={c.href}>
            {idx === 0 ? null : " / "}
            <Link href={c.href} prefetch={false} className="hover:text-foreground transition-colors">
              {c.label}
            </Link>
          </span>
        ))}
      </div>

      <PageHeader title={resolved.doc.title} />

      <div className="grid gap-10 xl:grid-cols-[1fr_260px]">
        <article className="docs-prose min-w-0" lang={isFarsi ? "fa" : undefined} dir={isFarsi ? "rtl" : undefined}>
          {rendered.nodes}
        </article>
        <div className="hidden xl:block">
          <div className="sticky top-24 grid gap-4">
            <DocToc items={rendered.toc} title={isFarsi ? "در این صفحه" : undefined} />
            <Callout title={isFarsi ? "ایمنی" : "Safety"} tone="warning">
              {isFarsi
                ? "فرض کنید دستگاه ممکن است ضبط شود. لاگ‌ها را حداقلی نگه دارید، فایل‌ها را قبل از اجرا تأیید کنید، و برای دریافت seed از کانال‌های مورد اعتماد استفاده کنید."
                : "Assume devices may be seized. Prefer minimal logs, verify artifacts, and use trusted channels for seed delivery."}
            </Callout>
          </div>
        </div>
      </div>
    </div>
  );
}

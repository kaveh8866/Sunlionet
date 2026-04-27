import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { DocToc } from "../../../../components/ui/DocToc";
import { PageHeader } from "../../../../components/ui/PageHeader";
import { Callout } from "../../../../components/ui/Callout";
import { getDocsIndex } from "../../../../lib/docs/fs";
import { readDocMarkdownBySlug } from "../../../../lib/docs/fs";
import { renderMarkdown } from "../../../../lib/docs/markdown";

export const dynamic = "force-static";

const legacyRedirects: Record<string, string> = {
  "install-linux": "/docs/user/install-linux",
  "install-android": "/docs/user/install-android",
  "install-ios": "/docs/install",
  "install-raspberrypi": "/docs/user/install-linux",
  mobile: "/docs/android/architecture",
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
  const params: Array<{ lang: string; slug: string[] }> = [];

  for (const e of entries) {
    if (e.slug.length === 1 && e.slug[0] === "index") continue;

    if (e.slug[0] === "fa") {
      const stripped = e.slug.slice(1);
      if (stripped.length === 1 && stripped[0] === "index") continue;
      params.push({ lang: "fa", slug: stripped });
      if (stripped.length >= 2 && stripped.at(-1) === "index") {
        params.push({ lang: "fa", slug: stripped.slice(0, -1) });
      }
      continue;
    }

    params.push({ lang: "en", slug: e.slug });
    if (e.slug.length >= 2 && e.slug.at(-1) === "index") {
      params.push({ lang: "en", slug: e.slug.slice(0, -1) });
    }
  }

  return params;
}

export default async function Page({ params }: { params: Promise<{ lang: string; slug: string[] }> }) {
  const resolved = await params;
  const resolvedBase = `/${resolved.lang}`;
  const prefersFa = resolvedBase === "/fa";

  const requestedSlug =
    resolved.lang === "fa" && resolved.slug[0] !== "fa" ? ["fa", ...resolved.slug] : resolved.slug;

  const legacy = tryLegacyRedirect(requestedSlug);
  if (legacy) redirect(`${resolvedBase}${legacy}`);

  const normalized = normalizeSlug(requestedSlug);
  if (prefersFa && normalized[0] === "fa" && normalized[1] === "fa") {
    redirect(`${resolvedBase}/docs/${normalized.slice(2).join("/")}`);
  }
  if (!prefersFa && normalized[0] === "fa") {
    const rest = normalized.slice(1);
    if (rest.length === 0 || (rest.length === 1 && rest[0] === "index")) redirect(`/fa/docs`);
    if (rest.at(-1) === "index") redirect(`/fa/docs/${rest.slice(0, -1).join("/")}`);
    redirect(`/fa/docs/${rest.join("/")}`);
  }
  if (normalized.length === 1 && normalized[0] === "index") redirect(`${resolvedBase}/docs`);

  const direct = await readDocMarkdownBySlug(normalized);
  const indexFallback = direct ? null : tryIndexFallback(normalized);
  const fallback = indexFallback ? await readDocMarkdownBySlug(indexFallback) : null;
  let resolvedDoc = direct ?? fallback;
  let usedEnglishFallback = false;

  if (!resolvedDoc && prefersFa && normalized[0] === "fa") {
    const enSlug = normalized.slice(1);
    const enDirect = await readDocMarkdownBySlug(enSlug);
    const enIndexFallback = enDirect ? null : tryIndexFallback(enSlug);
    const enFallback = enIndexFallback ? await readDocMarkdownBySlug(enIndexFallback) : null;
    resolvedDoc = enDirect ?? enFallback;
    usedEnglishFallback = Boolean(resolvedDoc);
  }

  if (!resolvedDoc) notFound();

  const renderBaseSlug = resolvedDoc.doc.slug[0] === "fa" ? resolvedDoc.doc.slug.slice(1) : resolvedDoc.doc.slug;
  const rendered = renderMarkdown(resolvedDoc.raw, { baseSlug: renderBaseSlug, basePrefix: resolvedBase });
  const isFarsi = resolvedDoc.doc.slug[0] === "fa";
  const displaySlug =
    prefersFa && normalized[0] === "fa"
      ? normalized.slice(1).filter((p) => p !== "index")
      : resolvedDoc.doc.slug.filter((p) => p !== "index");

  const crumbs: Array<{ href: string | null; label: string }> = [{ href: `${resolvedBase}/docs`, label: prefersFa ? "مستندات" : "Docs" }];
  for (let idx = 0; idx < displaySlug.length; idx += 1) {
    const p = displaySlug[idx];
    if (!p) continue;
    const full = displaySlug.slice(0, idx + 1);
    const directCrumb = await readDocMarkdownBySlug(full);
    const indexFallbackCrumb = directCrumb ? null : await readDocMarkdownBySlug([...full, "index"]);
    const href = directCrumb || indexFallbackCrumb ? `${resolvedBase}/docs/${full.join("/")}` : null;
    crumbs.push({ href, label: p });
  }

  return (
    <div className="grid gap-8">
      <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">
        {crumbs.map((c, idx) => (
          <span key={`${idx}-${c.label}`}>
            {idx === 0 ? null : " / "}
            {c.href ? (
              <Link href={c.href} prefetch={false} className="hover:text-foreground transition-colors">
                {c.label}
              </Link>
            ) : (
              <span>{c.label}</span>
            )}
          </span>
        ))}
      </div>

      <PageHeader title={resolvedDoc.doc.title} />

      <div className="grid gap-10 xl:grid-cols-[1fr_260px]">
        <div className="grid gap-6 min-w-0">
          {usedEnglishFallback ? (
            <Callout title="نسخه فارسی هنوز آماده نیست" tone="warning">
              این صفحه هنوز به فارسی ترجمه نشده است. فعلاً نسخه انگلیسی نمایش داده می‌شود.
            </Callout>
          ) : null}
          <article className="docs-prose min-w-0" lang={isFarsi ? "fa" : undefined} dir={isFarsi ? "rtl" : undefined}>
            {rendered.nodes}
          </article>
        </div>
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

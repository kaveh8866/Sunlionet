import Link from "next/link";
import { InfoCard } from "../../../components/ui/InfoCard";
import { PageHeader } from "../../../components/ui/PageHeader";
import { getDocsIndex } from "../../../lib/docs/fs";

export const dynamic = "force-static";

function docHref(basePrefix: string, slug: string[]) {
  const base = `${basePrefix}/docs`;
  if (slug.length === 1 && slug[0] === "index") return base;
  if (slug.length >= 2 && slug.at(-1) === "index") return `${base}/${slug.slice(0, -1).join("/")}`;
  return `${base}/${slug.join("/")}`;
}

function groupKey(slug: string[]) {
  if (slug.length === 1) return "Root";
  return slug[0] ?? "Root";
}

export default async function DocsAllPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const entries = await getDocsIndex();
  const resolvedBasePrefix = resolved.lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const isFa = resolvedBasePrefix === "/fa";
  const groups = new Map<string, { title: string; items: Array<{ href: string; title: string; slug: string[] }> }>();

  for (const e of entries) {
    if (e.slug.length === 1 && e.slug[0] === "index") continue;
    if (isFa && e.slug[0] !== "fa") continue;
    if (!isFa && e.slug[0] === "fa") continue;

    const displaySlug = isFa ? e.slug.slice(1) : e.slug;
    const key = groupKey(displaySlug);
    const list = groups.get(key) ?? { title: key, items: [] };
    list.items.push({ href: docHref(resolvedBasePrefix, displaySlug), title: e.title, slug: displaySlug });
    groups.set(key, list);
  }

  const ordered = Array.from(groups.values()).sort((a, b) => a.title.localeCompare(b.title));
  for (const g of ordered) {
    g.items.sort((a, b) => a.href.localeCompare(b.href));
  }

  return (
    <div className="grid gap-10">
      <PageHeader
        title={isFa ? "فهرست همه مستندات" : "Browse all docs"}
        subtitle={isFa ? "همه فایل‌های Markdown که زیر /docs/fa قرار دارند." : "All markdown documents found under /docs in the repository."}
        actions={
          <Link
            href={`${resolvedBasePrefix}/docs`}
            prefetch={false}
            className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
          >
            {isFa ? "بازگشت به مستندات" : "Back to docs"}
          </Link>
        }
      />

      <div className="grid gap-10">
        {ordered.map((g) => (
          <section key={g.title} className="grid gap-4">
            <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">{g.title}</div>
            <div className="grid md:grid-cols-2 gap-3">
              {g.items.map((i) => (
                <InfoCard key={i.href} href={i.href} title={i.title} description={<span className="font-mono">{i.slug.join("/")}</span>} />
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}

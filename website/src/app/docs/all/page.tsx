import Link from "next/link";
import { InfoCard } from "../../../components/ui/InfoCard";
import { PageHeader } from "../../../components/ui/PageHeader";
import { getDocsIndex } from "../../../lib/docs/fs";

export const dynamic = "force-static";

function docHref(slug: string[]) {
  if (slug.length === 1 && slug[0] === "index") return "/docs";
  if (slug.length >= 2 && slug.at(-1) === "index") return `/docs/${slug.slice(0, -1).join("/")}`;
  return `/docs/${slug.join("/")}`;
}

function groupKey(slug: string[]) {
  if (slug.length === 1) return "Root";
  return slug[0] ?? "Root";
}

export default async function DocsAllPage() {
  const entries = await getDocsIndex();
  const groups = new Map<string, { title: string; items: Array<{ href: string; title: string; slug: string[] }> }>();

  for (const e of entries) {
    if (e.slug.length === 1 && e.slug[0] === "index") continue;
    const key = groupKey(e.slug);
    const list = groups.get(key) ?? { title: key, items: [] };
    list.items.push({ href: docHref(e.slug), title: e.title, slug: e.slug });
    groups.set(key, list);
  }

  const ordered = Array.from(groups.values()).sort((a, b) => a.title.localeCompare(b.title));
  for (const g of ordered) {
    g.items.sort((a, b) => a.href.localeCompare(b.href));
  }

  return (
    <div className="grid gap-10">
      <PageHeader
        title="Browse all docs"
        subtitle="All markdown documents found under /docs in the repository."
        actions={
          <Link
            href="/docs"
            prefetch={false}
            className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
          >
            Back to docs
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

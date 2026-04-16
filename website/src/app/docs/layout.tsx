import Link from "next/link";
import { getDocsIndex } from "../../lib/docs/fs";
import { cx } from "../../lib/cx";

type NavItem = {
  label: string;
  href: string;
};

function docHref(slug: string[]) {
  if (slug.length === 1 && slug[0] === "index") return "/docs";
  if (slug.length >= 2 && slug.at(-1) === "index") return `/docs/${slug.slice(0, -1).join("/")}`;
  return `/docs/${slug.join("/")}`;
}

function pick(index: Map<string, { title: string; href: string }>, slug: string[]): NavItem | null {
  const key = slug.join("/");
  const found = index.get(key);
  if (!found) return null;
  return { label: found.title, href: found.href };
}

export default async function DocsLayout({ children }: { children: React.ReactNode }) {
  const entries = await getDocsIndex();
  const index = new Map(entries.map((e) => [e.slug.join("/"), { title: e.title, href: docHref(e.slug) }]));

  const overview = [
    pick(index, ["index"]),
    pick(index, ["install"]),
    pick(index, ["architecture"]),
    pick(index, ["core-modules"]),
    pick(index, ["bundle-format"]),
    pick(index, ["signal"]),
    pick(index, ["threat-model"]),
  ].filter(Boolean) as NavItem[];

  const user = [
    pick(index, ["user", "install-android"]),
    pick(index, ["user", "install-linux"]),
    pick(index, ["user", "update"]),
    pick(index, ["user", "safety"]),
  ].filter(Boolean) as NavItem[];

  const outside = [
    pick(index, ["outside", "verification"]),
    pick(index, ["outside", "trust-model"]),
    pick(index, ["outside", "bundle-generation"]),
  ].filter(Boolean) as NavItem[];

  const android = [
    pick(index, ["android", "setup"]),
    pick(index, ["android", "architecture"]),
    pick(index, ["android", "troubleshooting"]),
  ].filter(Boolean) as NavItem[];

  const governance = [
    pick(index, ["governance", "model"]),
    pick(index, ["governance", "trust"]),
    pick(index, ["governance", "resilience"]),
  ].filter(Boolean) as NavItem[];

  const farsi = [
    pick(index, ["fa", "index"]),
    pick(index, ["fa", "install"]),
    pick(index, ["fa", "user", "safety"]),
    pick(index, ["fa", "outside", "verification"]),
  ].filter(Boolean) as NavItem[];

  const sections: Array<{ title: string; items: NavItem[] }> = [
    { title: "Overview", items: overview },
    { title: "User", items: user },
    { title: "Outside", items: outside },
    { title: "Android", items: android },
    { title: "Governance", items: governance },
    { title: "فارسی", items: farsi },
  ].filter((s) => s.items.length > 0);

  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-10">
      <div className="grid gap-10 lg:grid-cols-[260px_1fr]">
        <aside className="hidden lg:block">
          <div className="sticky top-24 grid gap-6">
            <div className="rounded-xl border border-border bg-panel/60 p-4">
              <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">Docs</div>
              <div className="mt-2 text-sm text-muted-foreground leading-relaxed">
                This website renders documentation directly from the repository.
              </div>
            </div>

            <nav className="grid gap-6">
              {sections.map((s) => (
                <div key={s.title} className="grid gap-2" lang={s.title === "فارسی" ? "fa" : undefined} dir={s.title === "فارسی" ? "rtl" : undefined}>
                  <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">{s.title}</div>
                  <div className="grid gap-1">
                    {s.items.map((i) => (
                      <Link
                        key={i.href}
                        href={i.href}
                        prefetch={false}
                        className={cx(
                          "rounded-lg px-3 py-2 text-sm font-semibold text-muted-foreground hover:text-foreground hover:bg-card/60 transition-colors",
                        )}
                      >
                        {i.label}
                      </Link>
                    ))}
                  </div>
                </div>
              ))}

              <Link
                href="/docs/all"
                prefetch={false}
                className="rounded-lg border border-border bg-card/60 px-3 py-2 text-sm font-semibold text-muted-foreground hover:text-foreground hover:bg-card transition-colors"
              >
                Browse all docs
              </Link>
            </nav>
          </div>
        </aside>

        <div className="min-w-0">{children}</div>
      </div>
    </div>
  );
}

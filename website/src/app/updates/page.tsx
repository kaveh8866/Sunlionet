import Link from "next/link";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { getContentIndex } from "../../lib/content/fs";

export const dynamic = "force-static";

export default async function UpdatesIndexPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const lang = resolved.lang === "fa" ? "fa" : "en";
  const basePrefix = lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const isFa = lang === "fa";

  const updates = await getContentIndex("updates", lang);
  const hrefFor = (href: string) => `${basePrefix}${href}`;

  return (
    <div className="grid gap-12">
      <PageHeader
        title={isFa ? "به‌روزرسانی‌ها" : "Updates"}
        subtitle={isFa ? "یادداشت‌های کوتاه و عملیاتی برای هر تغییر و انتشار." : "Short, operational notes for each change and release."}
        actions={
          <Link
            href={hrefFor("/blog")}
            prefetch={false}
            className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
          >
            {isFa ? "بلاگ" : "Blog"}
          </Link>
        }
      />

      <section className="grid gap-6">
        <SectionHeader
          title={isFa ? "آخرین تغییرات" : "Latest changes"}
          subtitle={isFa ? "هر به‌روزرسانی باید هم EN و هم FA داشته باشد." : "Every update must ship in both EN and FA."}
        />
        <div className="grid md:grid-cols-2 gap-4">
          {updates.map((u) => (
            <InfoCard
              key={u.slug.join("/")}
              href={hrefFor(`/updates/${u.slug.join("/")}`)}
              title={u.title}
              description={<span className="font-mono">{u.slug.join("/")}</span>}
            />
          ))}
        </div>
      </section>
    </div>
  );
}


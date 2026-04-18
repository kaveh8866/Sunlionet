import Link from "next/link";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { getContentIndex } from "../../lib/content/fs";

export const dynamic = "force-static";

export default async function BlogIndexPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const lang = resolved.lang === "fa" ? "fa" : "en";
  const basePrefix = lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const isFa = lang === "fa";

  const posts = await getContentIndex("blog", lang);
  const hrefFor = (href: string) => `${basePrefix}${href}`;

  return (
    <div className="grid gap-12">
      <PageHeader
        title={isFa ? "بلاگ" : "Blog"}
        subtitle={isFa ? "یادداشت‌های فنی و توضیحی برای مسیر پروژه." : "Technical and explanatory posts for project direction."}
        actions={
          <Link
            href={hrefFor("/updates")}
            prefetch={false}
            className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
          >
            {isFa ? "به‌روزرسانی‌ها" : "Updates"}
          </Link>
        }
      />

      <section className="grid gap-6">
        <SectionHeader title={isFa ? "آخرین پست‌ها" : "Latest posts"} subtitle={isFa ? "هر پست باید هم EN و هم FA داشته باشد." : "Every post must ship in both EN and FA."} />
        <div className="grid md:grid-cols-2 gap-4">
          {posts.map((p) => (
            <InfoCard
              key={p.slug.join("/")}
              href={hrefFor(`/blog/${p.slug.join("/")}`)}
              title={p.title}
              description={<span className="font-mono">{p.slug.join("/")}</span>}
            />
          ))}
        </div>
      </section>
    </div>
  );
}


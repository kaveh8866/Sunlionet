import Link from "next/link";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { siteCopy } from "../../content/siteCopy";

export const dynamic = "force-static";

const repoUrl = (process.env.NEXT_PUBLIC_REPO_URL ?? "https://github.com/kaveh8866/Sunlionet").replace(/\.git$/, "");

export default async function CommunityPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const lang = resolved.lang === "fa" ? "fa" : "en";
  const basePrefix = resolved.lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const hrefFor = (href: string) => `${basePrefix}${href}`;
  const copy = siteCopy[lang].community;

  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-12">
      <div className="grid gap-10">
        <PageHeader
          eyebrow="SunLionet"
          title={copy.title}
          subtitle={copy.intro}
          actions={
            <>
              <Link
                href={hrefFor("/support")}
                prefetch={false}
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
              >
                {siteCopy[lang].nav.support}
              </Link>
              <a
                href={repoUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
              >
                GitHub
              </a>
            </>
          }
        />

        <section className="grid gap-4 md:grid-cols-2">
          <InfoCard
            title={lang === "fa" ? "مشارکت کد" : "Contribute code"}
            description={
              lang === "fa"
                ? "پروژه متن‌باز است و PRهای کوچک و قابل بازبینی بیشترین کمک را می‌کنند."
                : "The project is open-source. Small, reviewable pull requests help the most."
            }
            href={repoUrl}
          />
          <InfoCard
            title={lang === "fa" ? "اجرای Outside و انتقال بسته‌ها" : "Run Outside and deliver bundles"}
            description={
              lang === "fa"
                ? "اگر در شبکه‌ای امن‌تر هستید، می‌توانید Outside را اجرا کنید و خروجی را از کانال‌های قابل اعتماد منتقل کنید."
                : "If you are on a more stable network, you can run Outside and deliver artifacts through trusted channels."
            }
          />
          <InfoCard
            title={lang === "fa" ? "گزارش تجربه‌های میدانی" : "Share field observations"}
            description={
              lang === "fa"
                ? "گزارش‌های ناشناس دربارهٔ الگوهای جدید مسدودسازی کمک می‌کند تصمیم‌های بهتر گرفته شود."
                : "Anonymous reports about new blocking patterns help improve detection and response."
            }
            href={hrefFor("/docs/user/safety")}
          />
          <InfoCard
            title={lang === "fa" ? "گسترش آگاهی" : "Share the project"}
            description={
              lang === "fa"
                ? "لینک صفحهٔ دانلود یا نصب را با افراد مورد اعتماد به اشتراک بگذارید."
                : "Share the download or installation page with trusted contacts."
            }
            href={hrefFor("/download")}
          />
        </section>
      </div>
    </div>
  );
}

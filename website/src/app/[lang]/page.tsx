import Link from "next/link";
import { DownloadSection } from "../../components/DownloadSection";
import { Callout } from "../../components/ui/Callout";
import { InfoCard } from "../../components/ui/InfoCard";
import { PageHeader } from "../../components/ui/PageHeader";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { siteCopy } from "../../content/siteCopy";
import { getLocalReleases } from "../../lib/releases/local";

export default async function Home({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  const lang = resolved.lang === "fa" ? "fa" : "en";
  const releases = await getLocalReleases();

  const isFa = lang === "fa";
  const base = `/${lang}`;
  const copy = siteCopy[lang];

  return (
    <div dir={isFa ? "rtl" : "ltr"} className={isFa ? "font-fa" : undefined}>
      <div className="mx-auto w-full max-w-6xl px-4 py-12">
        <div className="grid gap-12">
          <PageHeader
            eyebrow="SunLionet"
            title={copy.home.headline}
            subtitle={copy.home.subheadline}
            actions={
              <>
                <Link
                  href={`${base}/download`}
                  prefetch={false}
                  className="bg-primary hover:opacity-90 text-primary-foreground px-6 py-3 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
                >
                  {copy.home.cta.download}
                </Link>
                <Link
                  href={`${base}/docs/outside/verification`}
                  prefetch={false}
                  className="bg-card hover:opacity-90 text-foreground px-6 py-3 rounded-md text-sm font-semibold transition-opacity border border-border"
                >
                  {copy.home.cta.verify}
                </Link>
                <Link
                  href={`${base}/manifest`}
                  prefetch={false}
                  className="bg-card hover:opacity-90 text-muted-foreground px-4 py-3 rounded-md text-sm font-semibold transition-opacity border border-border"
                >
                  {copy.home.cta.readManifest}
                </Link>
              </>
            }
          />

          <section className="grid gap-8">
            <div className="text-center">
              <h2 className="text-2xl font-bold text-foreground">{copy.home.why.title}</h2>
            </div>
            <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-6">
              {copy.home.why.cards.map((card: any, i: number) => (
                <InfoCard
                  key={i}
                  title={card.title}
                  description={card.desc}
                />
              ))}
            </div>
          </section>

          <section className="grid gap-8 bg-card/40 border border-border rounded-2xl p-8 shadow-[0_0_0_1px_var(--border)]">
            <div className="text-center">
              <h2 className="text-2xl font-bold text-foreground">{copy.home.steps.title}</h2>
            </div>
            <div className="grid md:grid-cols-3 gap-8">
              {[copy.home.steps.s1, copy.home.steps.s2, copy.home.steps.s3].map((step: any, i: number) => (
                <div key={i} className="flex flex-col items-center text-center gap-3">
                  <div className="w-10 h-10 rounded-full bg-primary text-primary-foreground flex items-center justify-center font-bold text-lg">
                    {i + 1}
                  </div>
                  <h3 className="font-bold text-foreground">{step.title}</h3>
                  <p className="text-sm text-muted-foreground">{step.desc}</p>
                </div>
              ))}
            </div>
            <div className="flex justify-center mt-4">
              <Link
                href={`${base}/download`}
                className="text-primary font-semibold hover:underline"
              >
                {copy.home.cta.getStarted} →
              </Link>
            </div>
          </section>

          <section className="grid gap-6">
            <SectionHeader title={isFa ? "مستندات" : "Documentation"} subtitle={isFa ? "راهنمای استفاده و جزئیات فنی" : "Guides and technical details"} />
            <div className="grid md:grid-cols-2 gap-4">
              <InfoCard
                href={`${base}/docs`}
                title={isFa ? "راهنمای کاربر" : "User Guides"}
                description={isFa ? "آموزش نصب، استفاده و نکات ایمنی برای کاربران." : "Installation, usage, and safety tips for users."}
              />
              <InfoCard
                href={`${base}/architecture`}
                title={isFa ? "مستندات فنی" : "Technical Docs"}
                description={isFa ? "جزئیات معماری، مدل امنیتی و توسعه‌دهندگان." : "Architecture details, security model, and developer info."}
              />
            </div>
          </section>

          <section className="grid gap-6">
            <SectionHeader title={isFa ? "حمایت و تماس" : "Support & Contact"} subtitle={isFa ? "دریافت کمک و راه‌های ارتباطی" : "Get help and reach out"} />
            <div className="grid md:grid-cols-2 gap-4">
              <InfoCard
                href={`${base}/support`}
                title={isFa ? "حمایت مالی" : "Donate"}
                description={isFa ? "کمک به پایداری پروژه به‌صورت ناشناس." : "Help sustain the project anonymously."}
              />
              <InfoCard
                href="https://github.com/kaveh/sunlionet-agent/issues"
                title={isFa ? "گزارش مشکل" : "Report Issues"}
                description={isFa ? "ثبت مشکلات یا پیشنهادات در گیت‌هاب." : "Submit bugs or suggestions on GitHub."}
              />
            </div>
          </section>

          <section className="grid gap-6">
            <SectionHeader title={isFa ? "مجوز و مسئولیت" : "License & responsibility"} subtitle={isFa ? "قبل از اجرا، قوانین محلی و مدل ریسک شخصی خود را در نظر بگیرید." : "Understand your local laws and your personal risk model before running."} />
            <div className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
              <div className="grid gap-5">
                <Callout title={isFa ? "مجوز" : "License"} tone="info">
                  {isFa ? "SunLionet تحت مجوز AGPL-3.0 منتشر می‌شود." : "SunLionet is licensed under AGPL-3.0."}
                </Callout>
                <Callout title={isFa ? "سلب مسئولیت" : "Disclaimer"}>
                  {isFa
                    ? "مسئولیت نصب و استفاده از این نرم‌افزار با اجراکننده است. با آگاهی از ریسک و مطابق قوانین محلی استفاده کنید."
                    : "The responsibility for installing and operating this software lies with the person who runs it. Use at your own risk and in accordance with your local laws."}
                </Callout>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

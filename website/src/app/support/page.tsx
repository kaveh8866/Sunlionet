import Link from "next/link";
import type { Metadata } from "next";
import { DonationQRCard } from "../../components/DonationQRCard";
import { RevolutReferralCard } from "../../components/RevolutReferralCard";
import { primaryQrValue, supportConfig } from "../../config/support";
import { resolveUILang, uiCopy } from "../../lib/uiCopy";

const repoUrl = (process.env.NEXT_PUBLIC_REPO_URL ?? "https://github.com/kaveh8866/Sunlionet").replace(/\.git$/, "");

export const metadata: Metadata = {
  title: "Support SunLionet – Donate or Earn with Revolut",
  description:
    "Optional ways to support SunLionet: Revolut referral rewards, anonymous crypto donations, and open-source contribution paths.",
};

export default async function SupportPage({ params }: { params: Promise<{ lang?: string }> }) {
  const resolved = await params;
  const lang = resolveUILang(resolved.lang);
  const isFa = lang === "fa";
  const shared = uiCopy[lang].supportPage;
  const resolvedBasePrefix = resolved.lang === "fa" ? "/fa" : resolved.lang === "en" ? "/en" : "";
  const hrefFor = (href: string) => `${resolvedBasePrefix}${href}`;

  return (
    <div className="container mx-auto max-w-6xl px-4 py-14">
      <nav aria-label="Breadcrumb" className="mb-6 text-sm text-muted-foreground">
        <ol className="flex items-center gap-2">
          <li>
            <Link href={hrefFor("/")} className="hover:text-foreground">
              {shared.home}
            </Link>
          </li>
          <li aria-hidden>→</li>
          <li className="text-foreground">{shared.support}</li>
        </ol>
      </nav>

      <section className="rounded-3xl border border-border bg-card/50 p-8 md:p-10 shadow-[0_0_0_1px_var(--border)]">
        <h1 className="text-4xl font-extrabold tracking-tight text-foreground md:text-5xl">
          {isFa ? "حمایت از SunLionet — برای پایداری و حفظ حریم خصوصی" : "Support SunLionet – Keep It Resilient and Private"}
        </h1>
        <p className="mt-4 max-w-3xl text-lg leading-relaxed text-muted">
          {isFa
            ? "هر مشارکت به نگهداری و بهبود سامانه‌ای کمک می‌کند که برای ارتباط امن و پایداری در شبکه‌های محدود طراحی شده است. می‌توانید از مسیرهای امن و اختیاری از پروژه حمایت کنید."
            : "Every contribution helps maintain and improve an open-source system designed for private, resilient communication on restricted networks."}
        </p>
        <p className="mt-4 max-w-3xl text-sm leading-relaxed text-muted-foreground">
          {isFa
            ? "حمایت کاملاً اختیاری است. SunLionet برای همه رایگان و متن‌باز باقی می‌ماند."
            : "Supporting is optional. SunLionet remains fully free and open-source for everyone."}
        </p>
      </section>

      <section className="mt-8 grid gap-6 md:grid-cols-2">
        <RevolutReferralCard
          referralUrl={supportConfig.revolutReferralUrl}
          maxRewardText={supportConfig.revolutMaxReward}
          qrImageSrc={supportConfig.revolutQrImagePath}
          lang={lang}
        />
        <DonationQRCard
          qrValue={primaryQrValue}
          addresses={supportConfig.donationAddresses}
          lang={lang}
        />
      </section>

      <section className="mt-10 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
        <h2 className="text-2xl font-extrabold tracking-tight text-foreground">Revolut Referral (Win-Win)</h2>
        <p className="mt-3 text-sm leading-relaxed text-muted">
          If you create a Revolut account from our referral link and complete the required steps, you may earn up to{" "}
          {supportConfig.revolutMaxReward}. Using our link also supports SunLionet indirectly.
        </p>
        <ol className="mt-4 list-decimal space-y-2 pl-5 text-sm text-muted">
          <li>Sign up with the link and verify your identity.</li>
          <li>Add money to your account.</li>
          <li>Make the required qualifying transactions (usually a few small purchases).</li>
        </ol>
        <p className="mt-4 rounded-lg border border-border bg-card p-4 text-xs leading-relaxed text-muted-foreground">
          Terms and bonus amounts are set by Revolut and may vary. We only provide the referral link as one optional way
          to support the project indirectly.
        </p>
      </section>

      <section className="mt-10 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
        <h2 className="text-2xl font-extrabold tracking-tight text-foreground">
          Direct Support via Cryptocurrency (Fully Anonymous)
        </h2>
        <p className="mt-3 text-sm leading-relaxed text-muted">
          Scan to donate BTC / XMR / USDT or copy the address. One-time or recurring – every satoshi helps keep
          SunLionet updated against new DPI techniques.
        </p>
      </section>

      <section className="mt-10 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
        <h2 className="text-2xl font-extrabold tracking-tight text-foreground">
          {isFa ? "گزینه‌های دیگر حمایت" : "Other Support Options"}
        </h2>
        <ul className="mt-4 space-y-2 text-sm text-muted">
          <li>
            {isFa 
              ? "اجرای نسخه Outside و اشتراک‌گذاری کانفیگ‌های تازه از طریق کانال‌های امن با افراد مورد اعتماد."
              : "Run the Outside version and share fresh configs via secure channels with trusted contacts."}
          </li>
          <li>
            {isFa ? "مشارکت در کدنویسی در " : "Contribute code on "}
            <a
              href={repoUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:opacity-90"
            >
              GitHub (AGPL-3.0)
            </a>
            .
          </li>
          <li>
            {isFa
              ? "معرفی پروژه به دیگران با اشتراک‌گذاری آدرس سایت یا مستندات."
              : "Spread the word by sharing the landing page or documentation."}
          </li>
        </ul>
      </section>

      <section className="mt-10 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
        <h2 className="text-2xl font-extrabold tracking-tight text-foreground">
          {isFa ? "شفافیت و مسئولیت" : "Transparency & Responsibility"}
        </h2>
        <p className="mt-4 text-sm leading-relaxed text-muted">
          {isFa
            ? "سان‌لاین‌نت ۱۰۰٪ رایگان و متن‌باز است. تمامی حمایت‌ها مستقیماً صرف هزینه‌های توسعه و نگهداری سرورهای تست می‌شود."
            : "SunLionet is 100% free and open-source. All donations go directly to development and server costs for testing."}
        </p>
        <p className="mt-4 text-sm leading-relaxed text-muted-foreground">
          {isFa
            ? "مسئولیت استفاده از سان‌لاین‌نت با خود کاربر است. هدف ما فراهم کردن امکان ارتباط امن و آزاد بدون وابستگی به سرویس‌های مرکزی است."
            : "Responsibility for using SunLionet lies with the user. The goal is private, resilient communication without relying on a central service."}
        </p>
      </section>
    </div>
  );
}

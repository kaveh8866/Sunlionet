import Link from "next/link";
import Image from "next/image";
import type { UILang } from "../lib/uiCopy";

type RevolutReferralCardProps = {
  referralUrl: string;
  maxRewardText: string;
  qrImageSrc?: string;
  lang?: UILang;
};

export function RevolutReferralCard({ referralUrl, maxRewardText, qrImageSrc, lang = "en" }: RevolutReferralCardProps) {
  const isFa = lang === "fa";
  return (
    <section
      aria-labelledby="revolut-referral-title"
      className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]"
    >
      <h2 id="revolut-referral-title" className="text-2xl font-extrabold tracking-tight text-foreground">
        {isFa ? `با Revolut تا ${maxRewardText} پاداش بگیرید` : `Earn up to ${maxRewardText} with Revolut`}
      </h2>
      <p className="mt-3 text-muted leading-relaxed">
        {isFa
          ? `با لینک دعوت ما یک حساب رایگان Revolut بسازید. اگر مراحل لازم را کامل کنید، ممکن است تا ${maxRewardText} پاداش بگیرید.`
          : `Create a free Revolut account using our referral link. Complete the required steps and you may earn up to ${maxRewardText}. It’s a win-win: you get the reward, and using our link helps sustain SunLionet.`}
      </p>

      <div className="mt-5">
        <Link
          href={referralUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex w-full items-center justify-center rounded-lg bg-primary px-5 py-3 text-center font-semibold text-primary-foreground transition-opacity hover:opacity-90 shadow-[0_0_0_1px_var(--border)]"
        >
          {isFa ? `ساخت حساب Revolut و دریافت تا ${maxRewardText}` : `Create Revolut Account & Earn up to ${maxRewardText}`}
        </Link>
      </div>

      {qrImageSrc ? (
        <div className="mt-6 rounded-xl border border-border bg-card p-4">
          <p className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">{isFa ? "اسکن QR Revolut" : "Scan Revolut QR"}</p>
          <Link
            href={referralUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="block w-full rounded-lg bg-white p-3"
            aria-label={isFa ? "باز کردن لینک دعوت Revolut با اسکن QR" : "Open Revolut referral link by scanning QR code"}
          >
            <Image
              src={qrImageSrc}
              alt="Revolut referral QR code"
              width={440}
              height={440}
              className="mx-auto h-auto w-full max-w-[280px]"
              priority
              unoptimized
            />
          </Link>
          <p className="mt-3 text-xs text-muted-foreground">
            {isFa ? "نکته: در موبایل دکمه بالا را بزنید. در دسکتاپ این QR را با گوشی اسکن کنید." : "Tip: On mobile, tap the button above. On desktop, scan this QR with your phone."}
          </p>
        </div>
      ) : null}

      <ol className="mt-6 list-decimal space-y-2 pl-5 text-sm text-muted">
        <li>{isFa ? "لینک را باز کنید، ثبت‌نام کنید و هویت خود را تأیید کنید." : "Open the link, sign up, and verify your identity."}</li>
        <li>{isFa ? "حساب را شارژ کنید." : "Add money to your account."}</li>
        <li>{isFa ? "تراکنش‌های لازم را انجام دهید (معمولاً چند خرید کوچک)." : "Make the required qualifying transactions (usually a few small purchases)."}</li>
      </ol>

      <p className="mt-5 rounded-lg border border-border bg-card p-4 text-xs leading-relaxed text-muted-foreground">
        {isFa
          ? "شرایط و مبلغ پاداش توسط Revolut تعیین می‌شود و ممکن است بر اساس کشور یا کمپین متفاوت باشد. این لینک فقط یکی از راه‌های اختیاری برای حمایت غیرمستقیم از پروژه است."
          : "Terms and bonus amounts are set by Revolut and may vary by country, account eligibility, and active campaign. We only provide the referral link as one optional way to support the project indirectly. Using our link helps sustain development of SunLionet."}
      </p>
    </section>
  );
}

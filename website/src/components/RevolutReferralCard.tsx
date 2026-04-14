import Link from "next/link";
import Image from "next/image";

type RevolutReferralCardProps = {
  referralUrl: string;
  maxRewardText: string;
  qrImageSrc?: string;
};

export function RevolutReferralCard({ referralUrl, maxRewardText, qrImageSrc }: RevolutReferralCardProps) {
  return (
    <section
      aria-labelledby="revolut-referral-title"
      className="rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]"
    >
      <h2 id="revolut-referral-title" className="text-2xl font-extrabold tracking-tight text-foreground">
        Earn up to {maxRewardText} with Revolut
      </h2>
      <p className="mt-3 text-muted leading-relaxed">
        Create a free Revolut account using our referral link. Complete the required steps and you may earn up to{" "}
        {maxRewardText}. It’s a win-win: you get the reward, and using our link helps sustain ShadowNet.
      </p>

      <div className="mt-5">
        <Link
          href={referralUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex w-full items-center justify-center rounded-lg bg-primary px-5 py-3 text-center font-semibold text-primary-foreground transition-opacity hover:opacity-90 shadow-[0_0_0_1px_var(--border)]"
        >
          Create Revolut Account &amp; Earn up to {maxRewardText}
        </Link>
      </div>

      {qrImageSrc ? (
        <div className="mt-6 rounded-xl border border-border bg-card p-4">
          <p className="mb-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Scan Revolut QR</p>
          <Link
            href={referralUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="block w-full rounded-lg bg-white p-3"
            aria-label="Open Revolut referral link by scanning QR code"
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
            Tip: On mobile, tap the button above. On desktop, scan this QR with your phone.
          </p>
        </div>
      ) : null}

      <ol className="mt-6 list-decimal space-y-2 pl-5 text-sm text-muted">
        <li>Open the link, sign up, and verify your identity.</li>
        <li>Add money to your account.</li>
        <li>Make the required qualifying transactions (usually a few small purchases).</li>
      </ol>

      <p className="mt-5 rounded-lg border border-border bg-card p-4 text-xs leading-relaxed text-muted-foreground">
        Terms and bonus amounts are set by Revolut and may vary by country, account eligibility, and active campaign.
        We only provide the referral link as one optional way to support the project indirectly. Using our link helps
        sustain development of ShadowNet.
      </p>
    </section>
  );
}

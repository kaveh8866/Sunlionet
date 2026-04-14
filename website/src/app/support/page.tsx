import Link from "next/link";
import type { Metadata } from "next";
import { DonationQRCard } from "../../components/DonationQRCard";
import { RevolutReferralCard } from "../../components/RevolutReferralCard";
import { primaryQrValue, supportConfig } from "../../config/support";

export const metadata: Metadata = {
  title: "Support ShadowNet Agent – Donate or Earn with Revolut",
  description:
    "Optional ways to support ShadowNet Agent: Revolut referral rewards, anonymous crypto donations, and open-source contribution paths.",
};

export default function SupportPage() {
  return (
    <div className="container mx-auto max-w-6xl px-4 py-14">
      <nav aria-label="Breadcrumb" className="mb-6 text-sm text-muted-foreground">
        <ol className="flex items-center gap-2">
          <li>
            <Link href="/" className="hover:text-foreground">
              Home
            </Link>
          </li>
          <li aria-hidden>→</li>
          <li className="text-foreground">Support</li>
        </ol>
      </nav>

      <section className="rounded-3xl border border-border bg-card/50 p-8 md:p-10 shadow-[0_0_0_1px_var(--border)]">
        <h1 className="text-4xl font-extrabold tracking-tight text-foreground md:text-5xl">
          Support ShadowNet – Keep the Light On for Free Internet
        </h1>
        <p className="mt-4 max-w-3xl text-lg leading-relaxed text-muted">
          Every contribution helps maintain and improve the tool that bypasses censorship. You can support us while
          earning rewards yourself.
        </p>
        <p className="mt-4 max-w-3xl text-sm leading-relaxed text-muted-foreground">
          Supporting is optional. ShadowNet remains fully free and open-source for everyone.
        </p>
      </section>

      <section className="mt-8 grid gap-6 md:grid-cols-2">
        <RevolutReferralCard
          referralUrl={supportConfig.revolutReferralUrl}
          maxRewardText={supportConfig.revolutMaxReward}
          qrImageSrc={supportConfig.revolutQrImagePath}
        />
        <DonationQRCard
          qrValue={primaryQrValue}
          addresses={supportConfig.donationAddresses}
        />
      </section>

      <section className="mt-10 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
        <h2 className="text-2xl font-extrabold tracking-tight text-foreground">Revolut Referral (Win-Win)</h2>
        <p className="mt-3 text-sm leading-relaxed text-muted">
          If you create a Revolut account from our referral link and complete the required steps, you may earn up to{" "}
          {supportConfig.revolutMaxReward}. Using our link also supports ShadowNet indirectly.
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
          ShadowNet updated against new DPI techniques.
        </p>
      </section>

      <section className="mt-10 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
        <h2 className="text-2xl font-extrabold tracking-tight text-foreground">Other Support Options</h2>
        <ul className="mt-4 space-y-2 text-sm text-muted">
          <li>Run the Outside version and share fresh configs via Signal with trusted contacts.</li>
          <li>
            Contribute code on{" "}
            <a
              href="https://github.com/kaveh8866/shadownet-agent"
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:opacity-90"
            >
              GitHub (AGPL-3.0)
            </a>
            .
          </li>
          <li>Spread the word by sharing the landing page, docs, or architecture video.</li>
        </ul>
      </section>

      <section className="mt-10 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
        <h2 className="text-2xl font-extrabold tracking-tight text-foreground">Transparency & Responsibility</h2>
        <p className="mt-4 text-sm leading-relaxed text-muted">
          ShadowNet is 100% free and open-source. All donations go directly to development, server costs for testing,
          and maintaining config seeds. No company or organization takes a cut.
        </p>
        <p className="mt-4 text-sm leading-relaxed text-muted-foreground">
          Responsibility for using ShadowNet lies with the user. The goal is bypassing blockers of the free flow of
          information.
        </p>
      </section>
    </div>
  );
}

import Link from "next/link";
import { DownloadSection } from "../components/DownloadSection";

export default function Home() {
  return (
    <div className="flex flex-col items-center justify-center pt-24 px-4">
      <section className="max-w-5xl w-full text-center space-y-8 mb-20 relative">
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[560px] h-[560px] bg-primary/15 blur-[120px] -z-10 rounded-full" />

        <h1 className="text-5xl md:text-7xl font-extrabold tracking-tight text-foreground">
          ShadowNet Agent — Local AI That Beats Censorship
        </h1>
        <p className="text-xl text-muted-foreground max-w-3xl mx-auto leading-relaxed">
          Offline. Intelligent. Unstoppable. Built for freedom of information.
        </p>

        <div className="flex flex-col sm:flex-row gap-4 justify-center items-center pt-4">
          <Link
            href="#downloads"
            className="px-8 py-3 bg-primary hover:opacity-90 text-primary-foreground rounded-lg font-semibold text-lg transition-opacity shadow-[0_0_18px_var(--ring)] w-full sm:w-auto"
          >
            Download Now
          </Link>
          <Link
            href="/architecture"
            prefetch={false}
            className="px-8 py-3 bg-card hover:opacity-90 text-foreground rounded-lg font-semibold text-lg border border-border transition-opacity w-full sm:w-auto"
          >
            Learn How It Works
          </Link>
        </div>

        <div className="mt-10 mx-auto max-w-4xl rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
          <div className="text-xs text-muted-foreground tracking-wider uppercase">Inside vs Outside (Control + Data Plane)</div>
          <div className="mt-4 grid md:grid-cols-2 gap-6">
            <div className="rounded-xl border border-border bg-card p-5 relative overflow-hidden">
              <div className="absolute inset-0 bg-gradient-to-br from-cyan-500/10 to-transparent" />
              <div className="relative">
                <div className="text-foreground font-bold">ShadowNet-Inside</div>
                <div className="text-sm text-muted-foreground mt-2">Detector → Policy Engine → sing-box reload</div>
              </div>
              <div className="relative mt-5 grid gap-2">
                <div className="h-2 rounded bg-cyan-400/30 animate-pulse" />
                <div className="h-2 rounded bg-cyan-400/20 animate-pulse [animation-delay:120ms]" />
                <div className="h-2 rounded bg-cyan-400/10 animate-pulse [animation-delay:240ms]" />
              </div>
            </div>

            <div className="rounded-xl border border-border bg-card p-5 relative overflow-hidden">
              <div className="absolute inset-0 bg-gradient-to-br from-primary/15 to-transparent" />
              <div className="relative">
                <div className="text-foreground font-bold">ShadowNet-Outside</div>
                <div className="text-sm text-muted-foreground mt-2">Discovery → validation → signed+encrypted bundles</div>
              </div>
              <div className="relative mt-5 grid gap-2">
                <div className="h-2 rounded bg-primary/30 animate-pulse" />
                <div className="h-2 rounded bg-primary/20 animate-pulse [animation-delay:120ms]" />
                <div className="h-2 rounded bg-primary/10 animate-pulse [animation-delay:240ms]" />
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="w-full max-w-5xl mb-20 border border-border rounded-xl overflow-hidden bg-card/60 shadow-[0_20px_60px_rgba(0,0,0,0.18)] relative">
        <div className="p-4 border-b border-border bg-card flex gap-2 items-center">
          <div className="w-3 h-3 rounded-full bg-red-500" />
          <div className="w-3 h-3 rounded-full bg-yellow-500" />
          <div className="w-3 h-3 rounded-full bg-green-500" />
          <span className="ml-4 text-xs font-mono text-muted-foreground tracking-wider uppercase">Status Simulator</span>
        </div>
        
        <div className="p-8 grid md:grid-cols-3 gap-8">
          <div className="space-y-4">
            <h3 className="font-mono text-primary font-bold mb-4">detector.Event</h3>
            <div className="p-3 bg-card rounded border border-border text-xs font-mono text-muted animate-pulse">
              [WARN] DNS_POISON_SUSPECTED<br/>
              domain: twitter.com<br/>
              ip: 10.10.34.34
            </div>
            <div className="p-3 bg-card rounded border border-border text-xs font-mono text-muted">
              [CRIT] UDP_BLOCK_DETECTED<br/>
              protocol: hysteria2<br/>
              dropped: true
            </div>
          </div>
          
          <div className="space-y-4">
            <h3 className="font-mono text-cyan-400 font-bold mb-4">policy.Action</h3>
            <div className="p-3 bg-card rounded border border-border text-xs font-mono text-muted">
              Eval: Deterministic<br/>
              Rank: profile_reality_01<br/>
              Action: SWITCH_PROFILE
            </div>
            <div className="p-3 bg-primary/15 rounded border border-border text-xs font-mono text-foreground mt-2 shadow-[0_0_16px_var(--ring)]">
              &gt; Applying config reload
            </div>
          </div>

          <div className="space-y-4">
            <h3 className="font-mono text-emerald-400 font-bold mb-4">sing-box.Reload</h3>
            <div className="p-3 bg-card rounded border border-border text-xs font-mono text-emerald-500/80">
              + outbound: reality_01<br/>
              + transport: tcp<br/>
              + tls: vision<br/>
              Status: Connected (45ms)
            </div>
          </div>
        </div>
      </section>

      <section className="max-w-5xl w-full mb-20">
        <h2 className="text-3xl font-extrabold tracking-tight text-foreground">Project purpose</h2>
        <p className="mt-3 text-muted-foreground leading-relaxed max-w-4xl">
          ShadowNet exists to bypass blockers of the free flow of information. Freedom of information is a human right in
          the age of artificial intelligence.
        </p>
        <p className="mt-3 text-muted-foreground leading-relaxed max-w-4xl">
          Two versions work together: ShadowNet-Inside for users in censored regions, and ShadowNet-Outside for global
          supporters who can provide fresh, tested configurations via signed and encrypted bundles.
        </p>
      </section>

      <section className="max-w-5xl w-full mb-20">
        <div className="flex items-end justify-between gap-6 flex-wrap">
          <div>
            <h2 className="text-3xl font-extrabold tracking-tight text-foreground">Architecture overview</h2>
            <p className="mt-2 text-muted-foreground leading-relaxed max-w-3xl">
              Data Plane: sing-box + TUN/VPN. Control Plane: Detector, deterministic Policy Engine, bounded local advisor
              (Phi-4-mini), and shadownetd supervisor. Coordination: Signal bundles + offline Bluetooth mesh.
            </p>
          </div>
          <Link
            href="/architecture"
            prefetch={false}
            className="bg-card hover:opacity-90 text-foreground px-5 py-3 rounded-lg font-semibold transition-opacity border border-border"
          >
            Read full architecture
          </Link>
        </div>
      </section>

      <section className="max-w-5xl w-full mb-20">
        <h2 className="text-3xl font-extrabold tracking-tight text-foreground">How it works</h2>
        <div className="mt-6 grid md:grid-cols-2 lg:grid-cols-4 gap-4">
          {[
            {
              t: "Real-time DPI detection",
              d: "Stealthy probes + passive telemetry to detect DNS poisoning, SNI resets, injection, and UDP drops.",
            },
            { t: "Automatic protocol rotation", d: "Deterministic ranking switches between Reality, Hysteria2, TUIC, and more." },
            { t: "Local LLM decisions", d: "No cloud calls. The advisor only runs for ambiguous cases with bounded inputs." },
            { t: "Offline resilience", d: "Bluetooth/Wi‑Fi Direct mesh for bundle transfer during partial outages." },
          ].map((c) => (
            <div key={c.t} className="rounded-xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-bold">{c.t}</div>
              <div className="mt-2 text-sm text-muted-foreground leading-relaxed">{c.d}</div>
            </div>
          ))}
        </div>
      </section>

      <div className="max-w-5xl w-full mb-20">
        <DownloadSection />
      </div>

      <section className="max-w-5xl w-full mb-20">
        <h2 className="text-3xl font-extrabold tracking-tight text-foreground">Documentation & guides</h2>
        <p className="mt-2 text-muted-foreground leading-relaxed max-w-3xl">
          Installation in under 10 minutes. Start with the recommended download, then follow the step-by-step guide.
        </p>
        <div className="mt-6 grid md:grid-cols-3 gap-4">
          {[
            { href: "/docs", t: "/docs", d: "Full project documentation." },
            { href: "/installation", t: "/installation", d: "Step-by-step setup with screenshots placeholders." },
            { href: "/architecture", t: "/architecture", d: "Detailed technical deep-dive." },
          ].map((c) => (
            <Link
              key={c.href}
              href={c.href}
              prefetch={false}
              className="rounded-xl border border-border bg-card/60 p-6 hover:opacity-90 transition-opacity shadow-[0_0_0_1px_var(--border)]"
            >
              <div className="text-foreground font-bold">{c.t}</div>
              <div className="mt-2 text-sm text-muted-foreground leading-relaxed">{c.d}</div>
            </Link>
          ))}
        </div>
      </section>

      <section className="max-w-5xl w-full mb-20">
        <h2 className="text-3xl font-extrabold tracking-tight text-foreground">Get involved & secure cooperation</h2>
        <p className="mt-2 text-muted-foreground leading-relaxed max-w-4xl">
          All cooperation can be done anonymously. No personal data required.
        </p>
        <div className="mt-6 grid md:grid-cols-2 gap-4">
          {[
            { t: "Run Outside + send bundles via Signal", d: "Supporters can run ShadowNet-Outside and deliver seeds through trusted Signal contacts." },
            { t: "Report DPI fingerprint changes", d: "Share anonymized notes about new blocks or resets without sharing personal details." },
            { t: "Contribute code (AGPL-3.0)", d: "Anonymous GitHub contributions are welcome. Keep changes small and reviewable." },
            { t: "Donate or referral support", d: "Use the dedicated support page for anonymous crypto donations and optional referral rewards." },
          ].map((c) => (
            <div key={c.t} className="rounded-xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
              <div className="text-foreground font-bold">{c.t}</div>
              <div className="mt-2 text-sm text-muted-foreground leading-relaxed">{c.d}</div>
            </div>
          ))}
        </div>
        <div className="mt-5">
          <Link
            href="/support"
            className="inline-flex items-center rounded-lg border border-border bg-card px-5 py-3 text-sm font-semibold text-foreground transition-opacity hover:opacity-90"
          >
            Open Support Page
          </Link>
        </div>
      </section>

      <section className="max-w-5xl w-full mb-24">
        <h2 className="text-3xl font-extrabold tracking-tight text-foreground">License & responsibility</h2>
        <div className="mt-6 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
          <div className="text-foreground font-bold mb-2">License</div>
          <p className="text-muted leading-relaxed">
            Licensed under AGPL-3.0 — fully open source and free for everyone.
          </p>
          <div className="mt-6 text-foreground font-bold mb-2">Disclaimer</div>
          <p className="text-muted leading-relaxed">
            The responsibility for using this project lies entirely with the individuals who install and run it. ShadowNet is a tool to bypass blockers of the free flow of information. Use at your own risk and in accordance with your local laws.
          </p>
        </div>
      </section>
    </div>
  );
}

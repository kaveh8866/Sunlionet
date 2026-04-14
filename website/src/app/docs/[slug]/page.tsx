import Link from "next/link";
import { notFound } from "next/navigation";
import type { ReactNode } from "react";

export const dynamic = "force-static";

type Doc = {
  title: string;
  body: ReactNode;
};

const version = "v0.1.0";

const docs: Record<string, Doc> = {
  "install-linux": {
    title: "Install (Linux)",
    body: (
      <>
        <p>
          Download a release artifact, verify SHA256, extract, then install the binary and (optionally) the systemd unit.
        </p>
        <div className="mt-6 grid gap-4">
          <div className="rounded-lg border border-gray-800 bg-gray-950 p-4">
            <div className="text-xs font-mono text-gray-300 whitespace-pre-wrap">
              {`curl -LO /downloads/${version}/shadownet-inside-${version}-linux-amd64.tar.gz
curl -LO /downloads/${version}/shadownet-inside-${version}-linux-amd64.tar.gz.sha256
sha256sum -c shadownet-inside-${version}-linux-amd64.tar.gz.sha256
tar -xzf shadownet-inside-${version}-linux-amd64.tar.gz
sudo ./install-linux.sh inside`}
            </div>
          </div>
          <div className="rounded-lg border border-gray-800 bg-gray-950 p-4">
            <div className="text-xs font-mono text-gray-300 whitespace-pre-wrap">
              {`sudo systemctl enable --now shadownet-inside.service
sudo journalctl -u shadownet-inside -f`}
            </div>
          </div>
        </div>
      </>
    ),
  },
  "install-android": {
    title: "Install (Android / Termux)",
    body: (
      <>
        <p>
          ShadowNet Inside can run as a CLI binary in Termux. A production VPN wrapper (foreground service + VpnService)
          should be a separate Android project.
        </p>
        <div className="mt-6 rounded-lg border border-gray-800 bg-gray-950 p-4">
          <div className="text-xs font-mono text-gray-300 whitespace-pre-wrap">
            {`pkg update -y
pkg install -y wget openssl-tool
wget -O shadownet-inside /downloads/${version}/shadownet-inside-${version}-android-arm64
wget -O shadownet-inside.sha256 /downloads/${version}/shadownet-inside-${version}-android-arm64.sha256
sha256sum -c shadownet-inside.sha256
chmod +x shadownet-inside
./shadownet-inside`}
          </div>
        </div>
      </>
    ),
  },
  "install-ios": {
    title: "Install (iOS / Network Extension)",
    body: (
      <>
        <p>
          On iOS, system-wide tunneling requires a Network Extension (Packet Tunnel Provider). This repository ships the
          core agent as a portable binary; a production iOS wrapper is a separate Xcode project.
        </p>
        <ul className="mt-4 space-y-2 text-sm text-gray-300">
          <li>Use a Packet Tunnel Provider to run sing-box and route traffic.</li>
          <li>Background persistence is improved but still bounded by iOS policy and user settings.</li>
          <li>VPN indicators and Settings entries are controlled by the OS and should be assumed visible.</li>
        </ul>
        <div className="mt-6">
          <Link href="/docs/mobile" className="text-indigo-300 hover:text-indigo-200">
            Read the Mobile Architecture →
          </Link>
        </div>
      </>
    ),
  },
  "install-raspberrypi": {
    title: "Install (Raspberry Pi)",
    body: (
      <>
        <p>
          Use the ARM64 Linux artifact. This is suitable for an always-on home gateway device that your local network can
          route through.
        </p>
        <div className="mt-6 rounded-lg border border-gray-800 bg-gray-950 p-4">
          <div className="text-xs font-mono text-gray-300 whitespace-pre-wrap">
            {`curl -LO /downloads/${version}/shadownet-inside-${version}-linux-arm64.tar.gz
curl -LO /downloads/${version}/shadownet-inside-${version}-linux-arm64.tar.gz.sha256
sha256sum -c shadownet-inside-${version}-linux-arm64.tar.gz.sha256
tar -xzf shadownet-inside-${version}-linux-arm64.tar.gz
sudo ./install-linux.sh inside`}
          </div>
        </div>
      </>
    ),
  },
  mobile: {
    title: "Mobile Architecture",
    body: (
      <>
        <p>
          Mobile wrappers host the ShadowNet core (detector + policy + LLM advisor + secure store) and orchestrate sing-box
          using platform VPN APIs. The wrappers must follow platform rules for user consent, indicators, and background
          execution.
        </p>
        <div className="mt-6 rounded-lg border border-gray-800 bg-gray-950 p-4">
          <div className="text-xs font-mono text-gray-300 whitespace-pre-wrap">
            {`Android (VpnService / Foreground Service)
┌───────────────────────────────┐
│ UI (Activity)                 │
│ - user consent + controls     │
│ - status + diagnostics        │
└───────────────┬───────────────┘
                │ Binder/IPC
┌───────────────▼───────────────┐
│ Foreground VpnService         │
│ - owns TUN fd                 │
│ - starts/stops sing-box       │
│ - schedules periodic work     │
└───────────────┬───────────────┘
                │ local exec / lib
┌───────────────▼───────────────┐
│ shadownet-inside core         │
│ - detector / policy / llm     │
│ - secure store                │
└───────────────────────────────┘

iOS (Network Extension)
┌───────────────────────────────┐
│ App UI                        │
│ - user consent + onboarding   │
│ - config import (Signal URI)  │
└───────────────┬───────────────┘
                │ shared app group
┌───────────────▼───────────────┐
│ PacketTunnelProvider          │
│ - starts sing-box core        │
│ - applies network settings    │
│ - periodic reachability       │
└───────────────┬───────────────┘
                │ local
┌───────────────▼───────────────┐
│ shadownet-inside core         │
│ - detector / policy / llm     │
│ - secure store                │
└───────────────────────────────┘`}
          </div>
        </div>
        <ul className="mt-6 space-y-2 text-sm text-gray-300">
          <li>Assume OS indicators are visible (VPN key / status tiles / Settings entries).</li>
          <li>Prioritize low CPU/RAM to avoid battery/thermal suspicion.</li>
          <li>Prefer bounded logs and explicit “clear local data” controls.</li>
        </ul>
      </>
    ),
  },
  signal: {
    title: "Signal Bundles (Outside → Inside)",
    body: (
      <>
        <p>
          Outside curates profiles and produces a signed + age-encrypted bundle URI: <span className="font-mono">snb://v2:</span>
          …
        </p>
        <ul className="mt-4 space-y-2 text-sm text-gray-300">
          <li>Inside accepts bundles only from explicitly trusted publisher keys.</li>
          <li>All failures are fail-closed: reject unknown versions, expired bundles, invalid signatures, decryption failures.</li>
          <li>Inside stores only what it needs: profiles + minimal state, bounded event buffer, no domains.</li>
        </ul>
        <div className="mt-6">
          <Link href="/docs/security" className="text-indigo-300 hover:text-indigo-200">
            Read the Security Model →
          </Link>
        </div>
      </>
    ),
  },
  security: {
    title: "Security Model",
    body: (
      <>
        <p>
          ShadowNet is designed to minimize local forensic value and limit network-identifiable fingerprints. The control plane
          is local-first; the website never hosts live proxy seeds.
        </p>
        <ul className="mt-4 space-y-2 text-sm text-gray-300">
          <li>Encrypted local store: profiles + health state encrypted at rest.</li>
          <li>Bounded event buffer: last 50 events only; no visited domains.</li>
          <li>Wipe-on-suspicion: secure erase + key zeroization when panic switch is triggered.</li>
          <li>LLM advisor is bounded: deterministic engine handles routine cases, LLM invoked only for ambiguous cases.</li>
          <li>Mobile constraints: VPN indicators and Settings visibility are controlled by Android/iOS and should be assumed visible.</li>
        </ul>
      </>
    ),
  },
  verification: {
    title: "Verify Downloads",
    body: (
      <>
        <p>Each artifact includes a SHA256 file. Always verify before running.</p>
        <div className="mt-6 rounded-lg border border-gray-800 bg-gray-950 p-4">
          <div className="text-xs font-mono text-gray-300 whitespace-pre-wrap">
            {`sha256sum -c shadownet-inside-${version}-linux-amd64.tar.gz.sha256
sha256sum -c shadownet-outside-${version}-windows-amd64.zip.sha256`}
          </div>
        </div>
      </>
    ),
  },
};

export function generateStaticParams() {
  return Object.keys(docs).map((slug) => ({ slug }));
}

export default function DocPage({ params }: { params: { slug: string } }) {
  const doc = docs[params.slug];
  if (!doc) {
    notFound();
  }

  return (
    <div className="container mx-auto px-4 py-16 max-w-3xl">
      <Link href="/docs" className="text-sm text-gray-400 hover:text-gray-200">
        ← Back to Docs
      </Link>
      <h1 className="mt-4 text-4xl font-extrabold tracking-tight text-white">{doc.title}</h1>
      <article className="docs-prose mt-8">{doc.body}</article>
    </div>
  );
}

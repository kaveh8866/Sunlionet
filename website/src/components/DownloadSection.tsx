"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { type DetectedOS, useOsDetection } from "./useOsDetection";

type Role = "inside" | "outside";

type DownloadItem = {
  id: string;
  title: string;
  description: string;
  file?: string;
  sha256Url?: string;
  installCommand?: string;
  verifyCommand?: string;
  notes?: string[];
};

type PlatformCard = {
  key: "linux" | "android" | "raspberrypi" | "windows" | "macos" | "other";
  title: string;
  items: DownloadItem[];
};

const version = "v0.1.0";
const base = `/downloads/${version}`;

function fileUrl(file: string) {
  return `${base}/${file}`;
}

function shaUrl(file: string) {
  return `${base}/${file}.sha256`;
}

function recommendedFor(os: DetectedOS, role: Role): DownloadItem {
  if (os === "android") {
    return {
      id: "android-termux-arm64",
      title: "Android (Termux) — arm64",
      description: "Run ShadowNet-Inside as a CLI binary in Termux (development).",
      file: `shadownet-inside-${version}-android-arm64`,
      sha256Url: shaUrl(`shadownet-inside-${version}-android-arm64`),
      installCommand: `pkg update -y && pkg install -y wget openssl-tool && wget -O shadownet-inside ${fileUrl(
        `shadownet-inside-${version}-android-arm64`,
      )} && wget -O shadownet-inside.sha256 ${shaUrl(`shadownet-inside-${version}-android-arm64`)} && sha256sum -c shadownet-inside.sha256 && chmod +x shadownet-inside && ./shadownet-inside`,
      verifyCommand: `sha256sum -c shadownet-inside.sha256`,
      notes: [
        "No Play Store distribution. Sideloading/VPN wrapper is a separate Android project.",
        "Use Signal bundles for seeds (website never hosts live proxy seeds).",
      ],
    };
  }

  if (os === "windows") {
    const file = role === "inside" ? `shadownet-inside-${version}-windows-amd64.zip` : `shadownet-outside-${version}-windows-amd64.zip`;
    return {
      id: `windows-${role}`,
      title: `Windows — ${role === "inside" ? "Inside" : "Outside"} (amd64)`,
      description: role === "inside" ? "Inside for restricted networks." : "Outside for supporters to curate and send bundles.",
      file,
      sha256Url: shaUrl(file),
      installCommand: `curl -LO ${fileUrl(file)} && curl -LO ${shaUrl(file)} && certutil -hashfile ${file} SHA256`,
      verifyCommand: `Compare with ${file}.sha256`,
      notes: ["Extract zip, run the binary. For Inside, provide SHADOWNET_MASTER_KEY env var."],
    };
  }

  if (os === "macos") {
    const file = role === "inside" ? `shadownet-inside-${version}-darwin-arm64.tar.gz` : `shadownet-outside-${version}-darwin-arm64.tar.gz`;
    return {
      id: `macos-${role}`,
      title: `macOS — ${role === "inside" ? "Inside" : "Outside"} (arm64)`,
      description: "Apple Silicon build.",
      file,
      sha256Url: shaUrl(file),
      installCommand: `curl -LO ${fileUrl(file)} && curl -LO ${shaUrl(file)} && shasum -a 256 -c ${file}.sha256 && tar -xzf ${file}`,
      verifyCommand: `shasum -a 256 -c ${file}.sha256`,
      notes: ["For Outside, use this machine to generate and send Signal bundles."],
    };
  }

  if (os === "raspberrypi") {
    const file = `shadownet-inside-${version}-linux-arm64.tar.gz`;
    return {
      id: "pi-inside-arm64",
      title: "Raspberry Pi — Inside (ARM64)",
      description: "Optimized ARM64 build suitable for an always-on gateway.",
      file,
      sha256Url: shaUrl(file),
      installCommand: `curl -LO ${fileUrl(file)} && curl -LO ${shaUrl(file)} && sha256sum -c ${file}.sha256 && tar -xzf ${file} && sudo ./install-linux.sh inside`,
      verifyCommand: `sha256sum -c ${file}.sha256`,
      notes: ["Install service file included in the tarball.", "Use ethernet when possible for stability."],
    };
  }

  const linuxFile = role === "inside" ? `shadownet-inside-${version}-linux-amd64.tar.gz` : `shadownet-outside-${version}-linux-amd64.tar.gz`;
  return {
    id: `linux-${role}`,
    title: `Linux — ${role === "inside" ? "Inside" : "Outside"} (amd64)`,
    description: "Static binary bundle with install script and service file.",
    file: linuxFile,
    sha256Url: shaUrl(linuxFile),
    installCommand: `curl -LO ${fileUrl(linuxFile)} && curl -LO ${shaUrl(linuxFile)} && sha256sum -c ${linuxFile}.sha256 && tar -xzf ${linuxFile} && sudo ./install-linux.sh ${role}`,
    verifyCommand: `sha256sum -c ${linuxFile}.sha256`,
    notes: ["For Inside, run on the censored side. For Outside, run on stable internet and send bundles."],
  };
}

async function fetchSha256(shaFileUrl: string): Promise<string | null> {
  try {
    const r = await fetch(shaFileUrl, { cache: "no-store" });
    if (!r.ok) return null;
    const t = await r.text();
    const first = t.trim().split(/\s+/)[0];
    if (!first || first.length < 32) return null;
    return first;
  } catch {
    return null;
  }
}

function CodeBlock({ children }: { children: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-[0_0_0_1px_var(--border)]">
      <pre className="text-xs font-mono text-muted whitespace-pre-wrap break-words">{children}</pre>
    </div>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={[
        "px-4 py-2 text-sm font-semibold rounded-md border transition-colors",
        active
          ? "bg-primary border-border text-primary-foreground shadow-[0_0_16px_var(--ring)]"
          : "bg-card border-border text-muted hover:text-foreground hover:opacity-90",
      ].join(" ")}
    >
      {children}
    </button>
  );
}

export function DownloadSection() {
  const { detection, supportsAutoRecommendation } = useOsDetection();
  const [role, setRole] = useState<Role>("inside");
  const [manualOS, setManualOS] = useState<DetectedOS>("unknown");
  const [sha, setSha] = useState<string | null>(null);

  const effectiveOS: DetectedOS = supportsAutoRecommendation ? detection.os : manualOS;
  const recommended = useMemo(() => recommendedFor(effectiveOS, role), [effectiveOS, role]);

  useEffect(() => {
    let cancelled = false;
    setSha(null);
    if (!recommended.sha256Url) return;
    void fetchSha256(recommended.sha256Url).then((v) => {
      if (!cancelled) setSha(v);
    });
    return () => {
      cancelled = true;
    };
  }, [recommended.sha256Url]);

  const cards: PlatformCard[] = useMemo(
    () => [
      {
        key: "linux",
        title: "Linux",
        items: [
          {
            id: "linux-amd64-inside",
            title: "Inside (x86_64 static bundle)",
            description: "Tarball with install script + systemd unit.",
            file: `shadownet-inside-${version}-linux-amd64.tar.gz`,
            sha256Url: shaUrl(`shadownet-inside-${version}-linux-amd64.tar.gz`),
            installCommand: `curl -LO ${fileUrl(`shadownet-inside-${version}-linux-amd64.tar.gz`)} && curl -LO ${shaUrl(
              `shadownet-inside-${version}-linux-amd64.tar.gz`,
            )} && sha256sum -c shadownet-inside-${version}-linux-amd64.tar.gz.sha256 && tar -xzf shadownet-inside-${version}-linux-amd64.tar.gz && sudo ./install-linux.sh inside`,
            verifyCommand: `sha256sum -c shadownet-inside-${version}-linux-amd64.tar.gz.sha256`,
            notes: ["Includes: install-linux.sh, shadownet-inside.service"],
          },
          {
            id: "linux-arm64-inside",
            title: "Inside (arm64 static bundle)",
            description: "ARM64 tarball for servers and single-board devices.",
            file: `shadownet-inside-${version}-linux-arm64.tar.gz`,
            sha256Url: shaUrl(`shadownet-inside-${version}-linux-arm64.tar.gz`),
            installCommand: `curl -LO ${fileUrl(`shadownet-inside-${version}-linux-arm64.tar.gz`)} && curl -LO ${shaUrl(
              `shadownet-inside-${version}-linux-arm64.tar.gz`,
            )} && sha256sum -c shadownet-inside-${version}-linux-arm64.tar.gz.sha256 && tar -xzf shadownet-inside-${version}-linux-arm64.tar.gz && sudo ./install-linux.sh inside`,
          },
          {
            id: "linux-deb",
            title: ".deb package",
            description: "Placeholder (will be added via CI release automation).",
            notes: ["Planned filename: shadownet-inside_${version}_amd64.deb", "Planned: apt install ./file.deb"],
          },
          {
            id: "linux-rpm",
            title: ".rpm package",
            description: "Placeholder (will be added via CI release automation).",
            notes: ["Planned filename: shadownet-inside-${version}-1.x86_64.rpm", "Planned: sudo rpm -i ./file.rpm"],
          },
        ],
      },
      {
        key: "android",
        title: "Android",
        items: [
          {
            id: "android-apk-universal",
            title: "Universal APK",
            description: "Placeholder (VPN wrapper app is a separate Android project).",
            notes: ["Planned filename: shadownet-inside-${version}-universal.apk", "Sideload only."],
          },
          {
            id: "android-apk-arm64",
            title: "APK (arm64-v8a)",
            description: "Placeholder (VPN wrapper app is a separate Android project).",
            notes: ["Planned filename: shadownet-inside-${version}-arm64-v8a.apk"],
          },
          {
            id: "android-termux",
            title: "Termux binary (arm64)",
            description: "Ready now (CLI).",
            file: `shadownet-inside-${version}-android-arm64`,
            sha256Url: shaUrl(`shadownet-inside-${version}-android-arm64`),
            installCommand: `pkg update -y && pkg install -y wget openssl-tool && wget -O shadownet-inside ${fileUrl(
              `shadownet-inside-${version}-android-arm64`,
            )} && wget -O shadownet-inside.sha256 ${shaUrl(`shadownet-inside-${version}-android-arm64`)} && sha256sum -c shadownet-inside.sha256 && chmod +x shadownet-inside && ./shadownet-inside`,
          },
        ],
      },
      {
        key: "raspberrypi",
        title: "Raspberry Pi",
        items: [
          {
            id: "pi-arm64",
            title: "ARM64 optimized bundle",
            description: "Use the Linux ARM64 Inside tarball.",
            file: `shadownet-inside-${version}-linux-arm64.tar.gz`,
            sha256Url: shaUrl(`shadownet-inside-${version}-linux-arm64.tar.gz`),
            installCommand: `curl -LO ${fileUrl(`shadownet-inside-${version}-linux-arm64.tar.gz`)} && curl -LO ${shaUrl(
              `shadownet-inside-${version}-linux-arm64.tar.gz`,
            )} && sha256sum -c shadownet-inside-${version}-linux-arm64.tar.gz.sha256 && tar -xzf shadownet-inside-${version}-linux-arm64.tar.gz && sudo ./install-linux.sh inside`,
          },
        ],
      },
      {
        key: "windows",
        title: "Windows",
        items: [
          {
            id: "win-inside",
            title: "Inside (amd64)",
            description: "Zip bundle for Windows.",
            file: `shadownet-inside-${version}-windows-amd64.zip`,
            sha256Url: shaUrl(`shadownet-inside-${version}-windows-amd64.zip`),
            installCommand: `curl -LO ${fileUrl(`shadownet-inside-${version}-windows-amd64.zip`)} && curl -LO ${shaUrl(
              `shadownet-inside-${version}-windows-amd64.zip`,
            )}`,
            notes: ["Verify SHA256 using certutil and compare to the .sha256 file."],
          },
          {
            id: "win-outside",
            title: "Outside (amd64)",
            description: "Zip bundle for Windows supporters.",
            file: `shadownet-outside-${version}-windows-amd64.zip`,
            sha256Url: shaUrl(`shadownet-outside-${version}-windows-amd64.zip`),
            installCommand: `curl -LO ${fileUrl(`shadownet-outside-${version}-windows-amd64.zip`)} && curl -LO ${shaUrl(
              `shadownet-outside-${version}-windows-amd64.zip`,
            )}`,
          },
        ],
      },
      {
        key: "macos",
        title: "macOS",
        items: [
          {
            id: "mac-inside",
            title: "Inside (arm64)",
            description: "Apple Silicon tarball.",
            file: `shadownet-inside-${version}-darwin-arm64.tar.gz`,
            sha256Url: shaUrl(`shadownet-inside-${version}-darwin-arm64.tar.gz`),
            installCommand: `curl -LO ${fileUrl(`shadownet-inside-${version}-darwin-arm64.tar.gz`)} && curl -LO ${shaUrl(
              `shadownet-inside-${version}-darwin-arm64.tar.gz`,
            )} && shasum -a 256 -c shadownet-inside-${version}-darwin-arm64.tar.gz.sha256`,
          },
          {
            id: "mac-outside",
            title: "Outside (arm64)",
            description: "Apple Silicon tarball.",
            file: `shadownet-outside-${version}-darwin-arm64.tar.gz`,
            sha256Url: shaUrl(`shadownet-outside-${version}-darwin-arm64.tar.gz`),
            installCommand: `curl -LO ${fileUrl(`shadownet-outside-${version}-darwin-arm64.tar.gz`)} && curl -LO ${shaUrl(
              `shadownet-outside-${version}-darwin-arm64.tar.gz`,
            )} && shasum -a 256 -c shadownet-outside-${version}-darwin-arm64.tar.gz.sha256`,
          },
        ],
      },
      {
        key: "other",
        title: "Other",
        items: [
          {
            id: "source",
            title: "Source code (tarball)",
            description: "Placeholder (use GitHub Releases or git clone).",
            notes: ["git clone https://github.com/kaveh8866/shadownet-agent.git", "go build -tags inside ./cmd/inside"],
          },
        ],
      },
    ],
    [],
  );

  const [activeTab, setActiveTab] = useState<PlatformCard["key"]>("linux");

  return (
    <section id="downloads" className="w-full">
      <div className="flex items-end justify-between gap-6 flex-wrap">
        <div>
          <h2 className="text-3xl font-extrabold tracking-tight text-foreground">Downloads</h2>
          <p className="mt-2 text-muted-foreground max-w-3xl leading-relaxed">
            Smart recommendation based on your device. Always verify SHA256 before running. The website never hosts live proxy seeds.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <TabButton active={role === "inside"} onClick={() => setRole("inside")}>
            Inside
          </TabButton>
          <TabButton active={role === "outside"} onClick={() => setRole("outside")}>
            Outside
          </TabButton>
        </div>
      </div>

      <div className="mt-8 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
        <div className="flex items-start justify-between gap-6 flex-wrap">
          <div>
            <div className="text-sm text-muted-foreground">Recommended for your system</div>
            <div className="mt-1 text-xl font-bold text-foreground">
              {supportsAutoRecommendation ? detection.label : manualOS === "unknown" ? "Select your OS" : manualOS.toUpperCase()}
            </div>
            <div className="mt-2 text-muted-foreground text-sm max-w-2xl">{recommended.description}</div>
            {recommended.file ? (
              <div className="mt-3 text-xs font-mono text-muted-foreground break-all">File: {recommended.file}</div>
            ) : null}
          </div>

          <div className="flex flex-col gap-3 min-w-[260px]">
            {supportsAutoRecommendation ? null : (
              <select
                value={manualOS}
                onChange={(e) => setManualOS(e.target.value as DetectedOS)}
                className="bg-card border border-border rounded-md px-3 py-2 text-sm text-foreground"
              >
                <option value="unknown">Choose OS…</option>
                <option value="linux">Linux</option>
                <option value="windows">Windows</option>
                <option value="macos">macOS</option>
                <option value="android">Android</option>
                <option value="raspberrypi">Raspberry Pi</option>
                <option value="ios">iOS</option>
              </select>
            )}

            {recommended.file ? (
              <a
                href={fileUrl(recommended.file)}
                className="bg-primary hover:opacity-90 text-primary-foreground px-5 py-3 rounded-lg font-semibold text-center transition-opacity shadow-[0_0_0_1px_var(--border)]"
                download
              >
                Download Now
              </a>
            ) : (
              <Link
                href="/installation"
                className="bg-primary hover:opacity-90 text-primary-foreground px-5 py-3 rounded-lg font-semibold text-center transition-opacity shadow-[0_0_0_1px_var(--border)]"
              >
                Learn Installation
              </Link>
            )}

            {recommended.sha256Url ? (
              <a
                href={recommended.sha256Url}
                className="bg-card hover:opacity-90 text-foreground px-5 py-3 rounded-lg font-semibold text-center transition-opacity border border-border"
                download
              >
                Download SHA256
              </a>
            ) : null}
          </div>
        </div>

        <div className="mt-6 grid md:grid-cols-2 gap-6">
          <div>
            <div className="text-sm font-semibold text-foreground mb-2">Checksum</div>
            <div className="text-xs font-mono text-muted break-all rounded-lg border border-border bg-card p-4 shadow-[0_0_0_1px_var(--border)]">
              {sha ? sha : recommended.sha256Url ? "Loading…" : "TBD"}
            </div>
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground mb-2">Verify</div>
            <CodeBlock>{recommended.verifyCommand ? recommended.verifyCommand : "See /docs/verification"}</CodeBlock>
          </div>
        </div>

        {recommended.installCommand ? (
          <div className="mt-6">
            <div className="text-sm font-semibold text-foreground mb-2">Install (one-liner)</div>
            <CodeBlock>{recommended.installCommand}</CodeBlock>
          </div>
        ) : null}

        {recommended.notes && recommended.notes.length ? (
          <ul className="mt-6 text-sm text-muted space-y-2">
            {recommended.notes.map((n) => (
              <li key={n}>- {n}</li>
            ))}
          </ul>
        ) : null}
      </div>

      <div className="mt-10 flex flex-wrap gap-2">
        {cards.map((c) => (
          <TabButton key={c.key} active={activeTab === c.key} onClick={() => setActiveTab(c.key)}>
            {c.title}
          </TabButton>
        ))}
      </div>

      <div className="mt-6 rounded-2xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
        {cards
          .filter((c) => c.key === activeTab)
          .map((c) => (
            <div key={c.key}>
              <div className="text-foreground font-bold text-lg">{c.title}</div>
              <div className="mt-4 grid md:grid-cols-2 gap-4">
                {c.items.map((i) => (
                  <div key={i.id} className="rounded-xl border border-border bg-card p-5 shadow-[0_0_0_1px_var(--border)]">
                    <div className="text-foreground font-semibold">{i.title}</div>
                    <div className="mt-1 text-sm text-muted-foreground leading-relaxed">{i.description}</div>

                    {i.file ? (
                      <div className="mt-3 flex items-center gap-3 flex-wrap">
                        <a
                          href={fileUrl(i.file)}
                          className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
                          download
                        >
                          Download
                        </a>
                        {i.sha256Url ? (
                          <a
                            href={i.sha256Url}
                            className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                            download
                          >
                            SHA256
                          </a>
                        ) : null}
                      </div>
                    ) : null}

                    {i.installCommand ? (
                      <div className="mt-4">
                        <div className="text-xs font-semibold text-muted mb-2">Install</div>
                        <CodeBlock>{i.installCommand}</CodeBlock>
                      </div>
                    ) : null}

                    {i.notes && i.notes.length ? (
                      <ul className="mt-4 text-sm text-muted space-y-2">
                        {i.notes.map((n) => (
                          <li key={n}>- {n}</li>
                        ))}
                      </ul>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          ))}
      </div>
    </section>
  );
}

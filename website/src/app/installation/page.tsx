export const dynamic = "force-static";

import Link from "next/link";

const repoOwner = "kaveh8866";
const repoName = "shadownet-agent";
const githubRepo = `https://github.com/${repoOwner}/${repoName}`;
const githubDownloads = `${githubRepo}/tree/main/website/public/downloads/v0.1.0`;
const githubDocsInstall = `${githubRepo}/blob/main/docs/install.md`;
const githubRawDownloads = `https://raw.githubusercontent.com/${repoOwner}/${repoName}/main/website/public/downloads/v0.1.0`;

function Step({ n, title, children }: { n: string; title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-2xl border border-gray-800 bg-gray-900/40 p-6">
      <div className="flex items-start gap-4">
        <div className="w-10 h-10 rounded-lg bg-indigo-600/20 border border-indigo-500/40 text-indigo-200 flex items-center justify-center font-bold">
          {n}
        </div>
        <div>
          <div className="text-white font-bold text-lg">{title}</div>
          <div className="mt-2 text-gray-300 leading-relaxed">{children}</div>
        </div>
      </div>
    </div>
  );
}

export default function InstallationPage() {
  return (
    <div className="container mx-auto px-4 py-16 max-w-5xl">
      <h1 className="text-4xl font-extrabold tracking-tight text-white">Installation</h1>
      <p className="mt-4 text-gray-400 leading-relaxed max-w-4xl">
        Installation in under 10 minutes. Choose the recommended download, verify SHA256, then run Inside (for censored
        networks) or Outside (for supporters). The website never hosts live proxy seeds.
      </p>

      <div className="mt-10 grid gap-6">
        <Step n="1" title="Download + verify">
          Go to{" "}
          <Link href="/download" className="text-indigo-300 hover:text-indigo-200">
            /download
          </Link>{" "}
          and download the artifact plus its <span className="font-mono">.sha256</span> file. Always verify before running. You can also
          browse the exact files on GitHub:{" "}
          <a className="text-indigo-300 hover:text-indigo-200" href={githubDownloads} target="_blank" rel="noreferrer">
            downloads/v0.1.0
          </a>
          .
          <div className="mt-4 rounded-lg border border-gray-800 bg-gray-950 p-4">
            <pre className="text-xs font-mono text-gray-300 whitespace-pre-wrap">{`curl -LO ${githubRawDownloads}/shadownet-inside-v0.1.0-linux-amd64.tar.gz
curl -LO ${githubRawDownloads}/shadownet-inside-v0.1.0-linux-amd64.tar.gz.sha256
sha256sum -c shadownet-inside-v0.1.0-linux-amd64.tar.gz.sha256`}</pre>
          </div>
          <div className="mt-4 rounded-xl border border-gray-800 bg-gray-950 p-6 text-gray-500 text-sm">
            Screenshot placeholder: checksum verification on a terminal
          </div>
        </Step>

        <Step n="2" title="Install (Linux bundle)">
          Extract and install:
          <div className="mt-4 rounded-lg border border-gray-800 bg-gray-950 p-4">
            <pre className="text-xs font-mono text-gray-300 whitespace-pre-wrap">{`tar -xzf shadownet-inside-v0.1.0-linux-amd64.tar.gz
sudo ./install-linux.sh inside
sudo systemctl enable --now shadownet-inside.service`}</pre>
          </div>
          <div className="mt-3 text-sm text-gray-400">
            Full install reference (GitHub source):{" "}
            <a className="text-indigo-300 hover:text-indigo-200" href={githubDocsInstall} target="_blank" rel="noreferrer">
              docs/install.md
            </a>
          </div>
          <div className="mt-4 rounded-xl border border-gray-800 bg-gray-950 p-6 text-gray-500 text-sm">
            Screenshot placeholder: systemd service enabled and running
          </div>
        </Step>

        <Step n="3" title="Install (Android / Termux)">
          Install the Termux binary and run it:
          <div className="mt-4 rounded-lg border border-gray-800 bg-gray-950 p-4">
            <pre className="text-xs font-mono text-gray-300 whitespace-pre-wrap">{`pkg update -y
pkg install -y wget openssl-tool
wget -O shadownet-inside ${githubRawDownloads}/shadownet-inside-v0.1.0-android-arm64
wget -O shadownet-inside.sha256 ${githubRawDownloads}/shadownet-inside-v0.1.0-android-arm64.sha256
sha256sum -c shadownet-inside.sha256
chmod +x shadownet-inside
./shadownet-inside`}</pre>
          </div>
          <div className="mt-4 rounded-xl border border-gray-800 bg-gray-950 p-6 text-gray-500 text-sm">
            Screenshot placeholder: Termux running the Inside binary
          </div>
        </Step>

        <Step n="4" title="Install (iOS / Network Extension)">
          iOS requires a dedicated wrapper app using a Packet Tunnel Provider for system-wide tunneling. The core agent in
          this repo is portable, but a production iOS wrapper should be maintained as a separate Xcode project. Assume VPN
          indicators and Settings entries are OS-controlled and visible.
        </Step>

        <Step n="5" title="Seeds via Signal (trusted contact)">
          Ask a trusted supporter to run Outside and send you signed + encrypted bundles via Signal. Inside accepts
          bundles only from explicitly trusted publisher keys. No domains are stored, and only a bounded event buffer is
          retained locally.
        </Step>
      </div>
    </div>
  );
}

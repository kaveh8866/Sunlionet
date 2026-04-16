import Link from "next/link";
import { CodeBlockShell } from "../../components/ui/CodeBlockShell";
import { PageHeader } from "../../components/ui/PageHeader";
import { getLocalReleases } from "../../lib/releases/local";

export const dynamic = "force-static";

const repoOwner = "kaveh8866";
const repoName = "shadownet-agent";
const githubRepo = `https://github.com/${repoOwner}/${repoName}`;
const githubDocsInstall = `${githubRepo}/blob/main/docs/install.md`;

function Step({ n, title, children }: { n: string; title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-border bg-card/60 p-6 shadow-[0_0_0_1px_var(--border)]">
      <div className="flex items-start gap-4">
        <div className="w-10 h-10 rounded-lg bg-primary/15 border border-border text-foreground flex items-center justify-center font-bold">
          {n}
        </div>
        <div>
          <div className="text-foreground font-bold text-lg">{title}</div>
          <div className="mt-2 text-muted-foreground leading-relaxed">{children}</div>
        </div>
      </div>
    </div>
  );
}

export default async function InstallationPage() {
  const releases = await getLocalReleases();
  const tag = releases[0]?.tag ?? "v0.1.0";
  const githubDownloads = `${githubRepo}/tree/main/website/public/downloads/${tag}`;

  const linuxAmd64 = `shadownet-inside-${tag}-linux-amd64.tar.gz`;
  const androidArm64 = `shadownet-inside-${tag}-android-arm64`;

  return (
    <div className="mx-auto w-full max-w-6xl px-4 py-12">
      <div className="grid gap-10">
        <PageHeader
          title="Installation"
          subtitle="Common install paths for ShadowNet-Inside and ShadowNet-Outside. Always verify checksums before running."
          actions={
            <>
              <Link
                href="/download"
                prefetch={false}
                className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
              >
                Download
              </Link>
              <a
                className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
                href={githubDocsInstall}
                target="_blank"
                rel="noreferrer"
              >
                Repo install doc
              </a>
            </>
          }
        />

        <div className="grid gap-6">
          <Step n="1" title="Download + verify">
            Go to{" "}
            <Link href="/download" prefetch={false} className="text-primary hover:opacity-90 transition-opacity">
              /download
            </Link>{" "}
            and download the artifact plus its <span className="font-mono">.sha256</span> file. Always verify before running.
            <div className="mt-3 text-sm text-muted-foreground">
              These examples assume you set <span className="font-mono">BASE_URL</span> to the website you are using (example:
              <span className="font-mono"> https://shadownet.example</span>).
            </div>
            <div className="mt-3 text-sm text-muted-foreground">
              You can also browse the exact files on GitHub:{" "}
            <a className="text-primary hover:opacity-90 transition-opacity" href={githubDownloads} target="_blank" rel="noreferrer">
                downloads/{tag}
            </a>
            .
            </div>
            <div className="mt-4">
              <CodeBlockShell
                language="bash"
                code={`BASE_URL="https://shadownet.example"
curl -fL -O "$BASE_URL/downloads/${tag}/${linuxAmd64}"
curl -fL -O "$BASE_URL/downloads/${tag}/${linuxAmd64}.sha256"
sha256sum -c "${linuxAmd64}.sha256"`}
              />
            </div>
          </Step>

          <Step n="2" title="Install (Linux bundle)">
            Extract and install:
            <div className="mt-4">
              <CodeBlockShell
                language="bash"
                code={`tar -xzf ${linuxAmd64}
sudo ./install-linux.sh inside
sudo systemctl enable --now shadownet-inside.service`}
              />
            </div>
            <div className="mt-3 text-sm text-muted-foreground">
              Full install reference (GitHub source):{" "}
              <a className="text-primary hover:opacity-90 transition-opacity" href={githubDocsInstall} target="_blank" rel="noreferrer">
                docs/install.md
              </a>
            </div>
          </Step>

          <Step n="3" title="Install (Android / Termux)">
            Install the Termux binary and run it:
            <div className="mt-4">
              <CodeBlockShell
                language="bash"
                code={`pkg update -y
pkg install -y wget openssl-tool coreutils
wget -O shadownet-inside "$BASE_URL/downloads/${tag}/${androidArm64}"
wget -O shadownet-inside.sha256 "$BASE_URL/downloads/${tag}/${androidArm64}.sha256"
sha256sum -c shadownet-inside.sha256
chmod +x shadownet-inside
./shadownet-inside`}
              />
            </div>
          </Step>

          <Step n="4" title="Install (iOS / Network Extension)">
            iOS requires a dedicated wrapper app using a Packet Tunnel Provider for system-wide tunneling. The core agent in this repo is
            portable, but a production iOS wrapper should be maintained as a separate Xcode project. Assume VPN indicators and Settings entries
            are OS-controlled and visible.
          </Step>

          <Step n="5" title="Seeds via Signal (trusted contact)">
            Ask a trusted supporter to run Outside and send you signed + encrypted bundles via Signal. Inside accepts bundles only from
            explicitly trusted publisher keys. No domains are stored, and only a bounded event buffer is retained locally.
          </Step>
        </div>
      </div>
    </div>
  );
}

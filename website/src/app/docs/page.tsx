import Link from "next/link";

export const dynamic = "force-static";

const docs = [
  { slug: "install-linux", title: "Install (Linux)", desc: "Inside + Outside binaries, systemd, verification." },
  { slug: "install-android", title: "Install (Android)", desc: "Termux install for Inside, verification steps." },
  { slug: "install-ios", title: "Install (iOS)", desc: "Packet Tunnel Provider wrapper, platform constraints." },
  { slug: "install-raspberrypi", title: "Install (Raspberry Pi)", desc: "ARM64 binary, always-on gateway setup." },
  { slug: "mobile", title: "Mobile Architecture", desc: "Android VpnService and iOS Network Extension integration." },
  { slug: "signal", title: "Signal Bundles", desc: "Outside → Inside bundle delivery via snb://v2." },
  { slug: "security", title: "Security Model", desc: "Threat model, local store, wipe-on-suspicion." },
  { slug: "verification", title: "Verify Downloads", desc: "SHA256 checksums and integrity verification." },
];

export default function DocsIndexPage() {
  return (
    <div className="container mx-auto px-4 py-16 max-w-5xl">
      <div className="flex items-end justify-between gap-6 flex-wrap">
        <div>
          <h1 className="text-4xl font-extrabold tracking-tight text-white mb-4">Documentation</h1>
          <p className="text-gray-400 max-w-3xl leading-relaxed">
            No accounts, no analytics. Everything runs locally. If you need seeds, use Signal bundles from a trusted
            helper or in-person transfer. The website never hosts live proxy seeds.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Link
            href="/installation"
            className="bg-gray-900 hover:bg-gray-800 text-gray-100 px-5 py-3 rounded-lg text-sm font-semibold transition-colors border border-gray-800"
          >
            Installation Guide
          </Link>
          <Link
            href="/download"
            className="bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-3 rounded-lg text-sm font-semibold transition-colors"
          >
            Go to Downloads
          </Link>
        </div>
      </div>

      <div className="mt-10 grid md:grid-cols-2 gap-6">
        {docs.map((d) => (
          <Link
            key={d.slug}
            href={`/docs/${d.slug}`}
            className="rounded-xl border border-gray-800 bg-gray-900/40 p-6 hover:border-gray-700 transition-colors"
          >
            <div className="text-white font-bold text-lg">{d.title}</div>
            <div className="text-gray-400 text-sm mt-2 leading-relaxed">{d.desc}</div>
          </Link>
        ))}
      </div>
    </div>
  );
}

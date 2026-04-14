import Link from "next/link";
import { DownloadSection } from "../../components/DownloadSection";

export const dynamic = "force-static";

export default function DownloadPage() {
  return (
    <div className="container mx-auto px-4 py-16 max-w-5xl">
      <h1 className="text-4xl font-extrabold tracking-tight text-white mb-4">Download</h1>
      <p className="text-gray-400 max-w-3xl leading-relaxed">
        Smart OS detection recommends the best download for your device. Verify SHA256 before running. Seed profiles are
        never hosted on the website.
      </p>

      <div className="mt-10">
        <DownloadSection />
      </div>

      <div className="mt-10 rounded-xl border border-gray-800 bg-gray-950 p-6">
        <div className="text-white font-bold mb-2">Need a step-by-step guide?</div>
        <p className="text-gray-400 leading-relaxed max-w-3xl">
          Follow the installation guide for Linux and Android (Termux). For seeded profiles, use Signal bundles from a
          trusted contact.
        </p>
        <div className="mt-4">
          <Link href="/installation" className="text-indigo-300 hover:text-indigo-200 text-sm">
            Installation in under 10 minutes →
          </Link>
        </div>
      </div>
    </div>
  );
}

import Link from "next/link";

export const dynamic = "force-static";

export default function ArchitecturePage() {
  return (
    <div className="container mx-auto px-4 py-16 max-w-5xl">
      <h1 className="text-4xl font-extrabold tracking-tight text-white mb-6">Architecture</h1>
      <p className="text-gray-400 leading-relaxed mb-10 max-w-3xl">
        ShadowNet Agent separates the data plane (sing-box) from the control plane (Detector → Policy Engine → optional
        bounded LLM advisor). Two binaries exist: Inside (runs in restricted networks) and Outside (runs on stable
        networks to curate and deliver seed bundles via Signal and offline transfers).
      </p>

      <div className="grid md:grid-cols-2 gap-8">
        <div className="rounded-xl border border-gray-800 bg-gray-900/40 p-6">
          <h2 className="text-xl font-bold text-white mb-3">Data Plane</h2>
          <ul className="text-gray-300 text-sm space-y-2 leading-relaxed">
            <li>sing-box runs the actual tunnel/outbound.</li>
            <li>Atomic reload applies a new outbound config without losing process state.</li>
            <li>No telemetry or external control channel required.</li>
          </ul>
        </div>
        <div className="rounded-xl border border-gray-800 bg-gray-900/40 p-6">
          <h2 className="text-xl font-bold text-white mb-3">Control Plane</h2>
          <ul className="text-gray-300 text-sm space-y-2 leading-relaxed">
            <li>Detector emits privacy-safe events (no domains, bounded buffer).</li>
            <li>Policy Engine resolves routine cases deterministically (80–90%).</li>
            <li>LLM advisor is invoked only for ambiguous multi-signal cases.</li>
          </ul>
        </div>
      </div>

      <div className="mt-10 rounded-xl border border-gray-800 bg-gray-900/40 p-6">
        <h2 className="text-xl font-bold text-white mb-3">Coordination</h2>
        <ul className="text-gray-300 text-sm space-y-2 leading-relaxed">
          <li>Signal bundles: Outside → Inside signed + encrypted profile delivery.</li>
          <li>Offline mesh: Bluetooth / Wi‑Fi Direct for local transfer during partial outages.</li>
          <li>No central seed API: distribution stays peer-to-peer to reduce blocking risk.</li>
        </ul>
      </div>

      <div className="mt-12 rounded-xl border border-gray-800 bg-gray-950 p-6">
        <h2 className="text-xl font-bold text-white mb-4">Flow</h2>
        <div className="grid md:grid-cols-3 gap-4 text-sm">
          <div className="rounded-lg border border-gray-800 bg-gray-900/40 p-4">
            <div className="font-mono text-indigo-300 mb-2">Detector</div>
            <div className="text-gray-300">Active probes + passive stats → event stream with jitter and budgets.</div>
          </div>
          <div className="rounded-lg border border-gray-800 bg-gray-900/40 p-4">
            <div className="font-mono text-cyan-300 mb-2">Policy Engine</div>
            <div className="text-gray-300">Scores and ranks profiles, applies switch/cooldown rules and diversity.</div>
          </div>
          <div className="rounded-lg border border-gray-800 bg-gray-900/40 p-4">
            <div className="font-mono text-emerald-300 mb-2">Supervisor</div>
            <div className="text-gray-300">Generates config → atomic reload → updates health metrics.</div>
          </div>
        </div>
      </div>

      <div className="mt-10 flex flex-wrap gap-3">
        <Link
          href="/docs"
          className="bg-gray-900 hover:bg-gray-800 text-gray-100 px-5 py-3 rounded-lg font-semibold transition-colors border border-gray-800"
        >
          Read docs
        </Link>
        <Link
          href="/installation"
          className="bg-gray-900 hover:bg-gray-800 text-gray-100 px-5 py-3 rounded-lg font-semibold transition-colors border border-gray-800"
        >
          Installation guide
        </Link>
        <Link
          href="/download"
          className="bg-indigo-600 hover:bg-indigo-500 text-white px-5 py-3 rounded-lg font-semibold transition-colors"
        >
          Downloads
        </Link>
      </div>
    </div>
  );
}

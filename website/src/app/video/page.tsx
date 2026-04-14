export const dynamic = "force-static";

import Link from "next/link";

export default function VideoPage() {
  return (
    <div className="container mx-auto px-4 py-16 max-w-4xl">
      <h1 className="text-4xl font-extrabold tracking-tight text-foreground">Video</h1>
      <p className="mt-4 text-muted-foreground leading-relaxed">
        ShadowNet overview video (local file, no telemetry). If the player doesn’t load, use the direct download link
        below.
      </p>

      <div className="mt-8 rounded-2xl border border-border bg-card/60 p-4 shadow-[0_0_0_1px_var(--border)]">
        <video
          className="w-full rounded-xl bg-black aspect-video"
          controls
          playsInline
          preload="metadata"
          src="/media/ShadowNet.mp4"
        />
        <div className="mt-3 text-sm text-muted-foreground">
          <a className="text-primary hover:opacity-90" href="/media/ShadowNet.mp4" download>
            Download ShadowNet.mp4
          </a>
        </div>
      </div>

      <div className="mt-10">
        <h2 className="text-2xl font-extrabold tracking-tight text-foreground">More about the project</h2>
        <p className="mt-2 text-muted-foreground leading-relaxed">
          Learn how ShadowNet works, how to install it, and where to find the full documentation.
        </p>

        <div className="mt-6 grid sm:grid-cols-2 gap-4">
          {[
            { href: "/docs", t: "Docs", d: "Full documentation and guides." },
            { href: "/architecture", t: "Architecture", d: "Inside vs Outside design and data flow." },
            { href: "/installation", t: "Installation", d: "Step-by-step setup." },
            { href: "/roadmap", t: "Roadmap", d: "Planned features and milestones." },
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

        <div className="mt-6 text-sm text-muted-foreground">
          <a
            className="text-primary hover:opacity-90"
            href="https://github.com/kaveh8866/shadownet-agent"
            target="_blank"
            rel="noreferrer"
          >
            View the GitHub repository
          </a>
        </div>
      </div>
    </div>
  );
}

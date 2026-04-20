import Link from "next/link";

export default function DashboardLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <div className="container mx-auto px-4 py-10">
      <div className="flex items-start justify-between gap-6 flex-wrap">
        <div>
          <h1 className="text-3xl md:text-4xl font-extrabold tracking-tight text-foreground">Dashboard</h1>
          <p className="mt-2 text-muted-foreground max-w-3xl">
            Local runtime monitoring plus optional aggregate operator views (when a live feed is configured).
          </p>
        </div>
        <div className="flex gap-2 flex-wrap">
          <Link
            href="/dashboard/runtime"
            prefetch={false}
            className="px-4 py-2 rounded-lg border border-border bg-card/60 hover:opacity-90 transition-opacity text-sm font-semibold"
          >
            Runtime
          </Link>
          <Link
            href="/dashboard/installation/wizard"
            prefetch={false}
            className="px-4 py-2 rounded-lg border border-border bg-card/60 hover:opacity-90 transition-opacity text-sm font-semibold"
          >
            Install
          </Link>
          <Link
            href="/dashboard/global"
            prefetch={false}
            className="px-4 py-2 rounded-lg border border-border bg-card/60 hover:opacity-90 transition-opacity text-sm font-semibold"
          >
            Global
          </Link>
          <Link
            href="/dashboard/iran"
            prefetch={false}
            className="px-4 py-2 rounded-lg border border-border bg-card/60 hover:opacity-90 transition-opacity text-sm font-semibold"
          >
            Iran Focus
          </Link>
          <Link
            href="/dashboard/protocols"
            prefetch={false}
            className="px-4 py-2 rounded-lg border border-border bg-card/60 hover:opacity-90 transition-opacity text-sm font-semibold"
          >
            Protocols
          </Link>
          <Link
            href="/dashboard/releases"
            prefetch={false}
            className="px-4 py-2 rounded-lg border border-border bg-card/60 hover:opacity-90 transition-opacity text-sm font-semibold"
          >
            Releases
          </Link>
        </div>
      </div>
      <div className="mt-8">{children}</div>
    </div>
  );
}

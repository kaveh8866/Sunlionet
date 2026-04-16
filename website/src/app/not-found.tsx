import Link from "next/link";

export default function NotFound() {
  return (
    <div className="mx-auto w-full max-w-3xl px-4 py-24 text-center">
      <h1 className="text-3xl md:text-4xl font-extrabold tracking-tight text-foreground">Page not found</h1>
      <p className="mt-4 text-muted-foreground leading-relaxed">
        If you followed a download link, visit the Download page to ensure the artifact exists for this release.
      </p>
      <div className="mt-8 flex items-center justify-center gap-3 flex-wrap">
        <Link
          href="/"
          className="bg-card hover:opacity-90 text-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity border border-border"
        >
          Home
        </Link>
        <Link
          href="/download"
          className="bg-primary hover:opacity-90 text-primary-foreground px-4 py-2 rounded-md text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]"
        >
          Download
        </Link>
      </div>
    </div>
  );
}

import Link from "next/link";
import type { ReactNode } from "react";
import { cx } from "../../lib/cx";

type InfoCardProps = {
  title: string;
  description?: ReactNode;
  href?: string;
  eyebrow?: ReactNode;
  tone?: "default" | "panel";
  children?: ReactNode;
};

export function InfoCard({ title, description, href, eyebrow, tone = "default", children }: InfoCardProps) {
  const base =
    "rounded-xl border border-border shadow-[0_0_0_1px_var(--border)] bg-card/60 transition-opacity min-w-0";
  const surface = tone === "panel" ? "bg-panel/70" : "bg-card/60";
  const inner = (
    <div className={cx(base, surface, href ? "hover:opacity-90" : null)}>
      <div className="p-5">
        {eyebrow ? <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">{eyebrow}</div> : null}
        <div className="mt-2 text-foreground font-bold">{title}</div>
        {description ? <div className="mt-2 text-sm text-muted-foreground leading-relaxed">{description}</div> : null}
        {children ? <div className="mt-4">{children}</div> : null}
      </div>
    </div>
  );

  if (href) {
    return (
      <Link href={href} prefetch={false} className="block">
        {inner}
      </Link>
    );
  }

  return inner;
}

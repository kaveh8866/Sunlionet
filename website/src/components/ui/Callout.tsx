import type { ReactNode } from "react";
import { cx } from "../../lib/cx";

type CalloutProps = {
  title?: string;
  children: ReactNode;
  tone?: "default" | "info" | "warning" | "danger" | "success";
};

export function Callout({ title, children, tone = "default" }: CalloutProps) {
  const toneClass =
    tone === "info"
      ? "border-info/30 bg-info/10"
      : tone === "warning"
        ? "border-warning/30 bg-warning/10"
        : tone === "danger"
          ? "border-danger/30 bg-danger/10"
          : tone === "success"
            ? "border-success/30 bg-success/10"
            : "border-border bg-panel/60";

  return (
    <div className={cx("rounded-xl border p-4", toneClass)}>
      {title ? <div className="text-sm font-semibold text-foreground">{title}</div> : null}
      <div className={cx("text-sm leading-relaxed text-muted-foreground", title ? "mt-2" : null)}>{children}</div>
    </div>
  );
}

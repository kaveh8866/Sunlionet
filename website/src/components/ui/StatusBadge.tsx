import { cx } from "../../lib/cx";

type StatusBadgeProps = {
  children: string;
  tone?: "neutral" | "success" | "warning" | "danger" | "info";
};

export function StatusBadge({ children, tone = "neutral" }: StatusBadgeProps) {
  const toneClass =
    tone === "success"
      ? "border-success/30 bg-success/10 text-success"
      : tone === "warning"
        ? "border-warning/30 bg-warning/10 text-warning"
        : tone === "danger"
          ? "border-danger/30 bg-danger/10 text-danger"
          : tone === "info"
            ? "border-info/30 bg-info/10 text-info"
            : "border-border bg-card/60 text-muted-foreground";

  return (
    <span
      className={cx(
        "inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-semibold tracking-wide uppercase",
        toneClass,
      )}
    >
      {children}
    </span>
  );
}

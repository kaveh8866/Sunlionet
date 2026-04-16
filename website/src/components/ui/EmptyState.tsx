import type { ReactNode } from "react";

type EmptyStateProps = {
  title: string;
  description?: ReactNode;
  action?: ReactNode;
};

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className="rounded-xl border border-border bg-panel/60 p-8 text-center">
      <div className="text-foreground font-bold">{title}</div>
      {description ? <div className="mt-2 text-sm text-muted-foreground leading-relaxed">{description}</div> : null}
      {action ? <div className="mt-5 flex justify-center">{action}</div> : null}
    </div>
  );
}

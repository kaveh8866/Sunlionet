import type { ReactNode } from "react";

type PageHeaderProps = {
  title: string;
  subtitle?: ReactNode;
  eyebrow?: ReactNode;
  actions?: ReactNode;
};

export function PageHeader({ title, subtitle, eyebrow, actions }: PageHeaderProps) {
  return (
    <div className="flex items-end justify-between gap-6 flex-wrap">
      <div className="min-w-0">
        {eyebrow ? (
          <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">{eyebrow}</div>
        ) : null}
        <h1 className="mt-2 text-3xl md:text-4xl font-extrabold tracking-tight text-foreground">{title}</h1>
        {subtitle ? <div className="mt-3 text-muted-foreground leading-relaxed max-w-3xl">{subtitle}</div> : null}
      </div>
      {actions ? <div className="flex items-center gap-2 flex-wrap">{actions}</div> : null}
    </div>
  );
}

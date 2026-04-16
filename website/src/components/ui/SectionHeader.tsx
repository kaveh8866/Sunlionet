import type { ReactNode } from "react";

type SectionHeaderProps = {
  title: string;
  subtitle?: ReactNode;
  actions?: ReactNode;
};

export function SectionHeader({ title, subtitle, actions }: SectionHeaderProps) {
  return (
    <div className="flex items-end justify-between gap-6 flex-wrap">
      <div className="min-w-0">
        <h2 className="text-xl md:text-2xl font-bold tracking-tight text-foreground">{title}</h2>
        {subtitle ? <div className="mt-2 text-sm text-muted-foreground leading-relaxed max-w-3xl">{subtitle}</div> : null}
      </div>
      {actions ? <div className="flex items-center gap-2 flex-wrap">{actions}</div> : null}
    </div>
  );
}

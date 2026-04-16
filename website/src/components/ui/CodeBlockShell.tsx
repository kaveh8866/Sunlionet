import { cx } from "../../lib/cx";

type CodeBlockShellProps = {
  code: string;
  language?: string;
  compact?: boolean;
};

export function CodeBlockShell({ code, language, compact = false }: CodeBlockShellProps) {
  return (
    <div className="rounded-xl border border-border bg-code shadow-[0_0_0_1px_var(--border)] overflow-hidden">
      {language ? (
        <div className="flex items-center justify-between gap-3 border-b border-border bg-card/40 px-3 py-2">
          <span className="text-xs font-mono text-muted-foreground uppercase tracking-wide">{language}</span>
        </div>
      ) : null}
      <pre className={cx("overflow-auto", compact ? "p-3" : "p-4")}>
        <code className="text-[0.875rem] leading-relaxed font-mono text-foreground whitespace-pre">{code}</code>
      </pre>
    </div>
  );
}

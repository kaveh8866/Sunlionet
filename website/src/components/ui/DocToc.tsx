import { cx } from "../../lib/cx";

export type DocTocItem = {
  id: string;
  text: string;
  level: number;
};

type DocTocProps = {
  items: DocTocItem[];
  title?: string;
};

export function DocToc({ items, title = "On this page" }: DocTocProps) {
  if (items.length === 0) return null;

  return (
    <div className="rounded-xl border border-border bg-panel/60 p-4">
      <div className="text-xs font-mono tracking-wide text-muted-foreground uppercase">{title}</div>
      <nav className="mt-3 grid gap-2">
        {items.map((i) => (
          <a
            key={i.id}
            href={`#${i.id}`}
            className={cx(
              "text-sm text-muted-foreground hover:text-foreground transition-colors leading-snug",
              i.level >= 3 ? "pl-4" : null,
            )}
          >
            {i.text}
          </a>
        ))}
      </nav>
    </div>
  );
}

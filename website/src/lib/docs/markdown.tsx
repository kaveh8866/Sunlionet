import type { ReactNode } from "react";
import { CodeBlockShell } from "../../components/ui/CodeBlockShell";

const repoUrl = (process.env.NEXT_PUBLIC_REPO_URL ?? "https://github.com/kaveh8866/Sunlionet").replace(/\.git$/i, "");

export type TocItem = {
  id: string;
  text: string;
  level: number;
};

type RenderOptions = {
  baseSlug: string[];
  basePrefix?: string;
  routeBase?: string;
};

type TextAlign = "left" | "center" | "right";

function slugify(text: string) {
  return text
    .normalize("NFKD")
    .toLowerCase()
    .trim()
    .replace(/[`"'()[\]{}]/g, "")
    .replace(/[^\p{L}\p{N}]+/gu, "-")
    .replace(/^-+|-+$/g, "");
}

function splitInline(text: string, baseSlug: string[], basePrefix?: string, routeBase?: string) {
  const parts: ReactNode[] = [];
  const re = /(`[^`]+`)|(\*\*[^*]+\*\*)|(\[[^\]]+\]\([^)]+\))/g;
  let last = 0;
  let m: RegExpExecArray | null;

  while ((m = re.exec(text))) {
    if (m.index > last) parts.push(text.slice(last, m.index));
    const token = m[0];

    if (token.startsWith("`")) {
      parts.push(
        <code key={`${m.index}-code`} className="font-mono">
          {token.slice(1, -1)}
        </code>,
      );
    } else if (token.startsWith("**")) {
      parts.push(
        <strong key={`${m.index}-strong`} className="font-semibold text-foreground">
          {token.slice(2, -2)}
        </strong>,
      );
    } else if (token.startsWith("[")) {
      const lm = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(token);
      if (lm) {
        const href = resolveHref(lm[2], baseSlug, basePrefix, routeBase);
        if (href.toLowerCase().startsWith("file:")) {
          parts.push(
            <span key={`${m.index}-link`} className="text-muted-foreground">
              {lm[1]}
            </span>,
          );
          last = m.index + token.length;
          continue;
        }
        parts.push(
          <a key={`${m.index}-link`} href={href} className="hover:text-foreground transition-colors">
            {lm[1]}
          </a>,
        );
      } else {
        parts.push(token);
      }
    } else {
      parts.push(token);
    }

    last = m.index + token.length;
  }

  if (last < text.length) parts.push(text.slice(last));
  return parts;
}

function resolveHref(href: string, baseSlug: string[], basePrefix?: string, routeBase?: string) {
  const trimmed = href.trim();
  if (trimmed.startsWith("http://") || trimmed.startsWith("https://")) return trimmed;
  if (trimmed.toLowerCase().startsWith("file:")) return trimmed;
  if (trimmed.startsWith("#")) return trimmed;
  if (trimmed.startsWith("/")) return trimmed;

  const withoutHash = trimmed.split("#")[0] ?? trimmed;
  const hash = trimmed.includes("#") ? `#${trimmed.split("#").slice(1).join("#")}` : "";

  if (withoutHash.toLowerCase().endsWith(".md")) {
    const rel = withoutHash.replace(/\\/g, "/");
    const baseDir = baseSlug.slice(0, -1).join("/");
    const joined = `${baseDir ? `${baseDir}/` : ""}${rel}`;
    const normalized = joined
      .split("/")
      .reduce<string[]>((acc, seg) => {
        if (seg === "." || seg === "") return acc;
        if (seg === "..") {
          acc.pop();
          return acc;
        }
        acc.push(seg);
        return acc;
      }, [])
      .join("/");
    const withoutExt = normalized.replace(/\.md$/i, "");
    const prefix = basePrefix?.trim() ? basePrefix : "";
    const base = routeBase?.trim() ? routeBase.trim().replace(/^\/+/, "") : "docs";
    if (withoutExt === "index") return `${prefix}/${base}${hash}`;
    if (withoutExt.endsWith("/index")) return `${prefix}/${base}/${withoutExt.slice(0, -"/index".length)}${hash}`;
    return `${prefix}/${base}/${withoutExt}${hash}`;
  }

  if (/\.[a-z0-9]+$/i.test(withoutHash)) {
    const rel = withoutHash.replace(/\\/g, "/");
    const baseDir = baseSlug.slice(0, -1).join("/");
    const joined = `${baseDir ? `${baseDir}/` : ""}${rel}`;
    const normalized = joined
      .split("/")
      .reduce<string[]>((acc, seg) => {
        if (seg === "." || seg === "") return acc;
        if (seg === "..") {
          acc.pop();
          return acc;
        }
        acc.push(seg);
        return acc;
      }, [])
      .join("/");
    return `${repoUrl}/blob/main/${normalized}${hash}`;
  }

  return trimmed;
}

export function renderMarkdown(md: string, { baseSlug, basePrefix, routeBase }: RenderOptions): { nodes: ReactNode[]; toc: TocItem[] } {
  const lines = md.replace(/\r\n/g, "\n").split("\n");
  const nodes: ReactNode[] = [];
  const toc: TocItem[] = [];
  const seenIds = new Map<string, number>();

  const nextId = (text: string) => {
    const base = slugify(text) || "section";
    const n = seenIds.get(base) ?? 0;
    seenIds.set(base, n + 1);
    return n === 0 ? base : `${base}-${n + 1}`;
  };

  let i = 0;
  while (i < lines.length) {
    const line = lines[i] ?? "";

    if (/^```/.test(line)) {
      const lang = line.slice(3).trim() || undefined;
      const codeLines: string[] = [];
      i += 1;
      while (i < lines.length && !/^```/.test(lines[i] ?? "")) {
        codeLines.push(lines[i] ?? "");
        i += 1;
      }
      i += 1;
      const code = codeLines.join("\n").trimEnd();
      nodes.push(<CodeBlockShell key={`code-${i}-${code.length}`} code={code} language={lang} />);
      continue;
    }

    const heading = /^(#{1,4})\s+(.+)$/.exec(line);
    if (heading) {
      const level = heading[1].length;
      const text = heading[2].trim();
      const id = nextId(text);
      if (level >= 2) toc.push({ id, text, level });
      const Tag = level === 1 ? "h1" : level === 2 ? "h2" : level === 3 ? "h3" : "h4";
      nodes.push(
        <Tag key={`h-${id}`} id={id}>
          {splitInline(text, baseSlug, basePrefix, routeBase)}
        </Tag>,
      );
      i += 1;
      continue;
    }

    if (/^\s*$/.test(line)) {
      i += 1;
      continue;
    }

    if (/^>\s?/.test(line)) {
      const quoteLines: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i] ?? "")) {
        quoteLines.push((lines[i] ?? "").replace(/^>\s?/, ""));
        i += 1;
      }
      nodes.push(
        <blockquote key={`q-${i}`}>
          {quoteLines.map((l, idx) => (
            <p key={idx}>{splitInline(l, baseSlug, basePrefix, routeBase)}</p>
          ))}
        </blockquote>,
      );
      continue;
    }

    if (/^(\*|-)\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^(\*|-)\s+/.test(lines[i] ?? "")) {
        items.push((lines[i] ?? "").replace(/^(\*|-)\s+/, ""));
        i += 1;
      }
      nodes.push(
        <ul key={`ul-${i}`}>
          {items.map((t, idx) => (
            <li key={idx}>{splitInline(t, baseSlug, basePrefix, routeBase)}</li>
          ))}
        </ul>,
      );
      continue;
    }

    if (/^\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s+/.test(lines[i] ?? "")) {
        items.push((lines[i] ?? "").replace(/^\d+\.\s+/, ""));
        i += 1;
      }
      nodes.push(
        <ol key={`ol-${i}`}>
          {items.map((t, idx) => (
            <li key={idx}>{splitInline(t, baseSlug, basePrefix, routeBase)}</li>
          ))}
        </ol>,
      );
      continue;
    }

    if (/^(-{3,}|\*{3,}|_{3,})$/.test(line.trim())) {
      nodes.push(<hr key={`hr-${i}`} className="border-border" />);
      i += 1;
      continue;
    }

    const isTableRow = (l: string) => /\|/.test(l) && !/^\s*```/.test(l);
    const isTableSep = (l: string) => /^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?\s*$/.test(l);

    if (isTableRow(line) && isTableSep(lines[i + 1] ?? "")) {
      const headerLine = line;
      const sepLine = lines[i + 1] ?? "";
      const rows: string[] = [];
      i += 2;
      while (i < lines.length && lines[i] && isTableRow(lines[i] ?? "")) {
        rows.push(lines[i] ?? "");
        i += 1;
      }

      const splitCells = (l: string) =>
        l
          .trim()
          .replace(/^\|/, "")
          .replace(/\|$/, "")
          .split("|")
          .map((c) => c.trim());

      const headers = splitCells(headerLine);
      const aligns = splitCells(sepLine).map<TextAlign>((c) => {
        const left = c.startsWith(":");
        const right = c.endsWith(":");
        if (left && right) return "center";
        if (right) return "right";
        return "left";
      });
      const bodyRows = rows.map(splitCells);

      nodes.push(
        <div key={`tbl-${i}`} className="overflow-auto rounded-xl border border-border bg-panel/40">
          <table className="min-w-full text-sm border-separate border-spacing-0">
            <thead className="bg-card/40">
              <tr>
                {headers.map((h, idx) => (
                  <th
                    key={idx}
                    className="px-4 py-3 text-left font-semibold text-foreground border-b border-border"
                    style={{ textAlign: aligns[idx] }}
                  >
                    {splitInline(h, baseSlug, basePrefix, routeBase)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {bodyRows.map((r, rIdx) => (
                <tr key={rIdx} className="border-b border-border last:border-b-0">
                  {r.map((c, cIdx) => (
                    <td
                      key={cIdx}
                      className="px-4 py-3 text-muted-foreground border-b border-border last:border-b-0"
                      style={{ textAlign: aligns[cIdx] }}
                    >
                      {splitInline(c, baseSlug, basePrefix, routeBase)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>,
      );
      continue;
    }

    const paraLines: string[] = [];
    while (i < lines.length && lines[i] && !/^\s*$/.test(lines[i] ?? "")) {
      const l = lines[i] ?? "";
      if (/^```/.test(l)) break;
      if (/^(#{1,4})\s+/.test(l)) break;
      if (/^>\s?/.test(l)) break;
      if (/^(\*|-)\s+/.test(l)) break;
      if (/^\d+\.\s+/.test(l)) break;
      paraLines.push(l);
      i += 1;
    }
    const para = paraLines.join(" ").trim();
    nodes.push(<p key={`p-${i}-${para.length}`}>{splitInline(para, baseSlug, basePrefix, routeBase)}</p>);
  }

  return { nodes, toc };
}

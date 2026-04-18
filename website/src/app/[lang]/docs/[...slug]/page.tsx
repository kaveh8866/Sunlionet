import DocPage from "../../../docs/[...slug]/page";
import { getDocsIndex } from "../../../../lib/docs/fs";

export const dynamic = "force-static";

export async function generateStaticParams() {
  const entries = await getDocsIndex();
  const params: Array<{ lang: string; slug: string[] }> = [];

  for (const e of entries) {
    if (e.slug.length === 1 && e.slug[0] === "index") continue;

    if (e.slug[0] === "fa") {
      const stripped = e.slug.slice(1);
      if (stripped.length === 1 && stripped[0] === "index") continue;
      params.push({ lang: "fa", slug: stripped });
      if (stripped.length >= 2 && stripped.at(-1) === "index") {
        params.push({ lang: "fa", slug: stripped.slice(0, -1) });
      }
      continue;
    }

    params.push({ lang: "en", slug: e.slug });
    if (e.slug.length >= 2 && e.slug.at(-1) === "index") {
      params.push({ lang: "en", slug: e.slug.slice(0, -1) });
    }
  }

  return params;
}

export default async function Page({ params }: { params: Promise<{ lang: string; slug: string[] }> }) {
  const resolved = await params;
  const mappedSlug = resolved.lang === "fa" && resolved.slug[0] !== "fa" ? ["fa", ...resolved.slug] : resolved.slug;
  return <DocPage params={Promise.resolve({ slug: mappedSlug })} basePrefix={`/${resolved.lang}`} />;
}

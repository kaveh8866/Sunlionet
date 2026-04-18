import UpdatePage from "../../../updates/[...slug]/page";
import { getContentIndex } from "../../../../lib/content/fs";

export const dynamic = "force-static";

export async function generateStaticParams() {
  const en = await getContentIndex("updates", "en");
  const fa = await getContentIndex("updates", "fa");
  return [
    ...en.map((e) => ({ lang: "en", slug: e.slug })),
    ...fa.map((e) => ({ lang: "fa", slug: e.slug })),
  ];
}

export default async function Page({ params }: { params: Promise<{ lang: string; slug: string[] }> }) {
  const resolved = await params;
  return <UpdatePage params={Promise.resolve({ slug: resolved.slug })} basePrefix={`/${resolved.lang}`} />;
}

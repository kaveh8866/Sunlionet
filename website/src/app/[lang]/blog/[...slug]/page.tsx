import BlogPostPage from "../../../blog/[...slug]/page";
import { getContentIndex } from "../../../../lib/content/fs";

export const dynamic = "force-static";

export async function generateStaticParams() {
  const en = await getContentIndex("blog", "en");
  const fa = await getContentIndex("blog", "fa");
  return [
    ...en.map((e) => ({ lang: "en", slug: e.slug })),
    ...fa.map((e) => ({ lang: "fa", slug: e.slug })),
  ];
}

export default async function Page({ params }: { params: Promise<{ lang: string; slug: string[] }> }) {
  const resolved = await params;
  return <BlogPostPage params={Promise.resolve({ slug: resolved.slug })} basePrefix={`/${resolved.lang}`} />;
}

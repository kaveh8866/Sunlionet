import BlogIndexPage from "../../blog/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <BlogIndexPage params={Promise.resolve(resolved)} />;
}


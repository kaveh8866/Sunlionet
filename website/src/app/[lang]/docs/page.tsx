import DocsIndexPage from "../../docs/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <DocsIndexPage params={Promise.resolve(resolved)} />;
}

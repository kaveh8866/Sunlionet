import DocsAllPage from "../../../docs/all/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <DocsAllPage params={Promise.resolve(resolved)} />;
}

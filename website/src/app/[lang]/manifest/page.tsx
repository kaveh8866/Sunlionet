import ManifestPage from "../../manifest/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <ManifestPage params={Promise.resolve(resolved)} />;
}


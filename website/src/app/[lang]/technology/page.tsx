import TechnologyPage from "../../technology/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <TechnologyPage params={Promise.resolve(resolved)} />;
}


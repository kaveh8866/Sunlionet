import ArchitecturePage from "../../architecture/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <ArchitecturePage params={Promise.resolve(resolved)} />;
}

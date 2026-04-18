import RoadmapPage from "../../roadmap/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <RoadmapPage params={Promise.resolve(resolved)} />;
}

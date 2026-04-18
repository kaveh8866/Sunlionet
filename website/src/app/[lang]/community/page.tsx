import CommunityPage from "../../community/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <CommunityPage params={Promise.resolve(resolved)} />;
}


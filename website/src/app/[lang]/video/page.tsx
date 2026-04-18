import VideoPage from "../../video/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <VideoPage params={Promise.resolve(resolved)} />;
}

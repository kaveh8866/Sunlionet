import DownloadPage from "../../download/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <DownloadPage params={Promise.resolve(resolved)} />;
}

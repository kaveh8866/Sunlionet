import UpdatesIndexPage from "../../updates/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <UpdatesIndexPage params={Promise.resolve(resolved)} />;
}


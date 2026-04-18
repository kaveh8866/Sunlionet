import SupportPage from "../../support/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <SupportPage params={Promise.resolve(resolved)} />;
}

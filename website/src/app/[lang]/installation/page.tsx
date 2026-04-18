import InstallationPage from "../../installation/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <InstallationPage params={Promise.resolve(resolved)} />;
}

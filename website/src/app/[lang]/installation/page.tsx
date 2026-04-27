import InstallationPage from "../../installation/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  return <InstallationPage params={params} />;
}

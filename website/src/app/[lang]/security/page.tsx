import SecurityPage from "../../security/page";

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  return <SecurityPage params={Promise.resolve(resolved)} />;
}


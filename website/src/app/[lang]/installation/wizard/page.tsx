import { redirect } from "next/navigation";

export const dynamic = "force-static";

export default async function InstallationWizardLangRoot({ params }: { params: Promise<{ lang: string }> }) {
  const resolved = await params;
  redirect(`/${resolved.lang}/installation/wizard/welcome`);
}


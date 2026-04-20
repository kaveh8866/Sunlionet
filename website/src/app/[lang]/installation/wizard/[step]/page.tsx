import { InstallWizard } from "../../../../../components/ui/InstallWizard";
import { normalizeWizardStepId } from "../../../../../lib/install-wizard/steps";

export const dynamic = "force-static";

export default async function InstallationWizardLangStepPage({
  params,
}: {
  params: Promise<{ lang: string; step: string }>;
}) {
  const resolved = await params;
  const step = normalizeWizardStepId(resolved.step);
  return <InstallWizard variant="outside" basePath={`/${resolved.lang}/installation/wizard`} step={step} />;
}


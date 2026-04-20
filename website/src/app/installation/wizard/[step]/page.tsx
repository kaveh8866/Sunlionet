import { InstallWizard } from "../../../../components/ui/InstallWizard";
import { normalizeWizardStepId } from "../../../../lib/install-wizard/steps";

export const dynamic = "force-static";

export default async function InstallationWizardStepPage({
  params,
}: {
  params: Promise<{ step: string }>;
}) {
  const resolved = await params;
  const step = normalizeWizardStepId(resolved.step);
  return <InstallWizard variant="outside" basePath="/installation/wizard" step={step} />;
}


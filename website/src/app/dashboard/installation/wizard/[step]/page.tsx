import { InstallWizard } from "../../../../../components/ui/InstallWizard";
import { normalizeWizardStepId } from "../../../../../lib/install-wizard/steps";

export const dynamic = "force-static";

export default async function DashboardInstallationWizardStepPage({
  params,
}: {
  params: Promise<{ step: string }>;
}) {
  const resolved = await params;
  const step = normalizeWizardStepId(resolved.step);
  return <InstallWizard variant="inside" basePath="/dashboard/installation/wizard" step={step} />;
}


import { redirect } from "next/navigation";

export const dynamic = "force-static";

export default function DashboardInstallationWizardRoot() {
  redirect("/dashboard/installation/wizard/welcome");
}


import { redirect } from "next/navigation";

export const dynamic = "force-static";

export default function InstallationWizardRoot() {
  redirect("/installation/wizard/welcome");
}


export type WizardVariant = "outside" | "inside";

export type WizardStepId = "welcome" | "platform" | "download" | "verify" | "configure" | "finish";

export type WizardStep = {
  id: WizardStepId;
  title: string;
  outside: {
    heading: string;
    summary: string;
  };
  inside: {
    heading: string;
    summary: string;
  };
};

export const wizardSteps: WizardStep[] = [
  {
    id: "welcome",
    title: "Welcome",
    outside: {
      heading: "Install SunLionet safely",
      summary: "Follow a verification-first path. Every step is short, touch-friendly, and works offline where possible.",
    },
    inside: {
      heading: "Provision SunLionet Inside",
      summary: "Set up the internal runtime safely: verify artifacts, configure services, and confirm health signals.",
    },
  },
  {
    id: "platform",
    title: "Platform",
    outside: {
      heading: "Choose your platform",
      summary: "Pick Android, Linux, or Windows. The wizard adapts the next steps and keeps your place.",
    },
    inside: {
      heading: "Choose your deployment target",
      summary: "Pick Linux host, Windows host, or a mixed environment. The internal steps focus on service reliability.",
    },
  },
  {
    id: "download",
    title: "Download",
    outside: {
      heading: "Download the right artifact",
      summary: "Download from trusted sources only. Prefer signed releases and keep the checksum alongside the file.",
    },
    inside: {
      heading: "Fetch release artifacts",
      summary: "Download Inside/Outside artifacts plus checksums. Keep a clean directory for repeatable deployment.",
    },
  },
  {
    id: "verify",
    title: "Verify",
    outside: {
      heading: "Verify checksums",
      summary: "Verification comes before execution. If a checksum fails, stop and re-download from a trusted mirror.",
    },
    inside: {
      heading: "Verify before enabling services",
      summary: "Verify checksums and only then install. This reduces supply-chain and mirror tampering risk.",
    },
  },
  {
    id: "configure",
    title: "Configure",
    outside: {
      heading: "Configure and connect",
      summary: "Import a trusted bundle, confirm encryption, and connect. Keep the UI minimal and predictable.",
    },
    inside: {
      heading: "Configure runtime and observability",
      summary: "Enable the runtime API, validate health, and confirm the local dashboard is reachable.",
    },
  },
  {
    id: "finish",
    title: "Finish",
    outside: {
      heading: "You’re ready",
      summary: "Save your verification results and keep your trusted keys and bundles backed up.",
    },
    inside: {
      heading: "Deployment complete",
      summary: "Confirm services survive reboot and validate that runtime events are flowing as expected.",
    },
  },
];

export function normalizeWizardStepId(value: string | undefined): WizardStepId {
  const v = (value ?? "").toLowerCase();
  for (const s of wizardSteps) {
    if (s.id === v) {
      return s.id;
    }
  }
  return "welcome";
}

export function getWizardStepIndex(step: WizardStepId): number {
  const idx = wizardSteps.findIndex((s) => s.id === step);
  return idx >= 0 ? idx : 0;
}


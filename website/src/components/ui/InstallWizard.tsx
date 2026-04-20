"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useMemo, useRef } from "react";
import { AnimatePresence, motion, useReducedMotion, type Transition } from "framer-motion";
import { cx } from "../../lib/cx";
import { getWizardStepIndex, type WizardStepId, type WizardVariant, wizardSteps } from "../../lib/install-wizard/steps";

type InstallWizardProps = {
  variant: WizardVariant;
  basePath: string;
  step: WizardStepId;
};

export function InstallWizard({ variant, basePath, step }: InstallWizardProps) {
  const router = useRouter();
  const pathname = usePathname();
  const reduced = useReducedMotion();
  const headingRef = useRef<HTMLHeadingElement | null>(null);

  const index = useMemo(() => getWizardStepIndex(step), [step]);
  const current = wizardSteps[index] ?? wizardSteps[0];
  const content = variant === "inside" ? current.inside : current.outside;

  const prev = index > 0 ? wizardSteps[index - 1]?.id ?? null : null;
  const next = index < wizardSteps.length - 1 ? wizardSteps[index + 1]?.id ?? null : null;
  const progressPct = wizardSteps.length <= 1 ? 100 : Math.round((index / (wizardSteps.length - 1)) * 100);

  useEffect(() => {
    headingRef.current?.focus();
  }, [pathname]);

  const goTo = (id: WizardStepId) => {
    router.push(`${basePath}/${id}`);
  };

  const shellVariants = reduced
    ? undefined
    : {
        initial: { opacity: 0, y: 10 },
        animate: { opacity: 1, y: 0 },
        exit: { opacity: 0, y: -10 },
      };

  const transition: Transition = reduced ? { duration: 0 } : { type: "spring", stiffness: 420, damping: 34 };

  return (
    <div className="mx-auto w-full max-w-4xl px-4 py-10">
      <div className="grid gap-6">
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div className="min-w-0">
            <div className="inline-flex items-center gap-2 rounded-full border border-border bg-card/60 px-3 py-1 text-xs font-mono text-muted-foreground">
              <span className="text-foreground font-semibold">{variant === "inside" ? "INSIDE" : "OUTSIDE"}</span>
              <span aria-hidden="true">•</span>
              <span>{current.title}</span>
            </div>
            <h1
              ref={headingRef}
              tabIndex={-1}
              className="mt-3 text-3xl md:text-4xl font-extrabold tracking-tight text-foreground outline-none"
            >
              {content.heading}
            </h1>
            <p className="mt-2 text-muted-foreground leading-relaxed max-w-2xl">{content.summary}</p>
          </div>

          <div className="flex gap-2 items-center">
            <Link
              href={variant === "inside" ? "/dashboard" : "/installation"}
              prefetch={false}
              className="wizard-tap inline-flex items-center justify-center rounded-md border border-border bg-card/60 px-4 py-2 text-sm font-semibold text-foreground hover:opacity-90 transition-opacity"
              style={{ touchAction: "manipulation" }}
            >
              Exit
            </Link>
          </div>
        </div>

        <div className="rounded-xl border border-border bg-card/60 shadow-[0_0_0_1px_var(--border)] overflow-hidden">
          <div className="p-4 md:p-5 border-b border-border">
            <div className="flex items-center justify-between gap-3">
              <div className="text-xs text-muted-foreground font-mono">
                Step {index + 1} / {wizardSteps.length}
              </div>
              <div className="text-xs text-muted-foreground font-mono">{progressPct}%</div>
            </div>
            <div className="mt-3 h-2 rounded-full bg-panel/70 border border-border overflow-hidden">
              <motion.div
                className="h-full bg-primary"
                initial={false}
                animate={{ width: `${progressPct}%` }}
                transition={transition}
              />
            </div>
            <WizardStepTabs variant={variant} basePath={basePath} active={step} onSelect={goTo} />
          </div>

          <div className="p-4 md:p-6">
            <AnimatePresence mode="wait" initial={false}>
              <motion.section
                key={`${variant}:${step}`}
                layout
                initial={shellVariants?.initial}
                animate={shellVariants?.animate}
                exit={shellVariants?.exit}
                transition={transition}
              >
                <WizardBody variant={variant} step={step} />
              </motion.section>
            </AnimatePresence>
          </div>

          <div className="wizard-safe-area border-t border-border bg-panel/50">
            <div className="p-4 md:p-5 flex items-center justify-between gap-3">
              <button
                type="button"
                className={cx(
                  "wizard-tap inline-flex items-center justify-center rounded-md border border-border px-4 py-3 text-sm font-semibold transition-opacity",
                  prev ? "bg-card/60 text-foreground hover:opacity-90" : "bg-card/30 text-muted-foreground opacity-70 cursor-not-allowed",
                )}
                style={{ touchAction: "manipulation" }}
                onClick={() => (prev ? goTo(prev) : null)}
                disabled={!prev}
              >
                Back
              </button>

              <div className="flex-1" />

              <button
                type="button"
                className={cx(
                  "wizard-tap inline-flex items-center justify-center rounded-md px-5 py-3 text-sm font-semibold transition-opacity shadow-[0_0_0_1px_var(--border)]",
                  next ? "bg-primary text-primary-foreground hover:opacity-90" : "bg-success text-success-foreground hover:opacity-90",
                )}
                style={{ touchAction: "manipulation" }}
                onClick={() => (next ? goTo(next) : router.push(variant === "inside" ? "/dashboard" : "/download"))}
              >
                {next ? "Next" : "Done"}
              </button>
            </div>
          </div>
        </div>

        <div className="text-xs text-muted-foreground">
          Tip: If you prefer fewer animations, enable your system’s reduced-motion setting and the wizard will simplify transitions.
        </div>
      </div>
    </div>
  );
}

function WizardStepTabs({
  variant,
  basePath,
  active,
  onSelect,
}: {
  variant: WizardVariant;
  basePath: string;
  active: WizardStepId;
  onSelect: (id: WizardStepId) => void;
}) {
  const reduced = useReducedMotion();
  const pillTransition: Transition = reduced ? { duration: 0 } : { type: "spring", stiffness: 520, damping: 40 };

  return (
    <div className="mt-4 overflow-x-auto">
      <div className="min-w-max flex items-center gap-2">
        {wizardSteps.map((s) => {
          const isActive = s.id === active;
          return (
            <button
              key={s.id}
              type="button"
              onClick={() => onSelect(s.id)}
              className={cx(
                "wizard-tap relative inline-flex items-center gap-2 rounded-full border border-border px-3 py-2 text-xs font-semibold transition-colors",
                isActive ? "text-foreground bg-card" : "text-muted-foreground bg-card/40 hover:bg-card/60",
              )}
              style={{ touchAction: "manipulation" }}
              aria-current={isActive ? "step" : undefined}
            >
              <span className="font-mono opacity-80">{String(wizardSteps.findIndex((x) => x.id === s.id) + 1)}</span>
              <span>{s.title}</span>
              {isActive ? (
                <motion.span
                  layoutId={`wizard-pill:${variant}:${basePath}`}
                  className="absolute inset-0 rounded-full ring-2 ring-ring"
                  transition={pillTransition}
                />
              ) : null}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function WizardBody({ variant, step }: { variant: WizardVariant; step: WizardStepId }) {
  const accent = variant === "inside" ? "bg-accent text-accent-foreground" : "bg-primary text-primary-foreground";

  if (step === "welcome") {
    return (
      <div className="grid gap-4">
        <div className="rounded-xl border border-border bg-card p-5">
          <div className={cx("inline-flex rounded-md px-3 py-1 text-xs font-semibold", accent)}>
            Verification-first
          </div>
          <div className="mt-3 text-sm text-muted-foreground leading-relaxed">
            You will download artifacts, verify checksums, then configure your connection. Each page is designed for mobile taps and
            quick review on desktop.
          </div>
        </div>

        <div className="grid md:grid-cols-3 gap-3">
          {[
            { t: "Fast", d: "Short steps with clear actions." },
            { t: "Safe", d: "Verification before execution." },
            { t: "Consistent", d: "Same motion + branding across pages." },
          ].map((x) => (
            <div key={x.t} className="rounded-xl border border-border bg-card p-4">
              <div className="text-foreground font-bold">{x.t}</div>
              <div className="mt-1 text-sm text-muted-foreground leading-relaxed">{x.d}</div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (step === "platform") {
    return (
      <div className="grid gap-4">
        <div className="text-sm text-muted-foreground leading-relaxed">
          Pick the path that matches your device. You can switch later without losing progress.
        </div>
        <div className="grid sm:grid-cols-3 gap-3">
          {[
            { t: "Android", d: "Install the APK and import a trusted bundle." },
            { t: "Linux", d: "Install systemd services for Inside/Outside." },
            { t: "Windows", d: "Use a verified build path (native or WSL) and keep checksums." },
          ].map((x) => (
            <div key={x.t} className="rounded-xl border border-border bg-card p-4">
              <div className="text-foreground font-bold">{x.t}</div>
              <div className="mt-1 text-sm text-muted-foreground leading-relaxed">{x.d}</div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (step === "download") {
    return (
      <div className="grid gap-4">
        <div className="rounded-xl border border-border bg-card p-5">
          <div className="text-foreground font-bold">Download artifacts + checksums</div>
          <div className="mt-2 text-sm text-muted-foreground leading-relaxed">
            Download the artifact and its <span className="font-mono">.sha256</span> file together. Keep them in the same folder.
          </div>
        </div>
        <div className="grid md:grid-cols-2 gap-3">
          <div className="rounded-xl border border-border bg-card p-4">
            <div className="text-xs text-muted-foreground uppercase tracking-wider">Outside</div>
            <div className="mt-1 text-sm text-muted-foreground leading-relaxed">
              Public-facing distribution + trust boundaries (bundles, verification, seed flow).
            </div>
          </div>
          <div className="rounded-xl border border-border bg-card p-4">
            <div className="text-xs text-muted-foreground uppercase tracking-wider">Inside</div>
            <div className="mt-1 text-sm text-muted-foreground leading-relaxed">
              Internal runtime engine and local observability endpoints.
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (step === "verify") {
    return (
      <div className="grid gap-4">
        <div className="rounded-xl border border-border bg-card p-5">
          <div className="text-foreground font-bold">Verify checksums</div>
          <div className="mt-2 text-sm text-muted-foreground leading-relaxed">
            If verification fails, do not run anything. Re-download from a trusted source.
          </div>
        </div>
        <div className="grid md:grid-cols-2 gap-3">
          <div className="rounded-xl border border-border bg-card p-4">
            <div className="text-foreground font-semibold">Linux</div>
            <div className="mt-2 text-sm text-muted-foreground font-mono">sha256sum -c yourfile.sha256</div>
          </div>
          <div className="rounded-xl border border-border bg-card p-4">
            <div className="text-foreground font-semibold">Windows</div>
            <div className="mt-2 text-sm text-muted-foreground font-mono">CertUtil -hashfile yourfile SHA256</div>
          </div>
        </div>
      </div>
    );
  }

  if (step === "configure") {
    return (
      <div className="grid gap-4">
        <div className="rounded-xl border border-border bg-card p-5">
          <div className="text-foreground font-bold">{variant === "inside" ? "Enable runtime + health" : "Import + connect"}</div>
          <div className="mt-2 text-sm text-muted-foreground leading-relaxed">
            {variant === "inside"
              ? "Enable the runtime API, confirm services are healthy, and validate event flow on the dashboard."
              : "Import a trusted bundle, confirm encryption is active, and connect. Keep keys and bundles backed up."}
          </div>
        </div>
        <div className="rounded-xl border border-border bg-card p-4">
          <div className="text-xs text-muted-foreground uppercase tracking-wider">Check</div>
          <div className="mt-1 text-sm text-muted-foreground leading-relaxed">
            {variant === "inside"
              ? "Dashboard → Runtime shows CONNECTED and a recent event timestamp."
              : "UI indicates secure mode and messages can be exchanged with a trusted contact."}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="grid gap-4">
      <div className="rounded-xl border border-border bg-card p-5">
        <div className="text-foreground font-bold">Next steps</div>
        <div className="mt-2 text-sm text-muted-foreground leading-relaxed">
          {variant === "inside"
            ? "Confirm services survive reboot and keep verification logs. Review the QA checklist for cross-device testing."
            : "Keep verification results. Review the QA checklist and share only with trusted contacts."}
        </div>
      </div>
      <div className="grid md:grid-cols-2 gap-3">
        <Link
          href={variant === "inside" ? "/dashboard/runtime" : "/download"}
          prefetch={false}
          className="wizard-tap rounded-xl border border-border bg-card/60 p-4 hover:opacity-90 transition-opacity shadow-[0_0_0_1px_var(--border)]"
          style={{ touchAction: "manipulation" }}
        >
          <div className="text-foreground font-bold">{variant === "inside" ? "Open Runtime" : "Open Download"}</div>
          <div className="mt-1 text-sm text-muted-foreground leading-relaxed">
            {variant === "inside" ? "Validate runtime state and events." : "Choose your artifact and verify."}
          </div>
        </Link>
        <Link
          href="/docs/web/playwright-testing"
          prefetch={false}
          className="wizard-tap rounded-xl border border-border bg-card/60 p-4 hover:opacity-90 transition-opacity shadow-[0_0_0_1px_var(--border)]"
          style={{ touchAction: "manipulation" }}
        >
          <div className="text-foreground font-bold">Testing</div>
          <div className="mt-1 text-sm text-muted-foreground leading-relaxed">Run automated checks and cross-device validation.</div>
        </Link>
      </div>
    </div>
  );
}

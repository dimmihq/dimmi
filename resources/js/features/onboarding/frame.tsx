/*
 * The frame of the setup steps: the product's mark and who is signed in
 * along the top, a thin bar that fills as the steps go by, and one
 * centred column below, so each step reads as a single question.
 */
import { RiArrowLeftLine, RiArrowRightLine, RiLogoutBoxRLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { ThemeToggle } from "@/components/application/theme/theme-toggle";
import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { ProgressBar } from "@/components/base/progress/progress";
import { DimmiLogo } from "@/features/brand/logo";
import { meQuery, signOut } from "@/lib/api/auth";

import { steps } from "./steps";

export function OnboardingFrame({ email, done, children }: { email: string; done: number; children: ReactNode }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const leave = useMutation({
    mutationFn: signOut,
    onSuccess: () => {
      queryClient.setQueryData(meQuery.queryKey, null);
      router.history.push("/login");
    },
  });

  return (
    <div className="flex min-h-screen flex-col bg-background-full">
      <header className="flex items-center justify-between gap-4 px-4 py-3 sm:px-6">
        <DimmiLogo className="h-5 text-text-primary" />
        <div className="flex items-center gap-3">
          <span className="hidden text-body-2-regular text-text-tertiary sm:inline">
            Signed in as <bdi dir="ltr">{email}</bdi>
          </span>
          <ThemeToggle appearance="segmented" size="small" />
          <Button variant="secondary" size="small" leadingIcon={RiLogoutBoxRLine} onClick={() => leave.mutate()}>
            Sign out
          </Button>
        </div>
      </header>
      <ProgressBar aria-label="Setup progress" value={done} maxValue={steps.length} size="sm" />
      <main className="flex flex-1 justify-center px-4 pt-10 pb-16 sm:pt-16">
        <div className="flex w-full max-w-xl flex-col">{children}</div>
      </main>
    </div>
  );
}

/*
 * StepHeader names the step: where it sits among the steps, a title and
 * what it is for.
 */
export function StepHeader({ label, title, description }: { label: string; title: string; description: ReactNode }) {
  return (
    <div className="flex flex-col">
      <div className="flex items-center gap-3">
        <span className="font-mono text-caption-1-regular text-text-tertiary">{label}</span>
        <span aria-hidden className="h-px flex-1 bg-background-tertiary-default" />
      </div>
      <h1 className="mt-5 text-title-1-medium text-text-primary">{title}</h1>
      <p className="mt-2 text-body-regular text-text-secondary">{description}</p>
    </div>
  );
}

/*
 * StepActions are a step's ways on: the main action, a quiet way to skip
 * when the step is optional, and a way back to the step before.
 */
export function StepActions({
  primary,
  primaryLabel = "Save and continue",
  isPending = false,
  isDisabled = false,
  skipLabel,
  onSkip,
  onBack,
}: {
  primary?: () => void;
  primaryLabel?: string;
  isPending?: boolean;
  isDisabled?: boolean;
  skipLabel?: string;
  onSkip?: () => void;
  onBack?: () => void;
}) {
  const buttonLabel = isPending ? "Saving…" : primaryLabel;

  return (
    <div className="mt-8 flex flex-col items-start gap-6">
      <div className="flex flex-wrap items-center gap-4">
        {primary ? (
          <Button
            size="small"
            type="submit"
            trailingIcon={RiArrowRightLine}
            disabled={isPending || isDisabled}
            onClick={(event) => {
              if (event.currentTarget.form) {
                return;
              }
              primary();
            }}
          >
            {buttonLabel}
          </Button>
        ) : null}
        {onSkip && skipLabel ? (
          <LinkButton variant="secondary" size="small" onClick={onSkip}>
            {skipLabel}
          </LinkButton>
        ) : null}
      </div>
      {onBack ? (
        <LinkButton variant="secondary" size="small" leadingIcon={RiArrowLeftLine} onClick={onBack}>
          Back
        </LinkButton>
      ) : null}
    </div>
  );
}

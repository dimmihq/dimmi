/*
 * The page in front of a private deployment, such as staging: one shared
 * password before anything else loads. It posts a plain form to the gate,
 * which checks the password, sets its cookie and sends the person on, so
 * it calls no API the gate would refuse. It sits in the sign-in pages'
 * frame and card so a private deployment looks like the product.
 */
import { RiLockLine } from "@remixicon/react";
import { ThemeToggle } from "@/components/application/theme/theme-toggle";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AuthCard, AuthFooter, AuthHeader } from "@/features/auth/auth-frame";

import { GATE_PATH } from "./gate-path";

export interface GatePageProps {
  next: string;
  failed: boolean;
}

export function GatePage({ next, failed }: GatePageProps) {
  return (
    <div className="flex min-h-screen flex-col bg-background-full">
      <header className="flex items-center justify-end p-4 sm:p-6">
        <ThemeToggle appearance="segmented" size="small" />
      </header>
      <main className="flex flex-1 items-start justify-center px-4 pt-8 pb-16 sm:items-center sm:pt-0">
        <AuthCard>
          <AuthHeader
            title="This site is private"
            description="Enter the shared password to continue. You stay signed in on this browser for 30 days."
          />
          <form method="post" action={GATE_PATH} className="mt-6 flex flex-col gap-4">
            <input type="hidden" name="next" value={next} />
            <Input
              name="password"
              type="password"
              label="Password"
              autoComplete="current-password"
              leadingIcon={RiLockLine}
              autoFocus
              isRequired
              validationBehavior="aria"
              isInvalid={failed}
              errorMessage={failed ? "That password is not right." : undefined}
            />
            <Button type="submit" className="w-full">
              Continue
            </Button>
          </form>
          <AuthFooter>Ask the person who shared this link for the password.</AuthFooter>
        </AuthCard>
      </main>
    </div>
  );
}
